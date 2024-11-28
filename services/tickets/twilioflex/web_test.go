package twilioflex_test

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
		testdata.Twilioflex,
		testdata.DefaultTopic,
		"Have you seen my cookies?",
		"CH6442c09c93ba4d13966fa42e9b78f620",
		time.Time{},
		testdata.Viewer,
	)

	testsuite.RunWebTests(t, ctx, rt, "testdata/event_callback.json", map[string]string{"cathy_ticket_uuid": string(ticket.UUID)})
}
