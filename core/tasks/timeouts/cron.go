package timeouts

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/core/tasks/handler"
	"github.com/nyaruka/mailroom/core/tasks/handler/ctasks"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/redisx"
)

func init() {
	tasks.RegisterCron("sessions_timeouts", NewTimeoutsCron(10, 100))
}

type timeoutsCron struct {
	marker        *redisx.IntervalSet
	bulkThreshold int // use bulk task for any org with this or more timeouts
	bulkBatchSize int // number of timeouts to queue in a single bulk task
}

func NewTimeoutsCron(bulkThreshold, bulkBatchSize int) tasks.Cron {
	return &timeoutsCron{
		marker:        redisx.NewIntervalSet("session_timeouts", time.Hour*24, 2),
		bulkThreshold: bulkThreshold,
		bulkBatchSize: bulkBatchSize,
	}
}

func (c *timeoutsCron) Next(last time.Time) time.Time {
	return tasks.CronNext(last, time.Minute)
}

func (c *timeoutsCron) AllInstances() bool {
	return false
}

func (c *timeoutsCron) Run(ctx context.Context, rt *runtime.Runtime) (map[string]any, error) {
	rows, err := rt.DB.QueryxContext(ctx, sqlSelectTimedoutSessions)
	if err != nil {
		return nil, fmt.Errorf("error querying sessions with timed out waits: %w", err)
	}
	defer rows.Close()

	taskID := func(t *Timeout) string { return fmt.Sprintf("%d:%s", t.SessionID, t.TimeoutOn.Format(time.RFC3339)) }

	// scan and organize by org
	byOrg := make(map[models.OrgID][]*Timeout, 50)

	rc := rt.RP.Get()
	defer rc.Close()

	numDupes, numQueuedHandler, numQueuedBulk := 0, 0, 0

	for rows.Next() {
		timeout := &Timeout{}
		if err := rows.StructScan(timeout); err != nil {
			return nil, fmt.Errorf("error scanning timeout: %w", err)
		}

		// check whether we've already queued this
		queued, err := c.marker.IsMember(rc, taskID(timeout))
		if err != nil {
			return nil, fmt.Errorf("error checking whether timeout is already queued: %w", err)
		}

		// already queued? move on
		if queued {
			numDupes++
			continue
		}

		byOrg[timeout.OrgID] = append(byOrg[timeout.OrgID], timeout)
	}

	for orgID, timeouts := range byOrg {
		throttle := len(timeouts) >= c.bulkThreshold

		for batch := range slices.Chunk(timeouts, c.bulkBatchSize) {
			if throttle {
				if err := tasks.Queue(rc, tasks.ThrottledQueue, orgID, &BulkTimeoutTask{Timeouts: batch}, true); err != nil {
					return nil, fmt.Errorf("error queuing bulk timeout task to throttle queue: %w", err)
				}
				numQueuedBulk += len(batch)
			}

			for _, timeout := range batch {
				if !throttle {
					err := handler.QueueTask(rc, orgID, timeout.ContactID, ctasks.NewWaitTimeout(timeout.SessionID, timeout.TimeoutOn))
					if err != nil {
						return nil, fmt.Errorf("error queuing timeout task to handler queue: %w", err)
					}
					numQueuedHandler++
				}

				// mark as queued
				if err = c.marker.Add(rc, taskID(timeout)); err != nil {
					return nil, fmt.Errorf("error marking timeout task as queued: %w", err)
				}
			}
		}
	}

	return map[string]any{"dupes": numDupes, "queued_handler": numQueuedHandler, "queued_bulk": numQueuedBulk}, nil
}

const sqlSelectTimedoutSessions = `
  SELECT id as session_id, org_id, contact_id, timeout_on
    FROM flows_flowsession
   WHERE status = 'W' AND timeout_on < NOW() AND call_id IS NULL
ORDER BY timeout_on ASC
   LIMIT 25000`

type Timeout struct {
	SessionID models.SessionID `db:"session_id" json:"session_id"`
	OrgID     models.OrgID     `db:"org_id"     json:"-"`
	ContactID models.ContactID `db:"contact_id" json:"contact_id"`
	TimeoutOn time.Time        `db:"timeout_on" json:"timeout_on"`
}
