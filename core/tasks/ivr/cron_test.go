package ivr_test

import (
	"encoding/json"
	"testing"

	"github.com/nyaruka/gocommon/dbutil/assertdb"
	"github.com/nyaruka/mailroom/core/ivr"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/queue"
	ivrtasks "github.com/nyaruka/mailroom/core/tasks/ivr"
	"github.com/nyaruka/mailroom/core/tasks/starts"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/stretchr/testify/assert"
)

func TestRetries(t *testing.T) {
	ctx, rt, db, rp := testsuite.Get()
	rc := rp.Get()
	defer rc.Close()

	defer testsuite.Reset(testsuite.ResetAll)

	// register our mock client
	ivr.RegisterServiceType(models.ChannelType("ZZ"), NewMockProvider)

	// update our twilio channel to be of type 'ZZ' and set max_concurrent_events to 1
	db.MustExec(`UPDATE channels_channel SET channel_type = 'ZZ', config = '{"max_concurrent_events": 1}' WHERE id = $1`, testdata.TwilioChannel.ID)

	// create a flow start for cathy
	start := models.NewFlowStart(testdata.Org1.ID, models.StartTypeTrigger, models.FlowTypeVoice, testdata.IVRFlow.ID).
		WithContactIDs([]models.ContactID{testdata.Cathy.ID})

	// call our master starter
	err := starts.CreateFlowBatches(ctx, rt, start)
	assert.NoError(t, err)

	// should have one task in our ivr queue
	task, err := queue.PopNextTask(rc, queue.HandlerQueue)
	assert.NoError(t, err)
	batch := &models.FlowStartBatch{}
	err = json.Unmarshal(task.Task, batch)
	assert.NoError(t, err)

	client.callError = nil
	client.callID = ivr.CallID("call1")
	err = ivrtasks.HandleFlowStartBatch(ctx, rt, batch)
	assert.NoError(t, err)
	assertdb.Query(t, db, `SELECT COUNT(*) FROM channels_channelconnection WHERE contact_id = $1 AND status = $2 AND external_id = $3`,
		testdata.Cathy.ID, models.ConnectionStatusWired, "call1").Returns(1)

	// change our call to be errored instead of wired
	db.MustExec(`UPDATE channels_channelconnection SET status = 'E', next_attempt = NOW() WHERE external_id = 'call1';`)

	// fire our retries
	err = ivrtasks.RetryCalls(ctx, rt)
	assert.NoError(t, err)

	// should now be in wired state
	assertdb.Query(t, db, `SELECT COUNT(*) FROM channels_channelconnection WHERE contact_id = $1 AND status = $2 AND external_id = $3`,
		testdata.Cathy.ID, models.ConnectionStatusWired, "call1").Returns(1)

	// back to retry and make the channel inactive
	db.MustExec(`UPDATE channels_channelconnection SET status = 'E', next_attempt = NOW() WHERE external_id = 'call1';`)
	db.MustExec(`UPDATE channels_channel SET is_active = FALSE WHERE id = $1`, testdata.TwilioChannel.ID)

	models.FlushCache()
	err = ivrtasks.RetryCalls(ctx, rt)
	assert.NoError(t, err)

	// this time should be failed
	assertdb.Query(t, db, `SELECT COUNT(*) FROM channels_channelconnection WHERE contact_id = $1 AND status = $2 AND external_id = $3`,
		testdata.Cathy.ID, models.ConnectionStatusFailed, "call1").Returns(1)
}

func TestRetryCallsInWorkerPool(t *testing.T) {
	ctx, rt, db, rp := testsuite.Get()
	rc := rp.Get()
	defer rc.Close()

	defer testsuite.Reset(testsuite.ResetAll)

	// register our mock client
	ivr.RegisterServiceType(models.ChannelType("ZZ"), NewMockProvider)

	// update our twilio channel to be of type 'ZZ' and set max_concurrent_events to 1
	db.MustExec(`UPDATE channels_channel SET channel_type = 'ZZ', config = '{"max_concurrent_events": 1}' WHERE id = $1`, testdata.TwilioChannel.ID)

	// create a flow start for cathy
	start := models.NewFlowStart(testdata.Org1.ID, models.StartTypeTrigger, models.FlowTypeVoice, testdata.IVRFlow.ID).
		WithContactIDs([]models.ContactID{testdata.Cathy.ID})

	// call our master starter
	err := starts.CreateFlowBatches(ctx, rt, start)
	assert.NoError(t, err)

	// should have one task in our ivr queue
	task, err := queue.PopNextTask(rc, queue.HandlerQueue)
	assert.NoError(t, err)
	batch := &models.FlowStartBatch{}
	err = json.Unmarshal(task.Task, batch)
	assert.NoError(t, err)

	client.callError = nil
	client.callID = ivr.CallID("call1")
	err = ivrtasks.HandleFlowStartBatch(ctx, rt, batch)
	assert.NoError(t, err)
	assertdb.Query(t, db, `SELECT COUNT(*) FROM channels_channelconnection WHERE contact_id = $1 AND status = $2 AND external_id = $3`,
		testdata.Cathy.ID, models.ConnectionStatusWired, "call1").Returns(1)

	// change our call to be errored instead of wired
	db.MustExec(`UPDATE channels_channelconnection SET status = 'E', next_attempt = NOW() WHERE external_id = 'call1';`)

	err = ivrtasks.RetryCallsInWorkerPool(ctx, rt)
	assert.NoError(t, err)

	// should now be in wired state
	assertdb.Query(t, db, `SELECT COUNT(*) FROM channels_channelconnection WHERE contact_id = $1 AND status = $2 AND external_id = $3`,
		testdata.Cathy.ID, models.ConnectionStatusWired, "call1").Returns(1)

	// back to retry and make the channel inactive
	db.MustExec(`UPDATE channels_channelconnection SET status = 'E', next_attempt = NOW() WHERE external_id = 'call1';`)
	db.MustExec(`UPDATE channels_channel SET is_active = FALSE WHERE id = $1`, testdata.TwilioChannel.ID)

	models.FlushCache()
	err = ivrtasks.RetryCallsInWorkerPool(ctx, rt)
	assert.NoError(t, err)

	// this time should be failed
	assertdb.Query(t, db, `SELECT COUNT(*) FROM channels_channelconnection WHERE contact_id = $1 AND status = $2 AND external_id = $3`,
		testdata.Cathy.ID, models.ConnectionStatusFailed, "call1").Returns(1)
}

func TestClearConnections(t *testing.T) {
	ctx, rt, db, rp := testsuite.Get()
	rc := rp.Get()
	defer rc.Close()

	defer testsuite.Reset(testsuite.ResetAll)

	ivr.RegisterServiceType(models.ChannelType("ZZ"), NewMockProvider)

	db.MustExec(`UPDATE channels_channel SET channel_type = 'ZZ', config = '{"max_concurrent_events": 1}' WHERE id = $1`, testdata.TwilioChannel.ID)

	start := models.NewFlowStart(testdata.Org1.ID, models.StartTypeTrigger, models.FlowTypeVoice, testdata.IVRFlow.ID).
		WithContactIDs([]models.ContactID{testdata.Cathy.ID})

	// call our master starter
	err := starts.CreateFlowBatches(ctx, rt, start)
	assert.NoError(t, err)

	task, err := queue.PopNextTask(rc, queue.HandlerQueue)
	assert.NoError(t, err)
	batch := &models.FlowStartBatch{}
	err = json.Unmarshal(task.Task, batch)
	assert.NoError(t, err)

	client.callError = nil
	client.callID = ivr.CallID("call1")
	err = ivrtasks.HandleFlowStartBatch(ctx, rt, batch)
	assert.NoError(t, err)
	assertdb.Query(t, db,
		`SELECT COUNT(*) FROM channels_channelconnection WHERE contact_id = $1 AND status = $2 AND external_id = $3`,
		testdata.Cathy.ID, models.ConnectionStatusWired, "call1",
	).Returns(1)

	// update channel connection to be modified_on 2 days ago
	db.MustExec(`UPDATE channels_channelconnection SET modified_on = NOW() - INTERVAL '2 DAY' WHERE contact_id = $1 AND status = $2 AND external_id = $3`,
		testdata.Cathy.ID, models.ConnectionStatusWired, "call1",
	)

	// cleaning
	err = ivrtasks.ClearStuckChannelConnections(ctx, rt)
	assert.NoError(t, err)

	// status should be Failed
	assertdb.Query(t, db,
		`SELECT COUNT(*) FROM channels_channelconnection WHERE contact_id = $1 AND status = $2 AND external_id = $3`,
		testdata.Cathy.ID, models.ConnectionStatusFailed, "call1",
	).Returns(1)
}

func TestUpdateMaxChannelsConnection(t *testing.T) {
	ctx, rt, db, rp := testsuite.Get()
	rc := rp.Get()
	defer rc.Close()

	defer testsuite.Reset(testsuite.ResetAll)

	// register our mock client
	ivr.RegisterServiceType(models.ChannelType("ZZ"), NewMockProvider)

	//set max_concurrent_events to 1
	db.MustExec(`UPDATE channels_channel SET channel_type = 'ZZ', config = '{"max_concurrent_events": 1}' WHERE id = $1`, testdata.TwilioChannel.ID)

	//set max_concurrent_events to 0
	err := ivrtasks.ChangeMaxConnectionsConfig(ctx, rt, "ZZ", 0)
	assert.NoError(t, err)
	var confStr string
	err = db.QueryRowx("SELECT config FROM channels_channel WHERE id = $1", testdata.TwilioChannel.ID).Scan(&confStr)
	assert.NoError(t, err)
	conf := make(map[string]interface{})
	err = json.Unmarshal([]byte(confStr), &conf)
	assert.NoError(t, err)
	assert.Equal(t, 0, int(conf["max_concurrent_events"].(float64)))

	// create a flow start for cathy
	start := models.NewFlowStart(testdata.Org1.ID, models.StartTypeTrigger, models.FlowTypeVoice, testdata.IVRFlow.ID).
		WithContactIDs([]models.ContactID{testdata.Cathy.ID})
	// call our master starter
	err = starts.CreateFlowBatches(ctx, rt, start)
	assert.NoError(t, err)

	// should have one task in our ivr queue
	task, err := queue.PopNextTask(rc, queue.HandlerQueue)
	assert.NoError(t, err)
	batch := &models.FlowStartBatch{}
	err = json.Unmarshal(task.Task, batch)
	assert.NoError(t, err)

	client.callError = nil
	client.callID = ivr.CallID("call1")
	err = ivrtasks.HandleFlowStartBatch(ctx, rt, batch)
	assert.NoError(t, err)
	assertdb.Query(t, db, `SELECT COUNT(*) FROM channels_channelconnection WHERE contact_id = $1 AND status = $2`,
		testdata.Cathy.ID, models.ConnectionStatusQueued).Returns(1)

	//set max_concurrent_events to 500
	err = ivrtasks.ChangeMaxConnectionsConfig(ctx, rt, "ZZ", 500)
	assert.NoError(t, err)
	err = db.QueryRowx("SELECT config FROM channels_channel WHERE id = $1", testdata.TwilioChannel.ID).Scan(&confStr)
	assert.NoError(t, err)
	conf2 := make(map[string]interface{})
	err = json.Unmarshal([]byte(confStr), &conf2)
	assert.NoError(t, err)
	assert.Equal(t, 500, int(conf2["max_concurrent_events"].(float64)))

	// change our call to next attempt to be now minus 1 minute
	db.MustExec(`UPDATE channels_channelconnection SET next_attempt = NOW() - INTERVAL '1 MINUTE' WHERE contact_id = $1;`, testdata.Cathy.ID)
	assert.NoError(t, err)

	db.MustExec("SELECT pg_sleep(5)")

	err = ivrtasks.RetryCalls(ctx, rt)
	assert.NoError(t, err)

	assertdb.Query(t, db, `SELECT COUNT(*) FROM channels_channelconnection WHERE contact_id = $1 AND status = $2`,
		testdata.Cathy.ID, models.ConnectionStatusWired).Returns(1)
}
