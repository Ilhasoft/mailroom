package hooks

import (
	"context"
	"fmt"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"

	"github.com/jmoiron/sqlx"
)

// InsertHTTPLogsHook is our hook for inserting classifier logs
var InsertHTTPLogsHook models.EventCommitHook = &insertHTTPLogsHook{}

type insertHTTPLogsHook struct{}

// Apply inserts all the classifier logs that were created
func (h *insertHTTPLogsHook) Apply(ctx context.Context, rt *runtime.Runtime, tx *sqlx.Tx, oa *models.OrgAssets, scenes map[*models.Scene][]any) error {
	// gather all our logs
	logs := make([]*models.HTTPLog, 0, len(scenes))
	for _, ls := range scenes {
		for _, l := range ls {
			logs = append(logs, l.(*models.HTTPLog))
		}
	}

	err := models.InsertHTTPLogs(ctx, tx, logs)
	if err != nil {
		return fmt.Errorf("error inserting http logs: %w", err)
	}

	return nil
}
