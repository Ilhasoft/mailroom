package models_test

import (
	"testing"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"

	"github.com/stretchr/testify/assert"
)

func TestChannelConnections(t *testing.T) {
	ctx := testsuite.CTX()
	db := testsuite.DB()

	conn, err := models.InsertIVRConnection(ctx, db, testdata.Org1.ID, models.TwilioChannelID, models.NilStartID, models.CathyID, models.CathyURNID, models.ConnectionDirectionOut, models.ConnectionStatusPending, "")
	assert.NoError(t, err)

	assert.NotEqual(t, models.ConnectionID(0), conn.ID())

	err = conn.UpdateExternalID(ctx, db, "test1")
	assert.NoError(t, err)

	testsuite.AssertQueryCount(t, db, `SELECT count(*) from channels_channelconnection where external_id = 'test1' AND id = $1`, []interface{}{conn.ID()}, 1)

	conn2, err := models.SelectChannelConnection(ctx, db, conn.ID())
	assert.NoError(t, err)
	assert.Equal(t, "test1", conn2.ExternalID())
}
