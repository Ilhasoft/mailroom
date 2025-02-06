package handlers

import (
	"context"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/events"
	"github.com/nyaruka/mailroom/core/hooks"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/shopspring/decimal"
)

func init() {
	models.RegisterEventHandler(events.TypeAirtimeTransferred, handleAirtimeTransferred)
}

// handleAirtimeTransferred is called for each airtime transferred event
func handleAirtimeTransferred(ctx context.Context, rt *runtime.Runtime, tx *sqlx.Tx, oa *models.OrgAssets, scene *models.Scene, e flows.Event) error {
	event := e.(*events.AirtimeTransferredEvent)

	slog.Debug("airtime transferred", "contact", scene.ContactUUID(), "session", scene.SessionID(), "sender", event.Sender, "recipient", event.Recipient, "currency", event.Currency, "amount", event.Amount.String())

	status := models.AirtimeTransferStatusSuccess
	if event.Amount == decimal.Zero {
		status = models.AirtimeTransferStatusFailed
	}

	transfer := models.NewAirtimeTransfer(
		event.TransferUUID,
		oa.OrgID(),
		status,
		event.ExternalID,
		scene.ContactID(),
		event.Sender,
		event.Recipient,
		event.Currency,
		event.Amount,
		event.CreatedOn(),
	)

	// add a log for each HTTP call
	for _, httpLog := range event.HTTPLogs {
		transfer.AddLog(models.NewAirtimeTransferredLog(
			oa.OrgID(),
			httpLog.URL,
			httpLog.StatusCode,
			httpLog.Request,
			httpLog.Response,
			httpLog.Status != flows.CallStatusSuccess,
			time.Duration(httpLog.ElapsedMS)*time.Millisecond,
			httpLog.Retries,
			httpLog.CreatedOn,
		))
	}

	scene.AppendToEventPreCommitHook(hooks.InsertAirtimeTransfersHook, transfer)

	return nil
}
