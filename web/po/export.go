package po

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/middleware"
	"github.com/nyaruka/gocommon/i18n"
	"github.com/nyaruka/goflow/flows/translation"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/mailroom/web"
	"github.com/pkg/errors"
)

func init() {
	web.RegisterRoute(http.MethodPost, "/mr/po/export", web.RequireAuthToken(handleExport))
}

// Exports a PO file from the given set of flows.
//
//	{
//	  "org_id": 123,
//	  "flow_ids": [123, 354, 456],
//	  "language": "spa"
//	}
type exportRequest struct {
	OrgID    models.OrgID    `json:"org_id"  validate:"required"`
	FlowIDs  []models.FlowID `json:"flow_ids" validate:"required"`
	Language i18n.Language   `json:"language" validate:"omitempty,language"`
}

func handleExport(ctx context.Context, rt *runtime.Runtime, r *http.Request, rawW http.ResponseWriter) error {
	request := &exportRequest{}
	if err := web.ReadAndValidateJSON(r, request); err != nil {
		return errors.Wrapf(err, "request failed validation")
	}

	flows, err := loadFlows(ctx, rt, request.OrgID, request.FlowIDs)
	if err != nil {
		return err
	}

	// extract everything the engine considers localizable except router arguments
	po, err := translation.ExtractFromFlows("Generated by mailroom", request.Language, excludeProperties, flows...)
	if err != nil {
		return errors.Wrapf(err, "unable to extract PO from flows")
	}

	w := middleware.NewWrapResponseWriter(rawW, r.ProtoMajor)
	w.Header().Set("Content-type", "text/x-gettext-translation")
	w.WriteHeader(http.StatusOK)
	po.Write(w)
	return nil
}
