package handlers_test

import (
	"testing"
	"time"

	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/actions"
	"github.com/nyaruka/mailroom/core/handlers"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
)

func TestMsgReceived(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.Reset(testsuite.ResetAll)

	now := time.Now()

	tcs := []handlers.TestCase{
		{
			Actions: handlers.ContactActionMap{
				testdata.Cathy: []flows.Action{
					actions.NewSendMsg(handlers.NewActionUUID(), "Hello World", nil, nil, false),
				},
				testdata.George: []flows.Action{
					actions.NewSendMsg(handlers.NewActionUUID(), "Hello world", nil, nil, false),
				},
			},
			Msgs: handlers.ContactMsgMap{
				testdata.Cathy: testdata.InsertIncomingMsg(rt, testdata.Org1, testdata.TwilioChannel, testdata.Cathy, "start", models.MsgStatusHandled).FlowMsg,
			},
			SQLAssertions: []handlers.SQLAssertion{
				{
					SQL:   "SELECT COUNT(*) FROM contacts_contact WHERE id = $1 AND last_seen_on > $2",
					Args:  []any{testdata.Cathy.ID, now},
					Count: 1,
				},
				{
					SQL:   "SELECT COUNT(*) FROM contacts_contact WHERE id = $1 AND last_seen_on IS NULL",
					Args:  []any{testdata.George.ID},
					Count: 1,
				},
			},
		},
	}

	handlers.RunTestCases(t, ctx, rt, tcs)
}
