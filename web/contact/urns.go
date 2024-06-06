package contact

import (
	"context"
	"fmt"
	"net/http"

	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/mailroom/web"
	"golang.org/x/exp/maps"
)

func init() {
	web.RegisterRoute(http.MethodPost, "/mr/contact/urns", web.RequireAuthToken(web.JSONPayload(handleURNs)))
}

// Request to validate a set of URNs and determine ownership.
//
//	{
//	  "org_id": 1,
//	  "urns": ["tel:+593 979 123456", "webchat:123456", "line:1234567890"]
//	}
//
//	{
//	  "urns": [
//	    {"normalized": "tel:+593979123456", "contact_id": 35657},
//	    {"normalized": "webchat:123456", "error": "invalid path component"}
//	    {"normalized": "line:1234567890"}
//	  ]
//	}
type urnsRequest struct {
	OrgID models.OrgID `json:"org_id"   validate:"required"`
	URNs  []urns.URN   `json:"urns"  validate:"required"`
}

type urnResult struct {
	Normalized urns.URN         `json:"normalized"`
	ContactID  models.ContactID `json:"contact_id,omitempty"`
	Error      string           `json:"error,omitempty"`
}

// handles a request to create the given contact
func handleURNs(ctx context.Context, rt *runtime.Runtime, r *urnsRequest) (any, int, error) {
	urnsToLookup := make(map[urns.URN]int, len(r.URNs)) // normalized to index of valid URNs
	results := make([]urnResult, len(r.URNs))

	for i, urn := range r.URNs {
		norm := urn.Normalize()

		results[i].Normalized = norm

		if err := norm.Validate(); err != nil {
			results[i].Error = err.Error()
		} else {
			urnsToLookup[norm] = i
		}
	}

	ownerIDs, err := models.GetContactIDsFromURNs(ctx, rt.DB, r.OrgID, maps.Keys(urnsToLookup))
	if err != nil {
		return nil, 0, fmt.Errorf("error getting URN owners: %w", err)
	}

	for nurn, ownerID := range ownerIDs {
		results[urnsToLookup[nurn]].ContactID = ownerID
	}

	return map[string]any{"urns": results}, http.StatusOK, nil
}
