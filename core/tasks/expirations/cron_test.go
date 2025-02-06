package expirations_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/nyaruka/gocommon/dbutil/assertdb"
	"github.com/nyaruka/gocommon/i18n"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/goflow/flows"
	_ "github.com/nyaruka/mailroom/core/handlers"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/core/tasks/expirations"
	"github.com/nyaruka/mailroom/core/tasks/handler"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/stretchr/testify/assert"
)

func TestExpirations(t *testing.T) {
	ctx, rt := testsuite.Runtime()
	rc := rt.RP.Get()
	defer rc.Close()

	defer testsuite.Reset(testsuite.ResetData | testsuite.ResetRedis)

	// create a blocked contact
	blake := testdata.InsertContact(rt, testdata.Org1, "9eef59ef-21b3-4f51-a296-937529a30e38", "Blake", i18n.NilLanguage, models.ContactStatusBlocked)

	// create single run session for Cathy, no parent to resume
	s1ID := testdata.InsertWaitingSession(rt, testdata.Org1, testdata.Cathy, models.FlowTypeMessaging, testdata.Favorites, models.NilCallID, time.Now(), time.Now(), false, nil)
	r1ID := testdata.InsertFlowRun(rt, testdata.Org1, s1ID, testdata.Cathy, testdata.Favorites, models.RunStatusWaiting, "")

	// create parent/child session for George, can resume
	s2ID := testdata.InsertWaitingSession(rt, testdata.Org1, testdata.George, models.FlowTypeMessaging, testdata.Favorites, models.NilCallID, time.Now(), time.Now(), true, nil)
	r2ID := testdata.InsertFlowRun(rt, testdata.Org1, s2ID, testdata.George, testdata.Favorites, models.RunStatusActive, "")
	r3ID := testdata.InsertFlowRun(rt, testdata.Org1, s2ID, testdata.George, testdata.Favorites, models.RunStatusWaiting, "")

	// create session for Bob with expiration in future
	s3ID := testdata.InsertWaitingSession(rt, testdata.Org1, testdata.Bob, models.FlowTypeMessaging, testdata.Favorites, models.NilCallID, time.Now(), time.Now().Add(time.Hour), true, nil)
	r4ID := testdata.InsertFlowRun(rt, testdata.Org1, s3ID, testdata.Bob, testdata.Favorites, models.RunStatusWaiting, "")

	// create an IVR session for Alexandria
	call := testdata.InsertCall(rt, testdata.Org1, testdata.TwilioChannel, testdata.Alexandria)
	s4ID := testdata.InsertWaitingSession(rt, testdata.Org1, testdata.Alexandria, models.FlowTypeVoice, testdata.IVRFlow, call, time.Now(), time.Now(), false, nil)
	r5ID := testdata.InsertFlowRun(rt, testdata.Org1, s4ID, testdata.Alexandria, testdata.IVRFlow, models.RunStatusWaiting, "")

	// create a parent/child session for the blocked contact
	s5ID := testdata.InsertWaitingSession(rt, testdata.Org1, blake, models.FlowTypeMessaging, testdata.Favorites, models.NilCallID, time.Now(), time.Now(), true, nil)
	r6ID := testdata.InsertFlowRun(rt, testdata.Org1, s5ID, blake, testdata.Favorites, models.RunStatusActive, "")
	r7ID := testdata.InsertFlowRun(rt, testdata.Org1, s5ID, blake, testdata.Favorites, models.RunStatusWaiting, "")

	// for other org create 6 waiting sessions that will expire
	for i := range 6 {
		c := testdata.InsertContact(rt, testdata.Org2, flows.ContactUUID(uuids.NewV4()), fmt.Sprint(i), i18n.NilLanguage, models.ContactStatusActive)
		testdata.InsertWaitingSession(rt, testdata.Org2, c, models.FlowTypeMessaging, testdata.Favorites, models.NilCallID, time.Now(), time.Now(), true, nil)
	}

	// expire our sessions...
	cron := expirations.NewExpirationsCron(3, 5)
	res, err := cron.Run(ctx, rt)
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"exited": 1, "dupes": 0, "queued_bulk": 6, "queued_handler": 2}, res)

	// Cathy's session should be expired along with its runs
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowsession WHERE id = $1;`, s1ID).Columns(map[string]any{"status": "X"})
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowrun WHERE id = $1;`, r1ID).Columns(map[string]any{"status": "X"})

	// Bob's session and runs should be unchanged because it's been queued for resumption
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowsession WHERE id = $1;`, s2ID).Columns(map[string]any{"status": "W"})
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowrun WHERE id = $1;`, r2ID).Columns(map[string]any{"status": "A"})
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowrun WHERE id = $1;`, r3ID).Columns(map[string]any{"status": "W"})

	// George's session and run should be unchanged
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowsession WHERE id = $1;`, s3ID).Columns(map[string]any{"status": "W"})
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowrun WHERE id = $1;`, r4ID).Columns(map[string]any{"status": "W"})

	// Alexandria's session and run should be unchanged because IVR expirations are handled separately
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowsession WHERE id = $1;`, s4ID).Columns(map[string]any{"status": "W"})
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowrun WHERE id = $1;`, r5ID).Columns(map[string]any{"status": "W"})

	// blocked contact's session and runs sshould be unchanged because it's been queued for resumption.. like any other contact
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowsession WHERE id = $1;`, s5ID).Columns(map[string]any{"status": "W"})
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowrun WHERE id = $1;`, r6ID).Columns(map[string]any{"status": "A"})
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowrun WHERE id = $1;`, r7ID).Columns(map[string]any{"status": "W"})

	// should have created two handler tasks for org 1
	task1, err := tasks.HandlerQueue.Pop(rc)
	assert.NoError(t, err)
	assert.Equal(t, int(testdata.Org1.ID), task1.OwnerID)
	assert.Equal(t, "handle_contact_event", task1.Type)
	task2, err := tasks.HandlerQueue.Pop(rc)
	assert.NoError(t, err)
	assert.Equal(t, int(testdata.Org1.ID), task2.OwnerID)
	assert.Equal(t, "handle_contact_event", task2.Type)

	// decode the tasks to check contacts
	eventTask := &handler.HandleContactEventTask{}
	jsonx.MustUnmarshal(task1.Task, eventTask)
	assert.Equal(t, testdata.George.ID, eventTask.ContactID)
	eventTask = &handler.HandleContactEventTask{}
	jsonx.MustUnmarshal(task2.Task, eventTask)
	assert.Equal(t, blake.ID, eventTask.ContactID)

	// no other
	task, err := tasks.HandlerQueue.Pop(rc)
	assert.NoError(t, err)
	assert.Nil(t, task)

	// should have created two throttled bulk tasks for org 2
	task3, err := tasks.ThrottledQueue.Pop(rc)
	assert.NoError(t, err)
	assert.Equal(t, int(testdata.Org2.ID), task3.OwnerID)
	assert.Equal(t, "bulk_expire", task3.Type)
	task4, err := tasks.ThrottledQueue.Pop(rc)
	assert.NoError(t, err)
	assert.Equal(t, int(testdata.Org2.ID), task4.OwnerID)
	assert.Equal(t, "bulk_expire", task4.Type)

	// if task runs again, these tasks won't be re-queued
	res, err = cron.Run(ctx, rt)
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"exited": 0, "dupes": 8, "queued_handler": 0, "queued_bulk": 0}, res)
}

func TestExpireVoiceSessions(t *testing.T) {
	ctx, rt := testsuite.Runtime()
	rc := rt.RP.Get()
	defer rc.Close()

	defer testsuite.Reset(testsuite.ResetData | testsuite.ResetRedis)

	// create voice session for Cathy
	conn1ID := testdata.InsertCall(rt, testdata.Org1, testdata.TwilioChannel, testdata.Cathy)
	s1ID := testdata.InsertWaitingSession(rt, testdata.Org1, testdata.Cathy, models.FlowTypeVoice, testdata.IVRFlow, conn1ID, time.Now(), time.Now(), false, nil)
	r1ID := testdata.InsertFlowRun(rt, testdata.Org1, s1ID, testdata.Cathy, testdata.Favorites, models.RunStatusWaiting, "")

	// create voice session for Bob with expiration in future
	conn2ID := testdata.InsertCall(rt, testdata.Org1, testdata.TwilioChannel, testdata.Bob)
	s2ID := testdata.InsertWaitingSession(rt, testdata.Org1, testdata.Bob, models.FlowTypeMessaging, testdata.IVRFlow, conn2ID, time.Now(), time.Now().Add(time.Hour), false, nil)
	r2ID := testdata.InsertFlowRun(rt, testdata.Org1, s2ID, testdata.Bob, testdata.IVRFlow, models.RunStatusWaiting, "")

	// create a messaging session for Alexandria
	s3ID := testdata.InsertWaitingSession(rt, testdata.Org1, testdata.Alexandria, models.FlowTypeMessaging, testdata.Favorites, models.NilCallID, time.Now(), time.Now(), false, nil)
	r3ID := testdata.InsertFlowRun(rt, testdata.Org1, s3ID, testdata.Alexandria, testdata.Favorites, models.RunStatusWaiting, "")

	time.Sleep(5 * time.Millisecond)

	// expire our sessions...
	cron := &expirations.VoiceExpirationsCron{}
	res, err := cron.Run(ctx, rt)
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"expired": 1}, res)

	// Cathy's session should be expired along with its runs
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowsession WHERE id = $1;`, s1ID).Columns(map[string]any{"status": "X"})
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowrun WHERE id = $1;`, r1ID).Columns(map[string]any{"status": "X"})

	// Bob's session and run should be unchanged
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowsession WHERE id = $1;`, s2ID).Columns(map[string]any{"status": "W"})
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowrun WHERE id = $1;`, r2ID).Columns(map[string]any{"status": "W"})

	// Alexandria's session and run should be unchanged because message expirations are handled separately
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowsession WHERE id = $1;`, s3ID).Columns(map[string]any{"status": "W"})
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowrun WHERE id = $1;`, r3ID).Columns(map[string]any{"status": "W"})
}
