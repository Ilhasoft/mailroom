package hooks

import (
	"context"
	"fmt"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"

	"github.com/jmoiron/sqlx"
)

// InsertAirtimeTransfersHook is our hook for inserting airtime transfers
var InsertAirtimeTransfersHook models.EventCommitHook = &insertAirtimeTransfersHook{}

type insertAirtimeTransfersHook struct{}

// Apply inserts all the airtime transfers that were created
func (h *insertAirtimeTransfersHook) Apply(ctx context.Context, rt *runtime.Runtime, tx *sqlx.Tx, oa *models.OrgAssets, scenes map[*models.Scene][]any) error {
	// gather all our transfers
	transfers := make([]*models.AirtimeTransfer, 0, len(scenes))

	for _, ts := range scenes {
		for _, t := range ts {
			transfer := t.(*models.AirtimeTransfer)
			transfers = append(transfers, transfer)
		}
	}

	// insert the transfers
	err := models.InsertAirtimeTransfers(ctx, tx, transfers)
	if err != nil {
		return fmt.Errorf("error inserting airtime transfers: %w", err)
	}

	// gather all our logs and set the newly inserted transfer IDs on them
	logs := make([]*models.HTTPLog, 0, len(scenes))

	for _, t := range transfers {
		for _, l := range t.Logs {
			l.SetAirtimeTransferID(t.ID())
			logs = append(logs, l)
		}
	}

	// insert the logs
	err = models.InsertHTTPLogs(ctx, tx, logs)
	if err != nil {
		return fmt.Errorf("error inserting airtime transfer logs: %w", err)
	}

	return nil
}
