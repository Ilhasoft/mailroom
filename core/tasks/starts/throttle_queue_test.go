package starts_test

import (
	"testing"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks/starts"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/utils/queues"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThrottleQueue(t *testing.T) {
	ctx, rt := testsuite.Runtime()
	rc := rt.RP.Get()
	defer rc.Close()

	defer testsuite.Reset(testsuite.ResetRedis | testsuite.ResetData)

	queue := queues.NewFairSorted("test")
	cron := &starts.ThrottleQueueCron{Queue: queue}
	res, err := cron.Run(ctx, rt)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"paused": 0, "resumed": 0}, res)

	queue.Push(rc, "type1", 1, "task1", queues.DefaultPriority)

	res, err = cron.Run(ctx, rt)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"paused": 0, "resumed": 1}, res)

	// make it look like org 1 has 20,000 messages in its outbox
	rt.DB.MustExec(`INSERT INTO msgs_systemlabelcount(org_id, label_type, count, is_squashed) VALUES (1, 'O', 10050, FALSE)`)

	models.FlushCache()

	res, err = cron.Run(ctx, rt)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"paused": 1, "resumed": 0}, res)

	// make it look like most of the inbox has cleared
	rt.DB.MustExec(`INSERT INTO msgs_systemlabelcount(org_id, label_type, count, is_squashed) VALUES (1, 'O', -10000, FALSE)`)

	models.FlushCache()

	res, err = cron.Run(ctx, rt)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"paused": 0, "resumed": 1}, res)
}
