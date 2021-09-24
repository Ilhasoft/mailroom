package ticket

import (
	"testing"

	_ "github.com/nyaruka/mailroom/services/tickets/mailgun"
	_ "github.com/nyaruka/mailroom/services/tickets/zendesk"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/nyaruka/mailroom/web"
)

func TestTicketAssign(t *testing.T) {
	_, _, db, _ := testsuite.Get()

	defer testsuite.ResetData(db)

	testdata.InsertOpenTicket(db, testdata.Org1, testdata.Cathy, testdata.Internal, testdata.DefaultTopic, "Have you seen my cookies?", "17", testdata.Admin)
	testdata.InsertOpenTicket(db, testdata.Org1, testdata.Cathy, testdata.Internal, testdata.DefaultTopic, "Have you seen my cookies?", "21", testdata.Agent)
	testdata.InsertClosedTicket(db, testdata.Org1, testdata.Cathy, testdata.Internal, testdata.DefaultTopic, "Have you seen my cookies?", "34", nil)
	testdata.InsertClosedTicket(db, testdata.Org1, testdata.Bob, testdata.Internal, testdata.DefaultTopic, "", "", nil)

	web.RunWebTests(t, "testdata/assign.json", nil)
}

func TestTicketAddNote(t *testing.T) {
	_, _, db, _ := testsuite.Get()

	defer testsuite.ResetData(db)

	testdata.InsertOpenTicket(db, testdata.Org1, testdata.Cathy, testdata.Internal, testdata.DefaultTopic, "Have you seen my cookies?", "17", testdata.Admin)
	testdata.InsertOpenTicket(db, testdata.Org1, testdata.Cathy, testdata.Internal, testdata.DefaultTopic, "Have you seen my cookies?", "21", testdata.Agent)
	testdata.InsertClosedTicket(db, testdata.Org1, testdata.Cathy, testdata.Internal, testdata.DefaultTopic, "Have you seen my cookies?", "34", nil)

	web.RunWebTests(t, "testdata/add_note.json", nil)
}

func TestTicketChangeTopic(t *testing.T) {
	_, _, db, _ := testsuite.Get()

	defer testsuite.ResetData(db)

	testdata.InsertOpenTicket(db, testdata.Org1, testdata.Cathy, testdata.Internal, testdata.DefaultTopic, "Have you seen my cookies?", "17", testdata.Admin)
	testdata.InsertOpenTicket(db, testdata.Org1, testdata.Cathy, testdata.Internal, testdata.SupportTopic, "Have you seen my cookies?", "21", testdata.Agent)
	testdata.InsertClosedTicket(db, testdata.Org1, testdata.Cathy, testdata.Internal, testdata.SalesTopic, "Have you seen my cookies?", "34", nil)

	web.RunWebTests(t, "testdata/change_topic.json", nil)
}

func TestTicketClose(t *testing.T) {
	_, _, db, _ := testsuite.Get()

	defer testsuite.ResetData(db)

	// create 2 open tickets and 1 closed one for Cathy across two different ticketers
	testdata.InsertOpenTicket(db, testdata.Org1, testdata.Cathy, testdata.Mailgun, testdata.DefaultTopic, "Have you seen my cookies?", "17", testdata.Admin)
	testdata.InsertOpenTicket(db, testdata.Org1, testdata.Cathy, testdata.Zendesk, testdata.DefaultTopic, "Have you seen my cookies?", "21", nil)
	testdata.InsertClosedTicket(db, testdata.Org1, testdata.Cathy, testdata.Zendesk, testdata.DefaultTopic, "Have you seen my cookies?", "34", testdata.Editor)
	testdata.InsertOpenTicket(db, testdata.Org1, testdata.Cathy, testdata.Zendesk, testdata.DefaultTopic, "Have you seen my cookies?", "21", nil)

	web.RunWebTests(t, "testdata/close.json", nil)
}

func TestTicketReopen(t *testing.T) {
	_, _, db, _ := testsuite.Get()

	defer testsuite.ResetData(db)

	// create 2 closed tickets and 1 open one for Cathy
	testdata.InsertClosedTicket(db, testdata.Org1, testdata.Cathy, testdata.Mailgun, testdata.DefaultTopic, "Have you seen my cookies?", "17", testdata.Admin)
	testdata.InsertClosedTicket(db, testdata.Org1, testdata.Cathy, testdata.Zendesk, testdata.DefaultTopic, "Have you seen my cookies?", "21", nil)
	testdata.InsertOpenTicket(db, testdata.Org1, testdata.Cathy, testdata.Zendesk, testdata.DefaultTopic, "Have you seen my cookies?", "34", testdata.Editor)

	web.RunWebTests(t, "testdata/reopen.json", nil)
}
