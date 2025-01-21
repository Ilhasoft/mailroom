package expirations

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

// TypeBulkExpire is the type of the task
const TypeBulkExpire = "bulk_expire"

func init() {
	tasks.RegisterType(TypeBulkExpire, func() tasks.Task { return &BulkExpireTask{} })
}

// BulkExpireTask is the payload of the task
type BulkExpireTask struct {
	Expirations []*ExpiredWait `json:"expirations"`
}

func (t *BulkExpireTask) Type() string {
	return TypeBulkExpire
}

// Timeout is the maximum amount of time the task can run for
func (t *BulkExpireTask) Timeout() time.Duration {
	return time.Hour
}

func (t *BulkExpireTask) WithAssets() models.Refresh {
	return models.RefreshNone
}

// Perform creates the actual task
func (t *BulkExpireTask) Perform(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets) error {
	rc := rt.RP.Get()
	defer rc.Close()

	for _, exp := range t.Expirations {
		err := handler.QueueTask(rc, oa.OrgID(), exp.ContactID, ctasks.NewWaitExpiration(exp.SessionID, exp.WaitExpiresOn))
		if err != nil {
			return fmt.Errorf("error queuing handle task for expiration on session #%d: %w", exp.SessionID, err)
		}
	}

	return nil
}
