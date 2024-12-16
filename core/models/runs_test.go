package models_test

import (
	"testing"

	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/stretchr/testify/assert"
)

func TestGetContactIDsAtNode(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.Reset(testsuite.ResetData)

	createRun := func(org *testdata.Org, contact *testdata.Contact, nodeUUID flows.NodeUUID) {
		sessionID := testdata.InsertFlowSession(rt, org, contact, models.FlowTypeMessaging, models.SessionStatusWaiting, testdata.Favorites, models.NilCallID)
		testdata.InsertFlowRun(rt, org, sessionID, contact, testdata.Favorites, models.RunStatusWaiting, nodeUUID)
	}

	createRun(testdata.Org1, testdata.Alexandria, "2fe26b10-2bb1-4115-9401-33a8a0d5d52a")
	createRun(testdata.Org1, testdata.Bob, "dd79811e-a88a-4e67-bb47-a132fe8ce3f2")
	createRun(testdata.Org1, testdata.George, "dd79811e-a88a-4e67-bb47-a132fe8ce3f2")
	createRun(testdata.Org2, testdata.Org2Contact, "dd79811e-a88a-4e67-bb47-a132fe8ce3f2") // shouldn't be possible but..

	contactIDs, err := models.GetContactIDsAtNode(ctx, rt, testdata.Org1.ID, "dd79811e-a88a-4e67-bb47-a132fe8ce3f2")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []models.ContactID{testdata.Bob.ID, testdata.George.ID}, contactIDs)
}
