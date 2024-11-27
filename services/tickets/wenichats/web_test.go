package wenichats_test

import (
	"testing"
	"time"

	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
)

func TestEventCallback(t *testing.T) {
	ctx, rt := testsuite.Runtime()
	testsuite.Reset(testsuite.ResetData | testsuite.ResetStorage)

	defer testsuite.Reset(testsuite.ResetData | testsuite.ResetStorage)

	ticket := testdata.InsertOpenTicket(
		rt,
		testdata.Org1,
		testdata.Cathy,
		testdata.Wenichats,
		testdata.DefaultTopic,
		"Have you seen my cookies?",
		"e0fa6b4b-92c2-4906-98dc-e1a9f6b141d2",
		time.Now(),
		nil,
	)

	testsuite.RunWebTests(t, ctx, rt, "testdata/event_callback.json", map[string]string{"cathy_ticket_uuid": string(ticket.UUID)})
}
