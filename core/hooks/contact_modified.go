package hooks

import (
	"context"
	"fmt"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"

	"github.com/jmoiron/sqlx"
)

// ContactModifiedHook is our hook for contact changes that require an update to modified_on
var ContactModifiedHook models.EventCommitHook = &contactModifiedHook{}

type contactModifiedHook struct{}

// Apply squashes and updates modified_on on all the contacts passed in
func (h *contactModifiedHook) Apply(ctx context.Context, rt *runtime.Runtime, tx *sqlx.Tx, oa *models.OrgAssets, scenes map[*models.Scene][]any) error {
	// our lists of contact ids
	contactIDs := make([]models.ContactID, 0, len(scenes))

	for scene := range scenes {
		contactIDs = append(contactIDs, scene.ContactID())
	}

	err := models.UpdateContactModifiedOn(ctx, tx, contactIDs)
	if err != nil {
		return fmt.Errorf("error updating modified_on on contacts: %w", err)
	}

	return nil
}
