package expirations_test

import (
	"testing"
	"time"

	"github.com/nyaruka/gocommon/dates"
	"github.com/nyaruka/mailroom/core/tasks/expirations"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/stretchr/testify/assert"
)

func TestBulkExpire(t *testing.T) {
	_, rt := testsuite.Runtime()
	defer testsuite.Reset(testsuite.ResetRedis)

	defer dates.SetNowFunc(time.Now)
	dates.SetNowFunc(dates.NewFixedNow(time.Date(2024, 11, 15, 13, 59, 0, 0, time.UTC)))

	testsuite.QueueBatchTask(t, rt, testdata.Org1, &expirations.BulkExpireTask{
		Expirations: []*expirations.ExpiredWait{
			{SessionID: 123456, ContactID: testdata.Cathy.ID, WaitExpiresOn: time.Date(2024, 11, 15, 13, 57, 0, 0, time.UTC)},
			{SessionID: 234567, ContactID: testdata.Bob.ID, WaitExpiresOn: time.Date(2024, 11, 15, 13, 58, 0, 0, time.UTC)},
		},
	})

	assert.Equal(t, map[string]int{"bulk_expire": 1}, testsuite.FlushTasks(t, rt, "batch", "throttled"))

	testsuite.AssertContactTasks(t, testdata.Org1, testdata.Cathy, []string{
		`{"type":"expiration_event","task":{"session_id":123456,"time":"2024-11-15T13:57:00Z"},"queued_on":"2024-11-15T13:59:00Z"}`,
	})
	testsuite.AssertContactTasks(t, testdata.Org1, testdata.Bob, []string{
		`{"type":"expiration_event","task":{"session_id":234567,"time":"2024-11-15T13:58:00Z"},"queued_on":"2024-11-15T13:59:00Z"}`,
	})
}
