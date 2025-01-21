package handlers

import (
	"context"
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/events"
	"github.com/nyaruka/mailroom/core/hooks"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
)

func init() {
	models.RegisterEventHandler(events.TypeBroadcastCreated, handleBroadcastCreated)
}

func handleBroadcastCreated(ctx context.Context, rt *runtime.Runtime, tx *sqlx.Tx, oa *models.OrgAssets, scene *models.Scene, e flows.Event) error {
	event := e.(*events.BroadcastCreatedEvent)

	slog.Debug("broadcast created", "contact", scene.ContactUUID(), "session", scene.SessionID(), "translations", event.Translations[event.BaseLanguage])

	scene.AppendToEventPostCommitHook(hooks.CreateBroadcastsHook, event)

	return nil
}
