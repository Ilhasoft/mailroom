package ctasks_test

import (
	"testing"

	"github.com/nyaruka/gocommon/dbutil/assertdb"
	_ "github.com/nyaruka/mailroom/core/handlers"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/core/tasks/handler"
	"github.com/nyaruka/mailroom/core/tasks/handler/ctasks"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/stretchr/testify/require"
)

func TestTicketClosed(t *testing.T) {
	ctx, rt := testsuite.Runtime()
	rc := rt.RP.Get()
	defer rc.Close()

	defer testsuite.Reset(testsuite.ResetAll)

	// add a ticket closed trigger
	testdata.InsertTicketClosedTrigger(rt, testdata.Org1, testdata.Favorites)

	ticket := testdata.InsertClosedTicket(rt, testdata.Org1, testdata.Cathy, testdata.DefaultTopic, nil)
	modelTicket := ticket.Load(rt)

	models.NewTicketClosedEvent(modelTicket, testdata.Admin.ID)

	err := handler.QueueTask(rc, testdata.Org1.ID, testdata.Cathy.ID, ctasks.NewTicketClosed(modelTicket.ID()))
	require.NoError(t, err)

	task, err := tasks.HandlerQueue.Pop(rc)
	require.NoError(t, err)

	err = tasks.Perform(ctx, rt, task)
	require.NoError(t, err)

	assertdb.Query(t, rt.DB, `SELECT count(*) FROM msgs_msg WHERE contact_id = $1 AND direction = 'O' AND text = 'What is your favorite color?'`, testdata.Cathy.ID).Returns(1)
}
