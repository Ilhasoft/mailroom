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
)

// CreateBroadcasts is our hook for creating broadcasts
var CreateBroadcasts models.SceneCommitHook = &createBroadcasts{}

type createBroadcasts struct{}

func (h *createBroadcasts) Order() int { return 1 }

func (h *createBroadcasts) Apply(ctx context.Context, rt *runtime.Runtime, tx *sqlx.Tx, oa *models.OrgAssets, scenes map[*models.Scene][]any) error {
	rc := rt.RP.Get()
	defer rc.Close()

	// for each of our scene
	for _, es := range scenes {
		for _, e := range es {
			event := e.(*events.BroadcastCreatedEvent)

			// create a non-persistent broadcast
			bcast, err := models.NewBroadcastFromEvent(ctx, tx, oa, event)
			if err != nil {
				return fmt.Errorf("error creating broadcast: %w", err)
			}

			err = tasks.Queue(rc, tasks.BatchQueue, oa.OrgID(), &msgs.SendBroadcastTask{Broadcast: bcast}, false)
			if err != nil {
				return fmt.Errorf("error queuing broadcast task: %w", err)
			}
		}
	}

	return nil
}
