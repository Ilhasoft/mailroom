package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/events"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/null/v3"
)

type FlowRunID int64

const NilFlowRunID = FlowRunID(0)

type RunStatus string

const (
	RunStatusActive      RunStatus = "A"
	RunStatusWaiting     RunStatus = "W"
	RunStatusCompleted   RunStatus = "C"
	RunStatusExpired     RunStatus = "X"
	RunStatusInterrupted RunStatus = "I"
	RunStatusFailed      RunStatus = "F"
)

var runStatusMap = map[flows.RunStatus]RunStatus{
	flows.RunStatusActive:    RunStatusActive,
	flows.RunStatusWaiting:   RunStatusWaiting,
	flows.RunStatusCompleted: RunStatusCompleted,
	flows.RunStatusExpired:   RunStatusExpired,
	flows.RunStatusFailed:    RunStatusFailed,
}

// FlowRun is the mailroom type for a FlowRun
type FlowRun struct {
	r struct {
		ID              FlowRunID       `db:"id"`
		UUID            flows.RunUUID   `db:"uuid"`
		Status          RunStatus       `db:"status"`
		CreatedOn       time.Time       `db:"created_on"`
		ModifiedOn      time.Time       `db:"modified_on"`
		ExitedOn        *time.Time      `db:"exited_on"`
		Responded       bool            `db:"responded"`
		Results         string          `db:"results"`
		Path            string          `db:"path"`
		CurrentNodeUUID null.String     `db:"current_node_uuid"`
		ContactID       flows.ContactID `db:"contact_id"`
		FlowID          FlowID          `db:"flow_id"`
		OrgID           OrgID           `db:"org_id"`
		SessionID       SessionID       `db:"session_id"`
		StartID         StartID         `db:"start_id"`
	}

	// we keep a reference to the engine's run
	run flows.Run
}

func (r *FlowRun) SetSessionID(sessionID SessionID) { r.r.SessionID = sessionID }
func (r *FlowRun) SetStartID(startID StartID)       { r.r.StartID = startID }
func (r *FlowRun) UUID() flows.RunUUID              { return r.r.UUID }
func (r *FlowRun) ModifiedOn() time.Time            { return r.r.ModifiedOn }

// MarshalJSON is our custom marshaller so that our inner struct get output
func (r *FlowRun) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.r)
}

// UnmarshalJSON is our custom marshaller so that our inner struct get output
func (r *FlowRun) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &r.r)
}

// Step represents a single step in a run, this struct is used for serialization to the steps
type Step struct {
	UUID      flows.StepUUID `json:"uuid"`
	NodeUUID  flows.NodeUUID `json:"node_uuid"`
	ArrivedOn time.Time      `json:"arrived_on"`
	ExitUUID  flows.ExitUUID `json:"exit_uuid,omitempty"`
}

const sqlInsertRun = `
INSERT INTO
flows_flowrun(uuid, created_on, modified_on, exited_on, status, responded, results, path, 
	          current_node_uuid, contact_id, flow_id, org_id, session_id, start_id)
	   VALUES(:uuid, :created_on, NOW(), :exited_on, :status, :responded, :results, :path,
	          :current_node_uuid, :contact_id, :flow_id, :org_id, :session_id, :start_id)
RETURNING id
`

// newRun writes the passed in flow run to our database, also applying any events in those runs as
// appropriate. (IE, writing db messages etc..)
func newRun(ctx context.Context, tx *sqlx.Tx, oa *OrgAssets, session *Session, fr flows.Run) (*FlowRun, error) {
	// build our path elements
	path := make([]Step, len(fr.Path()))
	for i, p := range fr.Path() {
		path[i].UUID = p.UUID()
		path[i].NodeUUID = p.NodeUUID()
		path[i].ArrivedOn = p.ArrivedOn()
		path[i].ExitUUID = p.ExitUUID()
	}

	flowID, err := FlowIDForUUID(ctx, tx, oa, fr.FlowReference().UUID)
	if err != nil {
		return nil, fmt.Errorf("unable to load flow with uuid: %s: %w", fr.FlowReference().UUID, err)
	}

	// create our run
	run := &FlowRun{}
	r := &run.r
	r.UUID = fr.UUID()
	r.Status = runStatusMap[fr.Status()]
	r.CreatedOn = fr.CreatedOn()
	r.ExitedOn = fr.ExitedOn()
	r.ModifiedOn = fr.ModifiedOn()
	r.ContactID = fr.Contact().ID()
	r.FlowID = flowID
	r.SessionID = session.ID()
	r.StartID = NilStartID
	r.OrgID = oa.OrgID()
	r.Path = string(jsonx.MustMarshal(path))
	r.Results = string(jsonx.MustMarshal(fr.Results()))

	if len(path) > 0 {
		r.CurrentNodeUUID = null.String(path[len(path)-1].NodeUUID)
	}
	run.run = fr

	// mark ourselves as responded if we received a message
	for _, e := range fr.Events() {
		if e.Type() == events.TypeMsgReceived {
			r.Responded = true
			break
		}
	}

	return run, nil
}

// GetContactIDsAtNode returns the ids of contacts currently waiting or active at the given flow node
func GetContactIDsAtNode(ctx context.Context, rt *runtime.Runtime, orgID OrgID, nodeUUID flows.NodeUUID) ([]ContactID, error) {
	rows, err := rt.ReadonlyDB.QueryContext(ctx,
		`SELECT contact_id FROM flows_flowrun WHERE org_id = $1 AND current_node_uuid = $2 AND status IN ('A' , 'W')`, orgID, nodeUUID,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("error querying contacts at node: %w", err)
	}
	defer rows.Close()

	contactIDs := make([]ContactID, 0, 10)

	for rows.Next() {
		var id ContactID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("error scanning contact id: %w", err)
		}
		contactIDs = append(contactIDs, id)
	}

	return contactIDs, nil
}
