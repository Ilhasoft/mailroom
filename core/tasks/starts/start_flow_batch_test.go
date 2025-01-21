package starts_test

import (
	"testing"

	"github.com/lib/pq"
	"github.com/nyaruka/gocommon/dbutil/assertdb"
	_ "github.com/nyaruka/mailroom/core/handlers"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/core/tasks/starts"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartFlowBatchTask(t *testing.T) {
	ctx, rt := testsuite.Runtime()
	rc := rt.RP.Get()
	defer rc.Close()

	defer testsuite.Reset(testsuite.ResetData | testsuite.ResetRedis)

	// create a start
	start1 := models.NewFlowStart(models.OrgID(1), models.StartTypeManual, testdata.SingleMessage.ID).
		WithContactIDs([]models.ContactID{testdata.Cathy.ID, testdata.Bob.ID, testdata.George.ID, testdata.Alexandria.ID})
	err := models.InsertFlowStarts(ctx, rt.DB, []*models.FlowStart{start1})
	require.NoError(t, err)

	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowstart WHERE id = $1`, start1.ID).Returns("P")

	batch1 := start1.CreateBatch([]models.ContactID{testdata.Cathy.ID, testdata.Bob.ID}, true, false, 4)
	batch2 := start1.CreateBatch([]models.ContactID{testdata.George.ID, testdata.Alexandria.ID}, false, true, 4)

	// start the first batch...
	err = tasks.Queue(rc, tasks.ThrottledQueue, testdata.Org1.ID, &starts.StartFlowBatchTask{FlowStartBatch: batch1}, false)
	assert.NoError(t, err)
	testsuite.FlushTasks(t, rt)

	assertdb.Query(t, rt.DB, `SELECT count(*) FROM flows_flowsession WHERE contact_id = ANY($1) 
		AND status = 'C' AND responded = FALSE AND org_id = 1 AND call_id IS NULL AND output IS NOT NULL`, pq.Array([]models.ContactID{testdata.Cathy.ID, testdata.Bob.ID})).
		Returns(2)

	assertdb.Query(t, rt.DB, `SELECT count(*) FROM flows_flowrun WHERE contact_id = ANY($1) and flow_id = $2 AND responded = FALSE AND org_id = 1 AND status = 'C'
		AND results IS NOT NULL AND path_nodes IS NOT NULL AND session_id IS NOT NULL`, pq.Array([]models.ContactID{testdata.Cathy.ID, testdata.Bob.ID}), testdata.SingleMessage.ID).
		Returns(2)

	assertdb.Query(t, rt.DB, `SELECT count(*) FROM msgs_msg WHERE contact_id = ANY($1) AND text = 'Hey, how are you?' AND org_id = 1 AND status = 'Q' 
		AND direction = 'O' AND msg_type = 'T'`, pq.Array([]models.ContactID{testdata.Cathy.ID, testdata.Bob.ID})).
		Returns(2)

	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowstart WHERE id = $1`, start1.ID).Returns("S")

	// start the second and final batch...
	err = tasks.Queue(rc, tasks.ThrottledQueue, testdata.Org1.ID, &starts.StartFlowBatchTask{FlowStartBatch: batch2}, false)
	assert.NoError(t, err)
	testsuite.FlushTasks(t, rt)

	assertdb.Query(t, rt.DB, `SELECT count(*) FROM flows_flowrun WHERE start_id = $1`, start1.ID).Returns(4)
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowstart WHERE id = $1`, start1.ID).Returns("C")

	// create a second start
	start2 := models.NewFlowStart(models.OrgID(1), models.StartTypeManual, testdata.SingleMessage.ID).
		WithContactIDs([]models.ContactID{testdata.Cathy.ID, testdata.Bob.ID, testdata.George.ID, testdata.Alexandria.ID})
	err = models.InsertFlowStarts(ctx, rt.DB, []*models.FlowStart{start2})
	require.NoError(t, err)

	start2Batch1 := start2.CreateBatch([]models.ContactID{testdata.Cathy.ID, testdata.Bob.ID}, true, false, 4)
	start2Batch2 := start2.CreateBatch([]models.ContactID{testdata.George.ID, testdata.Alexandria.ID}, false, true, 4)

	// start the first batch...
	err = tasks.Queue(rc, tasks.ThrottledQueue, testdata.Org1.ID, &starts.StartFlowBatchTask{FlowStartBatch: start2Batch1}, false)
	assert.NoError(t, err)
	testsuite.FlushTasks(t, rt)

	assertdb.Query(t, rt.DB, `SELECT count(*) FROM flows_flowrun WHERE start_id = $1`, start2.ID).Returns(2)

	// interrupt the start
	rt.DB.MustExec(`UPDATE flows_flowstart SET status = 'I' WHERE id = $1`, start2.ID)

	// start the second batch...
	err = tasks.Queue(rc, tasks.ThrottledQueue, testdata.Org1.ID, &starts.StartFlowBatchTask{FlowStartBatch: start2Batch2}, false)
	assert.NoError(t, err)
	testsuite.FlushTasks(t, rt)

	// check that second batch didn't create any runs and start status is still interrupted
	assertdb.Query(t, rt.DB, `SELECT count(*) FROM flows_flowrun WHERE start_id = $1`, start2.ID).Returns(2)
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowstart WHERE id = $1`, start2.ID).Returns("I")
}

func TestStartFlowBatchTaskNonPersistedStart(t *testing.T) {
	_, rt := testsuite.Runtime()
	rc := rt.RP.Get()
	defer rc.Close()

	defer testsuite.Reset(testsuite.ResetData)

	// create a start
	start := models.NewFlowStart(models.OrgID(1), models.StartTypeManual, testdata.SingleMessage.ID).
		WithContactIDs([]models.ContactID{testdata.Cathy.ID, testdata.Bob.ID, testdata.George.ID, testdata.Alexandria.ID})

	batch := start.CreateBatch([]models.ContactID{testdata.Cathy.ID, testdata.Bob.ID}, true, true, 2)

	// start the first batch...
	err := tasks.Queue(rc, tasks.ThrottledQueue, testdata.Org1.ID, &starts.StartFlowBatchTask{FlowStartBatch: batch}, false)
	assert.NoError(t, err)
	testsuite.FlushTasks(t, rt)

	assertdb.Query(t, rt.DB, `SELECT count(*) FROM flows_flowrun`).Returns(2)
}