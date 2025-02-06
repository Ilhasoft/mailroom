package timeouts_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/nyaruka/gocommon/i18n"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/goflow/flows"
	_ "github.com/nyaruka/mailroom/core/handlers"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/core/tasks/handler"
	"github.com/nyaruka/mailroom/core/tasks/timeouts"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"

	"github.com/stretchr/testify/assert"
)

func TestTimeouts(t *testing.T) {
	ctx, rt := testsuite.Runtime()
	rc := rt.RP.Get()
	defer rc.Close()

	defer testsuite.Reset(testsuite.ResetData | testsuite.ResetRedis)

	// create sessions for Bob and Cathy that have timed out and session for George that has not
	s1TimeoutOn := time.Now().Add(-time.Second)
	testdata.InsertWaitingSession(rt, testdata.Org1, testdata.Cathy, models.FlowTypeMessaging, testdata.Favorites, models.NilCallID, time.Now(), time.Now(), false, &s1TimeoutOn)
	s2TimeoutOn := time.Now().Add(-time.Second * 30)
	testdata.InsertWaitingSession(rt, testdata.Org1, testdata.Bob, models.FlowTypeMessaging, testdata.Favorites, models.NilCallID, time.Now(), time.Now(), false, &s2TimeoutOn)
	s3TimeoutOn := time.Now().Add(time.Hour * 24)
	testdata.InsertWaitingSession(rt, testdata.Org1, testdata.George, models.FlowTypeMessaging, testdata.Favorites, models.NilCallID, time.Now(), time.Now(), false, &s3TimeoutOn)

	// for other org create 6 waiting sessions
	for i := range 6 {
		c := testdata.InsertContact(rt, testdata.Org2, flows.ContactUUID(uuids.NewV4()), fmt.Sprint(i), i18n.NilLanguage, models.ContactStatusActive)
		timeoutOn := time.Now().Add(-time.Second * 10)
		testdata.InsertWaitingSession(rt, testdata.Org2, c, models.FlowTypeMessaging, testdata.Favorites, models.NilCallID, time.Now(), time.Now(), false, &timeoutOn)
	}

	// schedule our timeouts
	cron := timeouts.NewTimeoutsCron(3, 5)
	res, err := cron.Run(ctx, rt)
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"dupes": 0, "queued_handler": 2, "queued_bulk": 6}, res)

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
	assert.Equal(t, testdata.Bob.ID, eventTask.ContactID)
	eventTask = &handler.HandleContactEventTask{}
	jsonx.MustUnmarshal(task2.Task, eventTask)
	assert.Equal(t, testdata.Cathy.ID, eventTask.ContactID)

	// no other
	task, err := tasks.HandlerQueue.Pop(rc)
	assert.NoError(t, err)
	assert.Nil(t, task)

	// should have created two throttled bulk tasks for org 2
	task3, err := tasks.ThrottledQueue.Pop(rc)
	assert.NoError(t, err)
	assert.Equal(t, int(testdata.Org2.ID), task3.OwnerID)
	assert.Equal(t, "bulk_timeout", task3.Type)
	task4, err := tasks.ThrottledQueue.Pop(rc)
	assert.NoError(t, err)
	assert.Equal(t, int(testdata.Org2.ID), task4.OwnerID)
	assert.Equal(t, "bulk_timeout", task4.Type)

	// if task runs again, these timeouts won't be re-queued
	res, err = cron.Run(ctx, rt)
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"dupes": 8, "queued_handler": 0, "queued_bulk": 0}, res)
}
