package timeouts

import (
	"context"
	"fmt"
	"time"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/core/tasks/handler"
	"github.com/nyaruka/mailroom/core/tasks/handler/ctasks"
	"github.com/nyaruka/mailroom/runtime"
)

// TypeBulkTimeout is the type of the task
const TypeBulkTimeout = "bulk_timeout"

func init() {
	tasks.RegisterType(TypeBulkTimeout, func() tasks.Task { return &BulkTimeoutTask{} })
}

// BulkTimeoutTask is the payload of the task
type BulkTimeoutTask struct {
	Timeouts []*Timeout `json:"timeouts"`
}

func (t *BulkTimeoutTask) Type() string {
	return TypeBulkTimeout
}

// Timeout is the maximum amount of time the task can run for
func (t *BulkTimeoutTask) Timeout() time.Duration {
	return time.Hour
}

func (t *BulkTimeoutTask) WithAssets() models.Refresh {
	return models.RefreshNone
}

// Perform creates the actual task
func (t *BulkTimeoutTask) Perform(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets) error {
	rc := rt.RP.Get()
	defer rc.Close()

	for _, timeout := range t.Timeouts {
		err := handler.QueueTask(rc, oa.OrgID(), timeout.ContactID, ctasks.NewWaitTimeout(timeout.SessionID, timeout.TimeoutOn))
		if err != nil {
			return fmt.Errorf("error queuing handle task for timeout on session #%d: %w", timeout.SessionID, err)
		}
	}

	return nil
}
