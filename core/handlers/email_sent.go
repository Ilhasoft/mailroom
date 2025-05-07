package handlers

import (
	"context"
	"log/slog"

	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/events"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
)

func init() {
	models.RegisterEventHandler(events.TypeEmailSent, handleEmailSent)
}

// goflow now sends email so this just logs the event
func handleEmailSent(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, scene *models.Scene, e flows.Event) error {
	event := e.(*events.EmailSentEvent)

	slog.Debug("email sent", "contact", scene.ContactUUID(), "session", scene.SessionUUID(), "subject", event.Subject, "body", event.Body)

	return nil
}
