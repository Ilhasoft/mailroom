package web

import (
	"errors"
	"net/http"

	"github.com/nyaruka/goflow/contactql"
	"github.com/nyaruka/mailroom/core/goflow"
	"github.com/nyaruka/mailroom/core/models"
)

// ErrorResponse is the type for our error responses
type ErrorResponse struct {
	Error string         `json:"error"`
	Code  string         `json:"code,omitempty"`
	Extra map[string]any `json:"extra,omitempty"`
}

func ErrorToResponse(err error) (*ErrorResponse, int) {
	var qerr *contactql.QueryError
	if errors.As(err, &qerr) {
		return &ErrorResponse{
			Error: qerr.Error(),
			Code:  "query:" + qerr.Code(),
			Extra: qerr.Extra(),
		}, http.StatusUnprocessableEntity
	}

	var uerr *models.URNError
	if errors.As(err, &uerr) {
		return &ErrorResponse{
			Error: uerr.Error(),
			Code:  "urn:" + uerr.Code,
			Extra: map[string]any{"index": uerr.Index},
		}, http.StatusUnprocessableEntity
	}

	var ferr *goflow.FlowDefError
	if errors.As(err, &ferr) {
		return &ErrorResponse{
			Error: ferr.Error(),
			Code:  "flow:invalid",
		}, http.StatusUnprocessableEntity
	}

	return &ErrorResponse{Error: err.Error()}, http.StatusInternalServerError
}
