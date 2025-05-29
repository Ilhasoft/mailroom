package contacts_test

import (
	"testing"

	"github.com/nyaruka/gocommon/dbutil/assertdb"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks/contacts"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/stretchr/testify/assert"
)

func TestBulkSessionExpireTask(t *testing.T) {
	_, rt := testsuite.Runtime()

	defer testsuite.Reset(testsuite.ResetData)

	twilioCallID := testdata.InsertCall(rt, testdata.Org1, testdata.TwilioChannel, testdata.Alexandra)

	session1UUID := testdata.InsertWaitingSession(rt, testdata.Org1, testdata.Alexandra, models.FlowTypeVoice, testdata.Favorites, twilioCallID)
	session2UUID := testdata.InsertWaitingSession(rt, testdata.Org1, testdata.Bob, models.FlowTypeMessaging, testdata.PickANumber, models.NilCallID)
	session3UUID := testdata.InsertWaitingSession(rt, testdata.Org1, testdata.Cathy, models.FlowTypeMessaging, testdata.Favorites, models.NilCallID)

	testsuite.QueueBatchTask(t, rt, testdata.Org1, &contacts.BulkSessionExpireTask{
		SessionUUIDs: []flows.SessionUUID{session1UUID, session2UUID},
	})

	assert.Equal(t, map[string]int{"bulk_session_expire": 1}, testsuite.FlushTasks(t, rt, "batch", "throttled"))

	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowsession WHERE uuid = $1`, session1UUID).Returns("X")
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowrun WHERE session_uuid = $1`, session1UUID).Returns("X")
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowsession WHERE uuid = $1`, session2UUID).Returns("X")
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowrun WHERE session_uuid = $1`, session2UUID).Returns("X")
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowsession WHERE uuid = $1`, session3UUID).Returns("W")
	assertdb.Query(t, rt.DB, `SELECT status FROM flows_flowrun WHERE session_uuid = $1`, session3UUID).Returns("W")
}
