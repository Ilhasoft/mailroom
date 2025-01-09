package hooks

import (
	"context"
	"fmt"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"

	"github.com/jmoiron/sqlx"
)

// InsertWebhookEventHook is our hook for when a resthook needs to be inserted
var InsertWebhookEventHook models.EventCommitHook = &insertWebhookEventHook{}

type insertWebhookEventHook struct{}

// Apply inserts all the webook events that were created
func (h *insertWebhookEventHook) Apply(ctx context.Context, rt *runtime.Runtime, tx *sqlx.Tx, oa *models.OrgAssets, scenes map[*models.Scene][]any) error {
	events := make([]*models.WebhookEvent, 0, len(scenes))
	for _, rs := range scenes {
		for _, r := range rs {
			events = append(events, r.(*models.WebhookEvent))
		}
	}

	err := models.InsertWebhookEvents(ctx, tx, events)
	if err != nil {
		return fmt.Errorf("error inserting webhook events: %w", err)
	}

	return nil
}
