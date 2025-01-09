package hooks

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/goflow/flows/events"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/core/tasks/msgs"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/mailroom/utils/queues"
)

// StartBroadcastsHook is our hook for starting broadcasts
var StartBroadcastsHook models.EventCommitHook = &startBroadcastsHook{}

type startBroadcastsHook struct{}

// Apply queues up our broadcasts for sending
func (h *startBroadcastsHook) Apply(ctx context.Context, rt *runtime.Runtime, tx *sqlx.Tx, oa *models.OrgAssets, scenes map[*models.Scene][]any) error {
	rc := rt.RP.Get()
	defer rc.Close()

	// for each of our scene
	for _, es := range scenes {
		for _, e := range es {
			event := e.(*events.BroadcastCreatedEvent)

			bcast, err := models.NewBroadcastFromEvent(ctx, tx, oa, event)
			if err != nil {
				return fmt.Errorf("error creating broadcast: %w", err)
			}

			taskQ := tasks.HandlerQueue
			priority := queues.DefaultPriority

			// if we are starting groups, queue to our batch queue instead, but with high priority
			if len(bcast.GroupIDs) > 0 {
				taskQ = tasks.BatchQueue
				priority = queues.HighPriority
			}

			err = tasks.Queue(rc, taskQ, oa.OrgID(), &msgs.SendBroadcastTask{Broadcast: bcast}, priority)
			if err != nil {
				return fmt.Errorf("error queuing broadcast task: %w", err)
			}
		}
	}

	return nil
}
