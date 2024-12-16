package hooks

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/goflow/flows/events"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
)

// InsertStartHook is our hook to fire insert our starts
var InsertStartHook models.EventCommitHook = &insertStartHook{}

type insertStartHook struct{}

// Apply inserts our starts
func (h *insertStartHook) Apply(ctx context.Context, rt *runtime.Runtime, tx *sqlx.Tx, oa *models.OrgAssets, scenes map[*models.Scene][]any) error {
	rc := rt.RP.Get()
	defer rc.Close()

	starts := make([]*models.FlowStart, 0, len(scenes))

	// for each of our scene
	for s, es := range scenes {
		for _, e := range es {
			event := e.(*events.SessionTriggeredEvent)

			// look up our flow
			f, err := oa.FlowByUUID(event.Flow.UUID)
			if err != nil {
				return fmt.Errorf("unable to load flow with UUID: %s: %w", event.Flow.UUID, err)
			}
			flow := f.(*models.Flow)

			// load our groups by uuid
			groupIDs := make([]models.GroupID, 0, len(event.Groups))
			for i := range event.Groups {
				group := oa.GroupByUUID(event.Groups[i].UUID)
				if group != nil {
					groupIDs = append(groupIDs, group.ID())
				}
			}

			// load our contacts by uuid
			contactIDs, err := models.GetContactIDsFromReferences(ctx, tx, oa.OrgID(), event.Contacts)
			if err != nil {
				return fmt.Errorf("error loading contacts by reference: %w", err)
			}

			historyJSON, err := jsonx.Marshal(event.History)
			if err != nil {
				return fmt.Errorf("error marshaling session history: %w", err)
			}

			// create our start
			start := models.NewFlowStart(oa.OrgID(), models.StartTypeFlowAction, flow.ID()).
				WithGroupIDs(groupIDs).
				WithContactIDs(contactIDs).
				WithURNs(event.URNs).
				WithQuery(event.ContactQuery).
				WithExcludeInAFlow(event.Exclusions.InAFlow).
				WithCreateContact(event.CreateContact).
				WithParentSummary(event.RunSummary).
				WithSessionHistory(historyJSON)

			starts = append(starts, start)

			// this will add our task for our start after we commit
			s.AppendToEventPostCommitHook(StartStartHook, start)
		}
	}

	// insert all our starts
	err := models.InsertFlowStarts(ctx, tx, starts)
	if err != nil {
		return fmt.Errorf("error inserting flow starts for scene triggers: %w", err)
	}

	return nil
}
