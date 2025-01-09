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
	models.RegisterEventHandler(events.TypeMsgReceived, handleMsgReceived)
}

// handleMsgReceived takes care of update last seen on and any campaigns based on that
func handleMsgReceived(ctx context.Context, rt *runtime.Runtime, tx *sqlx.Tx, oa *models.OrgAssets, scene *models.Scene, e flows.Event) error {
	event := e.(*events.MsgReceivedEvent)

	slog.Debug("msg received", "contact", scene.ContactUUID(), "session", scene.SessionID(), "text", event.Msg.Text(), "urn", event.Msg.URN())

	// update the contact's last seen date
	scene.AppendToEventPreCommitHook(hooks.ContactLastSeenHook, event)
	scene.AppendToEventPreCommitHook(hooks.UpdateCampaignEventsHook, event)

	return nil
}
