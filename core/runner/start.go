package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/nyaruka/goflow/excellent/types"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/triggers"
	"github.com/nyaruka/mailroom/core/goflow"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
)

const (
	commitTimeout     = time.Minute
	postCommitTimeout = time.Minute
)

var startTypeToOrigin = map[models.StartType]string{
	models.StartTypeManual:    "ui",
	models.StartTypeAPI:       "api",
	models.StartTypeAPIZapier: "zapier",
}

// TriggerBuilder defines the interface for building a trigger for the passed in contact
type TriggerBuilder func(contact *flows.Contact) flows.Trigger

// StartOptions define the various parameters that can be used when starting a flow
type StartOptions struct {
	// Interrupt should be true if we want to interrupt the flows runs for any contact started in this flow
	Interrupt bool

	// CommitHook is the hook that will be called in the transaction where each session is written
	CommitHook models.SessionCommitHook

	// TriggerBuilder is the builder that will be used to build a trigger for each contact started in the flow
	TriggerBuilder TriggerBuilder
}

// StartFlowBatch starts the flow for the passed in org, contacts and flow
func StartFlowBatch(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, start *models.FlowStart, batch *models.FlowStartBatch) ([]*models.Session, error) {
	// try to load our flow
	flow, err := oa.FlowByID(start.FlowID)
	if err == models.ErrNotFound {
		slog.Info("skipping flow start, flow no longer active or archived", "flow_id", start.FlowID)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error loading flow: %d: %w", start.FlowID, err)
	}

	// get the user that created this flow start if there was one
	var flowUser *flows.User
	if start.CreatedByID != models.NilUserID {
		user := oa.UserByID(start.CreatedByID)
		if user != nil {
			flowUser = oa.SessionAssets().Users().Get(user.Email())
		}
	}

	var params *types.XObject
	if !start.Params.IsNull() {
		params, err = types.ReadXObject(start.Params)
		if err != nil {
			return nil, fmt.Errorf("unable to read JSON from flow start params: %w", err)
		}
	}

	var history *flows.SessionHistory
	if !start.SessionHistory.IsNull() {
		history, err = models.ReadSessionHistory(start.SessionHistory)
		if err != nil {
			return nil, fmt.Errorf("unable to read JSON from flow start history: %w", err)
		}
	}

	// whether engine allows some functions is based on whether there is more than one contact being started
	batchStart := batch.TotalContacts > 1

	// this will build our trigger for each contact started
	triggerBuilder := func(contact *flows.Contact) flows.Trigger {
		if !start.ParentSummary.IsNull() {
			tb := triggers.NewBuilder(oa.Env(), flow.Reference(), contact).FlowAction(history, json.RawMessage(start.ParentSummary))
			if batchStart {
				tb = tb.AsBatch()
			}
			return tb.Build()
		}

		tb := triggers.NewBuilder(oa.Env(), flow.Reference(), contact).Manual()
		if !start.Params.IsNull() {
			tb = tb.WithParams(params)
		}
		if batchStart {
			tb = tb.AsBatch()
		}
		return tb.WithUser(flowUser).WithOrigin(startTypeToOrigin[start.StartType]).Build()
	}

	options := &StartOptions{
		Interrupt:      flow.FlowType().Interrupts(),
		TriggerBuilder: triggerBuilder,
	}

	sessions, err := StartFlow(ctx, rt, oa, flow, batch.ContactIDs, options, batch.StartID)
	if err != nil {
		return nil, fmt.Errorf("error starting flow batch: %w", err)
	}

	return sessions, nil
}

// StartFlow runs the passed in flow for the passed in contacts
func StartFlow(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, flow *models.Flow, contactIDs []models.ContactID, options *StartOptions, startID models.StartID) ([]*models.Session, error) {
	if len(contactIDs) == 0 {
		return nil, nil
	}

	// we now need to grab locks for our contacts so that they are never in two starts or handles at the
	// same time we try to grab locks for up to five minutes, but do it in batches where we wait for one
	// second per contact to prevent deadlocks
	sessions := make([]*models.Session, 0, len(contactIDs))
	remaining := contactIDs
	start := time.Now()

	for len(remaining) > 0 && time.Since(start) < time.Minute*5 {
		if ctx.Err() != nil {
			return sessions, ctx.Err()
		}

		ss, skipped, err := tryToStartWithLock(ctx, rt, oa, flow, remaining, options, startID)
		if err != nil {
			return nil, err
		}

		sessions = append(sessions, ss...)
		remaining = skipped // skipped are now our remaining
	}

	if len(remaining) > 0 {
		slog.Warn("failed to acquire locks for contacts", "contacts", remaining)
	}

	return sessions, nil
}

// tries to start the given contacts, returning sessions for those we could, and the ids that were skipped because we
// couldn't get their locks
func tryToStartWithLock(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, flow *models.Flow, ids []models.ContactID, options *StartOptions, startID models.StartID) ([]*models.Session, []models.ContactID, error) {
	// try to get locks for these contacts, waiting for up to a second for each contact
	locks, skipped, err := models.LockContacts(ctx, rt, oa.OrgID(), ids, time.Second)
	if err != nil {
		return nil, nil, err
	}
	locked := slices.Collect(maps.Keys(locks))

	// whatever happens, we need to unlock the contacts
	defer models.UnlockContacts(rt, oa.OrgID(), locks)

	// load our locked contacts
	contacts, err := models.LoadContacts(ctx, rt.ReadonlyDB, oa, locked)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading contacts to start: %w", err)
	}

	// build our triggers
	triggers := make([]flows.Trigger, 0, len(locked))
	for _, c := range contacts {
		contact, err := c.FlowContact(oa)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating flow contact: %w", err)
		}
		trigger := options.TriggerBuilder(contact)
		triggers = append(triggers, trigger)
	}

	ss, err := StartFlowForContacts(ctx, rt, oa, flow, contacts, triggers, options.CommitHook, options.Interrupt, startID)
	if err != nil {
		return nil, nil, fmt.Errorf("error starting flow for contacts: %w", err)
	}

	return ss, skipped, nil
}

// StartFlowForContacts runs the passed in flow for the passed in contact
func StartFlowForContacts(
	ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets,
	flow *models.Flow, contacts []*models.Contact, triggers []flows.Trigger, hook models.SessionCommitHook, interrupt bool, startID models.StartID) ([]*models.Session, error) {
	sa := oa.SessionAssets()

	// no triggers? nothing to do
	if len(triggers) == 0 {
		return nil, nil
	}

	start := time.Now()
	log := slog.With(slog.Group("flow", "uuid", flow.UUID, "name", flow.Name))

	// for each trigger start the flow
	sessions := make([]flows.Session, 0, len(triggers))
	sprints := make([]flows.Sprint, 0, len(triggers))

	for _, trigger := range triggers {
		log := log.With("contact", trigger.Contact().UUID())

		session, sprint, err := goflow.Engine(rt).NewSession(sa, trigger)
		if err != nil {
			log.Error("error starting flow", "error", err)
			continue
		}
		log.Debug("new flow session")

		sessions = append(sessions, session)
		sprints = append(sprints, sprint)
	}

	if len(sessions) == 0 {
		return nil, nil
	}

	// we write our sessions and all their objects in a single transaction
	txCTX, cancel := context.WithTimeout(ctx, commitTimeout*time.Duration(len(sessions)))
	defer cancel()

	tx, err := rt.DB.BeginTxx(txCTX, nil)
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}

	// build our list of contact ids
	contactIDs := make([]models.ContactID, len(triggers))
	for i := range triggers {
		contactIDs[i] = models.ContactID(triggers[i].Contact().ID())
	}

	// interrupt all our contacts if desired
	if interrupt {
		if err := models.InterruptSessionsForContactsTx(txCTX, tx, contactIDs); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("error interrupting contacts: %w", err)
		}
	}

	// write our session to the db
	dbSessions, err := models.InsertSessions(txCTX, rt, tx, oa, sessions, sprints, contacts, hook, startID)
	if err == nil {
		// commit it at once
		commitStart := time.Now()
		err = tx.Commit()

		if err == nil {
			slog.Debug("sessions committed", "count", len(sessions), "elapsed", time.Since(commitStart))
		}
	}

	// retry committing our sessions one at a time
	if err != nil {
		slog.Debug("failed committing bulk transaction, retrying one at a time", "error", err)

		tx.Rollback()

		// we failed writing our sessions in one go, try one at a time
		for i := range sessions {
			session := sessions[i]
			sprint := sprints[i]
			contact := contacts[i]

			txCTX, cancel := context.WithTimeout(ctx, commitTimeout)
			defer cancel()

			tx, err := rt.DB.BeginTxx(txCTX, nil)
			if err != nil {
				return nil, fmt.Errorf("error starting transaction for retry: %w", err)
			}

			// interrupt this contact if appropriate
			if interrupt {
				err = models.InterruptSessionsForContactsTx(txCTX, tx, []models.ContactID{models.ContactID(session.Contact().ID())})
				if err != nil {
					tx.Rollback()
					log.Error("error interrupting contact", "error", err, "contact", session.Contact().UUID())
					continue
				}
			}

			dbSession, err := models.InsertSessions(txCTX, rt, tx, oa, []flows.Session{session}, []flows.Sprint{sprint}, []*models.Contact{contact}, hook, startID)
			if err != nil {
				tx.Rollback()
				log.Error("error writing session to db", "error", err, "contact", session.Contact().UUID())
				continue
			}

			err = tx.Commit()
			if err != nil {
				tx.Rollback()
				log.Error("error committing session to db", "error", err, "contact", session.Contact().UUID())
				continue
			}

			dbSessions = append(dbSessions, dbSession[0])
		}
	}

	// now take care of any post-commit hooks
	txCTX, cancel = context.WithTimeout(ctx, postCommitTimeout*time.Duration(len(sessions)))
	defer cancel()

	tx, err = rt.DB.BeginTxx(txCTX, nil)
	if err != nil {
		return nil, fmt.Errorf("error starting transaction for post commit hooks: %w", err)
	}

	scenes := make([]*models.Scene, 0, len(triggers))
	for _, s := range dbSessions {
		scenes = append(scenes, s.Scene())
	}

	err = models.ApplyEventPostCommitHooks(txCTX, rt, tx, oa, scenes)
	if err == nil {
		err = tx.Commit()
	}

	if err != nil {
		tx.Rollback()

		// we failed with our post commit hooks, try one at a time, logging those errors
		for _, session := range dbSessions {
			log = log.With("contact_uuid", session.ContactUUID())

			txCTX, cancel = context.WithTimeout(ctx, postCommitTimeout)
			defer cancel()

			tx, err := rt.DB.BeginTxx(txCTX, nil)
			if err != nil {
				tx.Rollback()
				log.Error("error starting transaction to retry post commits", "error", err)
				continue
			}

			err = models.ApplyEventPostCommitHooks(ctx, rt, tx, oa, []*models.Scene{session.Scene()})
			if err != nil {
				tx.Rollback()
				log.Error("error applying post commit hook", "error", err)
				continue
			}

			err = tx.Commit()

			if err != nil {
				tx.Rollback()
				log.Error("error comitting post commit hook", "error", err)
				continue
			}
		}
	}

	log.Debug("started sessions", "count", len(dbSessions), "elapsed", time.Since(start))

	return dbSessions, nil
}
