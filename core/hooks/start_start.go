package hooks

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/core/tasks/starts"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/mailroom/utils/queues"
)

// StartStartHook is our hook to fire our scene starts
var StartStartHook models.EventCommitHook = &startStartHook{}

type startStartHook struct{}

// Apply queues up our flow starts
func (h *startStartHook) Apply(ctx context.Context, rt *runtime.Runtime, tx *sqlx.Tx, oa *models.OrgAssets, scenes map[*models.Scene][]any) error {
	rc := rt.RP.Get()
	defer rc.Close()

	// for each of our scene
	for _, es := range scenes {
		for _, e := range es {
			start := e.(*models.FlowStart)

			taskQ := tasks.HandlerQueue
			priority := queues.DefaultPriority

			// if we are starting groups, queue to our batch queue instead, but with high priority
			if len(start.GroupIDs) > 0 || start.Query != "" {
				taskQ = tasks.BatchQueue
				priority = queues.HighPriority
			}

			err := tasks.Queue(rc, taskQ, oa.OrgID(), &starts.StartFlowTask{FlowStart: start}, priority)
			if err != nil {
				return fmt.Errorf("error queuing flow start: %w", err)
			}
		}
	}

	return nil
}
