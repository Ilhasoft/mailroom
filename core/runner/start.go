package runner

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/mailroom/core/goflow"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/runner/clocks"
	"github.com/nyaruka/mailroom/runtime"
)

const (
	commitTimeout = time.Minute
)

// TriggerBuilder defines the interface for building a trigger for the passed in contact
type TriggerBuilder func() flows.Trigger

// StartWithLock starts the given contacts in flow sessions after obtaining locks for them.
func StartWithLock(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, contactIDs []models.ContactID, triggerBuilder TriggerBuilder, interrupt bool, startID models.StartID) ([]*Scene, error) {
	if len(contactIDs) == 0 {
		return nil, nil
	}

	// we now need to grab locks for our contacts so that they are never in two starts or handles at the
	// same time we try to grab locks for up to five minutes, but do it in batches where we wait for one
	// second per contact to prevent deadlocks
	scenes := make([]*Scene, 0, len(contactIDs))
	remaining := contactIDs
	start := time.Now()

	for len(remaining) > 0 && time.Since(start) < time.Minute*5 {
		if ctx.Err() != nil {
			return scenes, ctx.Err()
		}

		ss, skipped, err := tryToStartWithLock(ctx, rt, oa, remaining, triggerBuilder, interrupt, startID)
		if err != nil {
			return nil, err
		}

		scenes = append(scenes, ss...)
		remaining = skipped // skipped are now our remaining
	}

	if len(remaining) > 0 {
		slog.Warn("failed to acquire locks for contacts", "contacts", remaining)
	}

	return scenes, nil
}

// tries to start the given contacts, returning sessions for those we could, and the ids that were skipped because we
// couldn't get their locks
func tryToStartWithLock(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, ids []models.ContactID, triggerBuilder TriggerBuilder, interrupt bool, startID models.StartID) ([]*Scene, []models.ContactID, error) {
	// try to get locks for these contacts, waiting for up to a second for each contact
	locks, skipped, err := clocks.TryToLock(ctx, rt, oa, ids, time.Second)
	if err != nil {
		return nil, nil, err
	}
	locked := slices.Collect(maps.Keys(locks))

	// whatever happens, we need to unlock the contacts
	defer clocks.Unlock(ctx, rt, oa, locks)

	// load our locked contacts
	mcs, err := models.LoadContacts(ctx, rt.ReadonlyDB, oa, locked)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading contacts to start: %w", err)
	}

	// create scenes and triggers
	scenes := make([]*Scene, 0, len(mcs))
	triggers := make([]flows.Trigger, 0, len(locked))
	for _, mc := range mcs {
		c, err := mc.EngineContact(oa)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating flow contact: %w", err)
		}

		scene := NewScene(mc, c, models.NilUserID)
		scene.StartID = startID
		scene.Interrupt = interrupt

		scenes = append(scenes, scene)
		triggers = append(triggers, triggerBuilder())
	}

	err = StartSessions(ctx, rt, oa, scenes, triggers)
	if err != nil {
		return nil, nil, fmt.Errorf("error starting flow for contacts: %w", err)
	}

	return scenes, skipped, nil
}

// StartSessions starts the given contacts in flow sessions. It's assumed that the contacts are already locked.
func StartSessions(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, scenes []*Scene, triggers []flows.Trigger) error {
	if len(scenes) == 0 {
		return nil
	}

	start := time.Now()
	sa := oa.SessionAssets()

	// for sanity, check that contacts have been locked
	lockCheck, _ := clocks.IsLocked(ctx, rt, oa, scenes[0].DBContact.ID())
	if !lockCheck {
		slog.Error("starting session for contact that isn't locked", "contact", scenes[0].DBContact.ID())
	}

	// for each trigger start the flow
	sessions := make([]flows.Session, len(triggers))
	sprints := make([]flows.Sprint, len(triggers))

	for i, scene := range scenes {
		trigger := triggers[i]

		session, sprint, err := goflow.Engine(rt).NewSession(ctx, sa, oa.Env(), scene.Contact, trigger, scene.Call)
		if err != nil {
			return fmt.Errorf("error starting contact %s in flow %s: %w", scene.ContactUUID(), trigger.Flow().UUID, err)
		}

		sessions[i] = session
		sprints[i] = sprint
		scene.AddSprint(session, sprint, false)

		if err := scene.ProcessEvents(ctx, rt, oa); err != nil {
			return fmt.Errorf("error processing events for session %s: %w", session.UUID(), err)
		}
	}

	// we write our sessions and all their objects in a single transaction
	txCTX, cancel := context.WithTimeout(ctx, commitTimeout*time.Duration(len(sessions)))
	defer cancel()

	tx, err := rt.DB.BeginTxx(txCTX, nil)
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}

	mcs := make([]*models.Contact, len(scenes))
	callIDs := make([]models.CallID, len(triggers))
	startIDs := make([]models.StartID, len(triggers))
	for i, s := range scenes {
		mcs[i] = s.DBContact
		startIDs[i] = s.StartID
		if s.DBCall != nil {
			callIDs[i] = s.DBCall.ID()
		}
	}

	// interrupt contacts if desired
	interruptIDs := make([]models.ContactID, 0, len(scenes))
	for _, s := range scenes {
		if s.Interrupt {
			interruptIDs = append(interruptIDs, s.DBContact.ID())
		}
	}
	if len(interruptIDs) > 0 {
		if err := models.InterruptSessionsForContactsTx(txCTX, tx, interruptIDs); err != nil {
			tx.Rollback()
			return fmt.Errorf("error interrupting contacts: %w", err)
		}
	}

	// write our session to the db
	_, err = models.InsertSessions(txCTX, rt, tx, oa, sessions, sprints, mcs, callIDs, startIDs)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error interrupting contacts: %w", err)
	}

	// gather all our pre commit events, group them by hook
	if err := ExecutePreCommitHooks(ctx, rt, tx, oa, scenes); err != nil {
		tx.Rollback()
		return fmt.Errorf("error applying session pre commit hooks: %w", err)
	}

	if err := tx.Commit(); err != nil {
		// retry committing our scenes one at a time
		slog.Debug("failed committing bulk transaction, retrying one at a time", "error", err)

		tx.Rollback()

		// we failed writing our sessions in one go, try one at a time
		for i, scene := range scenes {
			mc, session, sprint, callID, startID := mcs[i], sessions[i], sprints[i], callIDs[i], startIDs[i]

			txCTX, cancel := context.WithTimeout(ctx, commitTimeout)
			defer cancel()

			tx, err := rt.DB.BeginTxx(txCTX, nil)
			if err != nil {
				return fmt.Errorf("error starting transaction for retry: %w", err)
			}

			// interrupt this contact if appropriate
			if scene.Interrupt {
				err = models.InterruptSessionsForContactsTx(txCTX, tx, []models.ContactID{mc.ID()})
				if err != nil {
					tx.Rollback()
					slog.Error("error interrupting contact", "error", err, "contact", session.Contact().UUID())
					continue
				}
			}

			_, err = models.InsertSessions(txCTX, rt, tx, oa, []flows.Session{session}, []flows.Sprint{sprint}, []*models.Contact{mc}, []models.CallID{callID}, []models.StartID{startID})
			if err != nil {
				tx.Rollback()
				slog.Error("error writing session to db", "error", err, "contact", session.Contact().UUID())
				continue
			}

			// gather all our pre commit events, group them by hook
			if err := ExecutePreCommitHooks(ctx, rt, tx, oa, []*Scene{scene}); err != nil {
				return fmt.Errorf("error applying session pre commit hooks: %w", err)
			}

			if err := tx.Commit(); err != nil {
				tx.Rollback()
				slog.Error("error committing session to db", "error", err, "contact", session.Contact().UUID())
				continue
			}
		}
	} else {
		slog.Debug("sessions committed", "count", len(sessions))
	}

	if err := ExecutePostCommitHooks(ctx, rt, oa, scenes); err != nil {
		return fmt.Errorf("error processing post commit hooks: %w", err)
	}

	slog.Debug("started sessions", "count", len(sessions), "elapsed", time.Since(start))

	return nil
}
