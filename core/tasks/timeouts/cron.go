package timeouts

import (
	"context"
	"fmt"
	"time"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/core/tasks/handler"
	"github.com/nyaruka/mailroom/core/tasks/handler/ctasks"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/redisx"
)

func init() {
	tasks.RegisterCron("sessions_timeouts", newTimeoutsCron())
}

type timeoutsCron struct {
	marker *redisx.IntervalSet
}

func newTimeoutsCron() tasks.Cron {
	return &timeoutsCron{
		marker: redisx.NewIntervalSet("session_timeouts", time.Hour*24, 2),
	}
}

func (c *timeoutsCron) Next(last time.Time) time.Time {
	return tasks.CronNext(last, time.Minute)
}

func (c *timeoutsCron) AllInstances() bool {
	return false
}

// timeoutRuns looks for any runs that have timed out and schedules for them to continue
// TODO: extend lock
func (c *timeoutsCron) Run(ctx context.Context, rt *runtime.Runtime) (map[string]any, error) {
	// find all sessions that need to be expired (we exclude IVR runs)
	rows, err := rt.DB.QueryxContext(ctx, timedoutSessionsSQL)
	if err != nil {
		return nil, fmt.Errorf("error selecting timed out sessions: %w", err)
	}
	defer rows.Close()

	rc := rt.RP.Get()
	defer rc.Close()

	numQueued, numDupes := 0, 0

	// add a timeout task for each run
	timeout := &Timeout{}
	for rows.Next() {
		err := rows.StructScan(timeout)
		if err != nil {
			return nil, fmt.Errorf("error scanning timeout: %w", err)
		}

		// check whether we've already queued this
		taskID := fmt.Sprintf("%d:%s", timeout.SessionID, timeout.TimeoutOn.Format(time.RFC3339))
		queued, err := c.marker.IsMember(rc, taskID)
		if err != nil {
			return nil, fmt.Errorf("error checking whether task is queued: %w", err)
		}

		// already queued? move on
		if queued {
			numDupes++
			continue
		}

		// ok, queue this task
		err = handler.QueueTask(rc, timeout.OrgID, timeout.ContactID, ctasks.NewWaitTimeout(timeout.SessionID, timeout.TimeoutOn))
		if err != nil {
			return nil, fmt.Errorf("error adding new handle task: %w", err)
		}

		// and mark it as queued
		err = c.marker.Add(rc, taskID)
		if err != nil {
			return nil, fmt.Errorf("error marking timeout task as queued: %w", err)
		}

		numQueued++
	}

	return map[string]any{"dupes": numDupes, "queued": numQueued}, nil
}

const timedoutSessionsSQL = `
  SELECT id as session_id, org_id, contact_id, timeout_on
    FROM flows_flowsession
   WHERE status = 'W' AND timeout_on < NOW() AND call_id IS NULL
ORDER BY timeout_on ASC
   LIMIT 25000`

type Timeout struct {
	SessionID models.SessionID `db:"session_id"`
	OrgID     models.OrgID     `db:"org_id"`
	ContactID models.ContactID `db:"contact_id"`
	TimeoutOn time.Time        `db:"timeout_on"`
}
