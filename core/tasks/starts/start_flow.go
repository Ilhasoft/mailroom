package starts

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nyaruka/gocommon/i18n"
	"github.com/nyaruka/goflow/contactql"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/search"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/core/tasks/ivr"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/mailroom/utils/queues"
)

const (
	TypeStartFlow = "start_flow"

	startBatchSize = 100
)

func init() {
	tasks.RegisterType(TypeStartFlow, func() tasks.Task { return &StartFlowTask{} })
}

// StartFlowBatchTask is the start flow batch task
type StartFlowTask struct {
	*models.FlowStart
}

func (t *StartFlowTask) Type() string {
	return TypeStartFlow
}

// Timeout is the maximum amount of time the task can run for
func (t *StartFlowTask) Timeout() time.Duration {
	return time.Minute * 60
}

func (t *StartFlowTask) WithAssets() models.Refresh {
	return models.RefreshNone
}

func (t *StartFlowTask) Perform(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets) error {
	if err := createFlowStartBatches(ctx, rt, oa, t.FlowStart); err != nil {
		models.MarkStartFailed(ctx, rt.DB, t.FlowStart.ID)

		// if error is user created query error.. don't escalate error to sentry
		isQueryError, _ := contactql.IsQueryError(err)
		if !isQueryError {
			return err
		}
	}

	return nil
}

// creates batches of flow starts for all the unique contacts
func createFlowStartBatches(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, start *models.FlowStart) error {
	flow, err := oa.FlowByID(start.FlowID)
	if err != nil {
		return fmt.Errorf("error loading flow: %w", err)
	}

	var contactIDs []models.ContactID

	if start.CreateContact {
		// if we are meant to create a new contact, do so
		contact, _, err := models.CreateContact(ctx, rt.DB, oa, models.NilUserID, "", i18n.NilLanguage, models.ContactStatusActive, nil)
		if err != nil {
			return fmt.Errorf("error creating new contact: %w", err)
		}
		contactIDs = []models.ContactID{contact.ID()}
	} else {
		// otherwise resolve recipients across contacts, groups, urns etc

		// queries in start_session flow actions only match a single contact
		limit := -1
		if string(start.Query) != "" && start.StartType == models.StartTypeFlowAction {
			limit = 1
		}

		contactIDs, err = search.ResolveRecipients(ctx, rt, oa, flow, &search.Recipients{
			ContactIDs:      start.ContactIDs,
			GroupIDs:        start.GroupIDs,
			URNs:            start.URNs,
			Query:           string(start.Query),
			Exclusions:      start.Exclusions,
			ExcludeGroupIDs: start.ExcludeGroupIDs,
		}, limit)
		if err != nil {
			return fmt.Errorf("error resolving start recipients: %w", err)
		}
	}

	// mark our start as starting, last task will mark as complete
	err = models.MarkStartStarted(ctx, rt.DB, start.ID, len(contactIDs))
	if err != nil {
		return fmt.Errorf("error marking start as started: %w", err)
	}

	// if there are no contacts to start, mark our start as complete, we are done
	if len(contactIDs) == 0 {
		err = models.MarkStartComplete(ctx, rt.DB, start.ID)
		if err != nil {
			return fmt.Errorf("error marking start as complete: %w", err)
		}
		return nil
	}

	// split the contact ids into batches to become batch tasks
	idBatches := models.ChunkSlice(contactIDs, startBatchSize)

	// by default we start in the batch queue unless we have two or fewer contacts
	q := tasks.BatchQueue
	if len(contactIDs) <= 2 {
		q = tasks.HandlerQueue
	}

	// if this is a big multi batch blast, give it low priority
	priority := queues.DefaultPriority
	if len(idBatches) > 1 {
		priority = queues.LowPriority
	}

	rc := rt.RP.Get()
	defer rc.Close()

	for i, idBatch := range idBatches {
		isLast := (i == len(idBatches)-1)

		batch := start.CreateBatch(idBatch, flow.FlowType(), isLast, len(contactIDs))

		// task is different if we are an IVR flow
		var batchTask tasks.Task
		if flow.FlowType() == models.FlowTypeVoice {
			batchTask = &ivr.StartIVRFlowBatchTask{FlowStartBatch: batch}
		} else {
			batchTask = &StartFlowBatchTask{FlowStartBatch: batch}
		}

		err = tasks.Queue(rc, q, start.OrgID, batchTask, priority)
		if err != nil {
			if i == 0 {
				return fmt.Errorf("error queuing flow start batch: %w", err)
			}
			// if we've already queued other batches.. we don't want to error and have the task be retried
			slog.Error("error queuing flow start batch", "error", err)
		}
	}

	return nil
}
