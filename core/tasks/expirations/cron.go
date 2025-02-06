package expirations

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/nyaruka/mailroom/core/ivr"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/core/tasks/handler"
	"github.com/nyaruka/mailroom/core/tasks/handler/ctasks"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/redisx"
)

func init() {
	tasks.RegisterCron("run_expirations", NewExpirationsCron(10, 100))
	tasks.RegisterCron("expire_ivr_calls", &VoiceExpirationsCron{})
}

type ExpirationsCron struct {
	marker        *redisx.IntervalSet
	bulkThreshold int // use bulk task for any org with this or more expirations
	bulkBatchSize int // number of expirations to queue in a single bulk task
}

func NewExpirationsCron(bulkThreshold, bulkBatchSize int) *ExpirationsCron {
	return &ExpirationsCron{
		marker:        redisx.NewIntervalSet("run_expirations", time.Hour*24, 2),
		bulkThreshold: bulkThreshold,
		bulkBatchSize: bulkBatchSize,
	}
}

func (c *ExpirationsCron) Next(last time.Time) time.Time {
	return tasks.CronNext(last, time.Minute)
}

func (c *ExpirationsCron) AllInstances() bool {
	return false
}

// handles waiting messaging sessions whose waits have expired, resuming those that can be resumed,
// and expiring those that can't
func (c *ExpirationsCron) Run(ctx context.Context, rt *runtime.Runtime) (map[string]any, error) {
	rows, err := rt.DB.QueryxContext(ctx, sqlSelectExpiredWaits)
	if err != nil {
		return nil, fmt.Errorf("error querying sessions with expired waits: %w", err)
	}
	defer rows.Close()

	taskID := func(w *ExpiredWait) string {
		return fmt.Sprintf("%d:%s", w.SessionID, w.WaitExpiresOn.Format(time.RFC3339))
	}

	// scan and organize by org
	byOrg := make(map[models.OrgID][]*ExpiredWait, 50)

	// the sessions that can't be resumed and will be exited
	toExit := make([]models.SessionID, 0, 100)

	rc := rt.RP.Get()
	defer rc.Close()

	numDupes, numQueuedHandler, numQueuedBulk, numExited := 0, 0, 0, 0

	for rows.Next() {
		expiredWait := &ExpiredWait{}
		if err := rows.StructScan(expiredWait); err != nil {
			return nil, fmt.Errorf("error scanning expired wait: %w", err)
		}

		if !expiredWait.WaitResumes {
			toExit = append(toExit, expiredWait.SessionID)
			continue
		}

		// check whether we've already queued this
		queued, err := c.marker.IsMember(rc, taskID(expiredWait))
		if err != nil {
			return nil, fmt.Errorf("error checking whether expiration is already queued: %w", err)
		}

		// already queued? move on
		if queued {
			numDupes++
			continue
		}

		byOrg[expiredWait.OrgID] = append(byOrg[expiredWait.OrgID], expiredWait)
	}

	for orgID, expirations := range byOrg {
		throttle := len(expirations) >= c.bulkThreshold

		for batch := range slices.Chunk(expirations, c.bulkBatchSize) {
			if throttle {
				if err := tasks.Queue(rc, tasks.ThrottledQueue, orgID, &BulkExpireTask{Expirations: batch}, true); err != nil {
					return nil, fmt.Errorf("error queuing bulk expiration task to throttle queue: %w", err)
				}
				numQueuedBulk += len(batch)
			}

			for _, exp := range batch {
				if !throttle {
					err := handler.QueueTask(rc, orgID, exp.ContactID, ctasks.NewWaitExpiration(exp.SessionID, exp.WaitExpiresOn))
					if err != nil {
						return nil, fmt.Errorf("error queuing expiration task to handler queue: %w", err)
					}
					numQueuedHandler++
				}

				// mark as queued
				if err = c.marker.Add(rc, taskID(exp)); err != nil {
					return nil, fmt.Errorf("error marking expiration task as queued: %w", err)
				}
			}
		}
	}

	// exit the sessions that can't be resumed
	for batch := range slices.Chunk(toExit, 500) {
		err = models.ExitSessions(ctx, rt.DB, batch, models.SessionStatusExpired)
		if err != nil {
			return nil, fmt.Errorf("error exiting expired sessions: %w", err)
		}
		numExited += len(batch)
	}

	return map[string]any{"exited": numExited, "dupes": numDupes, "queued_handler": numQueuedHandler, "queued_bulk": numQueuedBulk}, nil
}

const sqlSelectExpiredWaits = `
    SELECT s.id as session_id, s.org_id, s.wait_expires_on, s.wait_resume_on_expire , s.contact_id
      FROM flows_flowsession s
     WHERE s.session_type = 'M' AND s.status = 'W' AND s.wait_expires_on <= NOW()
  ORDER BY s.wait_expires_on ASC
     LIMIT 25000`

type ExpiredWait struct {
	SessionID     models.SessionID `db:"session_id"             json:"session_id"`
	OrgID         models.OrgID     `db:"org_id"                 json:"-"`
	WaitExpiresOn time.Time        `db:"wait_expires_on"        json:"wait_expires_on"`
	WaitResumes   bool             `db:"wait_resume_on_expire"  json:"-"`
	ContactID     models.ContactID `db:"contact_id"             json:"contact_id"`
}

type VoiceExpirationsCron struct{}

func (c *VoiceExpirationsCron) Next(last time.Time) time.Time {
	return tasks.CronNext(last, time.Minute)
}

func (c *VoiceExpirationsCron) AllInstances() bool {
	return false
}

// looks for voice sessions that should be expired and ends them
func (c *VoiceExpirationsCron) Run(ctx context.Context, rt *runtime.Runtime) (map[string]any, error) {
	log := slog.With("comp", "ivr_cron_expirer")

	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	// select voice sessions with expired waits
	rows, err := rt.DB.QueryxContext(ctx, sqlSelectExpiredVoiceWaits)
	if err != nil {
		return nil, fmt.Errorf("error querying voice sessions with expired waits: %w", err)
	}
	defer rows.Close()

	expiredSessions := make([]models.SessionID, 0, 100)
	clogs := make([]*models.ChannelLog, 0, 100)

	for rows.Next() {
		expiredWait := &ExpiredVoiceWait{}
		err := rows.StructScan(expiredWait)
		if err != nil {
			return nil, fmt.Errorf("error scanning expired wait: %w", err)
		}

		// add the session to those we need to expire
		expiredSessions = append(expiredSessions, expiredWait.SessionID)

		// load our call
		conn, err := models.GetCallByID(ctx, rt.DB, expiredWait.OrgID, expiredWait.CallID)
		if err != nil {
			log.Error("unable to load call", "error", err, "call_id", expiredWait.CallID)
			continue
		}

		// hang up our call
		clog, err := ivr.HangupCall(ctx, rt, conn)
		if err != nil {
			// log error but carry on with other calls
			log.Error("error hanging up call", "error", err, "call_id", conn.ID())
		}

		if clog != nil {
			clogs = append(clogs, clog)
		}
	}

	// now expire our runs and sessions
	if len(expiredSessions) > 0 {
		err := models.ExitSessions(ctx, rt.DB, expiredSessions, models.SessionStatusExpired)
		if err != nil {
			log.Error("error expiring sessions for expired calls", "error", err)
		}
	}

	if err := models.InsertChannelLogs(ctx, rt, clogs); err != nil {
		return nil, fmt.Errorf("error inserting channel logs: %w", err)
	}

	return map[string]any{"expired": len(expiredSessions)}, nil
}

const sqlSelectExpiredVoiceWaits = `
  SELECT id, org_id, call_id, wait_expires_on
    FROM flows_flowsession
   WHERE session_type = 'V' AND status = 'W' AND wait_expires_on <= NOW()
ORDER BY wait_expires_on ASC
   LIMIT 100`

type ExpiredVoiceWait struct {
	SessionID models.SessionID `db:"id"`
	OrgID     models.OrgID     `db:"org_id"`
	CallID    models.CallID    `db:"call_id"`
	ExpiresOn time.Time        `db:"wait_expires_on"`
}
