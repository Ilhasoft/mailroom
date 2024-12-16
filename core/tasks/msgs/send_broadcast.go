package msgs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nyaruka/goflow/contactql"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/search"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/mailroom/utils/queues"
)

const (
	// TypeSendBroadcast is the task type for sending a broadcast
	TypeSendBroadcast = "send_broadcast"

	startBatchSize = 100
)

func init() {
	tasks.RegisterType(TypeSendBroadcast, func() tasks.Task { return &SendBroadcastTask{} })
}

// SendBroadcastTask is the task send broadcasts
type SendBroadcastTask struct {
	*models.Broadcast
}

func (t *SendBroadcastTask) Type() string {
	return TypeSendBroadcast
}

// Timeout is the maximum amount of time the task can run for
func (t *SendBroadcastTask) Timeout() time.Duration {
	return time.Minute * 60
}

func (t *SendBroadcastTask) WithAssets() models.Refresh {
	return models.RefreshNone
}

// Perform handles sending the broadcast by creating batches of broadcast sends for all the unique contacts
func (t *SendBroadcastTask) Perform(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets) error {
	if err := createBroadcastBatches(ctx, rt, oa, t.Broadcast); err != nil {
		if t.Broadcast.ID != models.NilBroadcastID {
			models.MarkBroadcastFailed(ctx, rt.DB, t.Broadcast.ID)
		}

		// if error is user created query error.. don't escalate error to sentry
		isQueryError, _ := contactql.IsQueryError(err)
		if !isQueryError {
			return err
		}
	}

	return nil
}

func createBroadcastBatches(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, bcast *models.Broadcast) error {
	contactIDs, err := search.ResolveRecipients(ctx, rt, oa, nil, &search.Recipients{
		ContactIDs:      bcast.ContactIDs,
		GroupIDs:        bcast.GroupIDs,
		URNs:            bcast.URNs,
		Query:           string(bcast.Query),
		Exclusions:      bcast.Exclusions,
		ExcludeGroupIDs: nil,
	}, -1)
	if err != nil {
		return fmt.Errorf("error resolving broadcast recipients: %w", err)
	}

	// if a node is specified, add all the contacts at that node
	if bcast.NodeUUID != "" {
		nodeContactIDs, err := models.GetContactIDsAtNode(ctx, rt, oa.OrgID(), bcast.NodeUUID)
		if err != nil {
			return fmt.Errorf("error getting contacts at node %s: %w", bcast.NodeUUID, err)
		}

		contactIDs = append(contactIDs, nodeContactIDs...)
	}

	// if there are no contacts to send to, mark our broadcast as sent, we are done
	if len(contactIDs) == 0 {
		if bcast.ID != models.NilBroadcastID {
			err = models.MarkBroadcastSent(ctx, rt.DB, bcast.ID)
			if err != nil {
				return fmt.Errorf("error marking broadcast as sent: %w", err)
			}
		}
		return nil
	}

	// two or fewer contacts? queue to our handler queue for sending
	q := tasks.BatchQueue
	if len(contactIDs) <= 2 {
		q = tasks.HandlerQueue
	}

	rc := rt.RP.Get()
	defer rc.Close()

	// create tasks for batches of contacts
	idBatches := models.ChunkSlice(contactIDs, startBatchSize)
	for i, idBatch := range idBatches {
		isLast := (i == len(idBatches)-1)

		batch := bcast.CreateBatch(idBatch, isLast)
		err = tasks.Queue(rc, q, bcast.OrgID, &SendBroadcastBatchTask{BroadcastBatch: batch}, queues.DefaultPriority)
		if err != nil {
			if i == 0 {
				return fmt.Errorf("error queuing broadcast batch: %w", err)
			}
			// if we've already queued other batches.. we don't want to error and have the task be retried
			slog.Error("error queuing broadcast batch", "error", err)
		}
	}

	return nil
}
