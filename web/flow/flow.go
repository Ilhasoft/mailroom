package flow

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/goflow/envs"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/mailroom/core/goflow"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/mailroom/web"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
)

func init() {
	web.RegisterJSONRoute(http.MethodPost, "/mr/flow/migrate", web.RequireAuthToken(handleMigrate))
	web.RegisterJSONRoute(http.MethodPost, "/mr/flow/inspect", web.RequireAuthToken(handleInspect))
	web.RegisterJSONRoute(http.MethodPost, "/mr/flow/clone", web.RequireAuthToken(handleClone))
	web.RegisterJSONRoute(http.MethodPost, "/mr/flow/change_language", web.RequireAuthToken(handleChangeLanguage))
}

// Migrates a flow to the latest flow specification
//
//	{
//	  "flow": {"uuid": "468621a8-32e6-4cd2-afc1-04416f7151f0", "action_sets": [], ...},
//	  "to_version": "13.0.0"
//	}
type migrateRequest struct {
	Flow      json.RawMessage `json:"flow" validate:"required"`
	ToVersion *semver.Version `json:"to_version"`
}

func handleMigrate(ctx context.Context, rt *runtime.Runtime, r *http.Request) (interface{}, int, error) {
	request := &migrateRequest{}
	if err := web.ReadAndValidateJSON(r, request); err != nil {
		return errors.Wrapf(err, "request failed validation"), http.StatusBadRequest, nil
	}

	// do a JSON to JSON migration of the definition
	migrated, err := goflow.MigrateDefinition(rt.Config, request.Flow, request.ToVersion)
	if err != nil {
		return errors.Wrapf(err, "unable to migrate flow"), http.StatusUnprocessableEntity, nil
	}

	// try to read result to check that it's valid
	_, err = goflow.ReadFlow(rt.Config, migrated)
	if err != nil {
		return errors.Wrapf(err, "unable to read migrated flow"), http.StatusUnprocessableEntity, nil
	}

	return migrated, http.StatusOK, nil
}

// Inspects a flow, and returns metadata including the possible results generated by the flow,
// and dependencies in the flow. If `org_id` is specified then the dependencies will be checked
// to see if they exist in the org assets.
//
//	{
//	  "flow": { "uuid": "468621a8-32e6-4cd2-afc1-04416f7151f0", "nodes": [...]},
//	  "org_id": 1
//	}
type inspectRequest struct {
	Flow  json.RawMessage `json:"flow" validate:"required"`
	OrgID models.OrgID    `json:"org_id"`
}

func handleInspect(ctx context.Context, rt *runtime.Runtime, r *http.Request) (interface{}, int, error) {
	request := &inspectRequest{}
	if err := web.ReadAndValidateJSON(r, request); err != nil {
		return errors.Wrapf(err, "request failed validation"), http.StatusBadRequest, nil
	}

	flow, err := goflow.ReadFlow(rt.Config, request.Flow)
	if err != nil {
		return errors.Wrapf(err, "unable to read flow"), http.StatusUnprocessableEntity, nil
	}

	var sa flows.SessionAssets
	// if we have an org ID, create session assets to look for missing dependencies
	if request.OrgID != models.NilOrgID {
		oa, err := models.GetOrgAssetsWithRefresh(ctx, rt, request.OrgID, models.RefreshFields|models.RefreshGroups|models.RefreshFlows)
		if err != nil {
			return nil, 0, err
		}
		sa = oa.SessionAssets()
	}

	return flow.Inspect(sa), http.StatusOK, nil
}

// Clones a flow, replacing all UUIDs with either the given mapping or new random UUIDs.
//
//	{
//	  "dependency_mapping": {
//	    "4ee4189e-0c06-4b00-b54f-5621329de947": "db31d23f-65b8-4518-b0f6-45638bfbbbf2",
//	    "723e62d8-a544-448f-8590-1dfd0fccfcd4": "f1fd861c-9e75-4376-a829-dcf76db6e721"
//	  },
//	  "flow": { "uuid": "468621a8-32e6-4cd2-afc1-04416f7151f0", "nodes": [...]}
//	}
type cloneRequest struct {
	DependencyMapping map[uuids.UUID]uuids.UUID `json:"dependency_mapping"`
	Flow              json.RawMessage           `json:"flow" validate:"required"`
}

func handleClone(ctx context.Context, rt *runtime.Runtime, r *http.Request) (interface{}, int, error) {
	request := &cloneRequest{}
	if err := web.ReadAndValidateJSON(r, request); err != nil {
		return errors.Wrapf(err, "request failed validation"), http.StatusBadRequest, nil
	}

	// try to clone the flow definition
	cloneJSON, err := goflow.CloneDefinition(request.Flow, request.DependencyMapping)
	if err != nil {
		return errors.Wrapf(err, "unable to read flow"), http.StatusUnprocessableEntity, nil
	}

	// read flow to check that cloning produced something valid
	_, err = goflow.ReadFlow(rt.Config, cloneJSON)
	if err != nil {
		return errors.Wrapf(err, "unable to clone flow"), http.StatusUnprocessableEntity, nil
	}

	return cloneJSON, http.StatusOK, nil
}

// Changes the language of a flow by replacing the text with a translation.
//
//	{
//	  "language": "spa",
//	  "flow": { "uuid": "468621a8-32e6-4cd2-afc1-04416f7151f0", "nodes": [...]}
//	}
type changeLanguageRequest struct {
	Language envs.Language   `json:"language" validate:"required"`
	Flow     json.RawMessage `json:"flow"     validate:"required"`
}

func handleChangeLanguage(ctx context.Context, rt *runtime.Runtime, r *http.Request) (interface{}, int, error) {
	request := &changeLanguageRequest{}
	if err := web.ReadAndValidateJSON(r, request); err != nil {
		return errors.Wrapf(err, "request failed validation"), http.StatusBadRequest, nil
	}

	flow, err := goflow.ReadFlow(rt.Config, request.Flow)
	if err != nil {
		return errors.Wrapf(err, "unable to read flow"), http.StatusUnprocessableEntity, nil
	}

	copy, err := flow.ChangeLanguage(request.Language)
	if err != nil {
		return errors.Wrapf(err, "unable to change flow language"), http.StatusUnprocessableEntity, nil
	}

	return copy, http.StatusOK, nil
}
