package models_test

import (
	"testing"
	"time"

	"github.com/nyaruka/gocommon/dbutil/assertdb"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestAirtimeTransfers(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer rt.DB.MustExec(`DELETE FROM airtime_airtimetransfer`)

	// insert a transfer
	transfer := models.NewAirtimeTransfer(
		flows.AirtimeTransferUUID("3fd9af31-0fe9-4b63-bb8f-00805355c905"),
		testdata.Org1.ID,
		models.AirtimeTransferStatusSuccess,
		"2237512891",
		testdata.Cathy.ID,
		urns.URN("tel:+250700000001"),
		urns.URN("tel:+250700000002"),
		"RWF",
		decimal.RequireFromString(`100`),
		time.Now(),
	)
	err := models.InsertAirtimeTransfers(ctx, rt.DB, []*models.AirtimeTransfer{transfer})
	assert.Nil(t, err)

	assertdb.Query(t, rt.DB, `SELECT org_id, status, external_id from airtime_airtimetransfer`).Columns(map[string]any{"org_id": int64(1), "status": "S", "external_id": "2237512891"})

	// insert a failed transfer with nil sender, empty currency
	transfer = models.NewAirtimeTransfer(
		flows.AirtimeTransferUUID("e59dbe78-a159-4027-aae9-3232b00c77d5"),
		testdata.Org1.ID,
		models.AirtimeTransferStatusFailed,
		"2237512891",
		testdata.Cathy.ID,
		urns.NilURN,
		urns.URN("tel:+250700000002"),
		"",
		decimal.Zero,
		time.Now(),
	)
	err = models.InsertAirtimeTransfers(ctx, rt.DB, []*models.AirtimeTransfer{transfer})
	assert.Nil(t, err)

	assertdb.Query(t, rt.DB, `SELECT count(*) from airtime_airtimetransfer WHERE org_id = $1 AND status = $2`, testdata.Org1.ID, models.AirtimeTransferStatusFailed).Returns(1)
}
