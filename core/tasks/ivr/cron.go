package ivr

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/nyaruka/gocommon/dbutil"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/mailroom"
	"github.com/nyaruka/mailroom/core/ivr"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	retryIVRLock           = "retry_ivr_calls"
	expireIVRLock          = "expire_ivr_calls"
	clearIVRLock           = "clear_ivr_connections"
	changeMaxConnNightLock = "change_ivr_max_conn_night"
	changeMaxConnDayLock   = "change_ivr_max_conn_day"
	cancelIVRCallsLock     = "cancel_ivr_calls"
)

var location *time.Location

func init() {
	mailroom.RegisterCron("retry_ivr_calls", time.Minute, false, func(ctx context.Context, rt *runtime.Runtime) error {
		var err error
		location, err = time.LoadLocation(rt.Config.IVRTimeZone)
		if err != nil {
			return err
		}
		currentHour := time.Now().In(location).Hour()
		if currentHour >= rt.Config.IVRStartHour && currentHour < rt.Config.IVRStopHour {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(rt.Config.IVRRetryTimeout))
			defer cancel()
			return RetryCallsInWorkerPool(ctx, rt)
		}
		return nil
	})

	mailroom.RegisterCron(clearIVRLock, time.Hour, false, ClearStuckChannelConnections)

	mailroom.RegisterCron(changeMaxConnNightLock, time.Minute*10, false, func(ctx context.Context, rt *runtime.Runtime) error {
		currentHour := time.Now().In(location).Hour()
		if currentHour >= rt.Config.IVRStopHour || currentHour < rt.Config.IVRStartHour {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
			defer cancel()
			return ChangeMaxConnectionsConfig(ctx, rt, "TW", 0)
		}
		return nil
	})

	mailroom.RegisterCron(changeMaxConnDayLock, time.Minute*10, false, func(ctx context.Context, rt *runtime.Runtime) error {
		currentHour := time.Now().In(location).Hour()
		if currentHour >= rt.Config.IVRStartHour && currentHour < rt.Config.IVRStopHour {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
			defer cancel()
			return ChangeMaxConnectionsConfig(ctx, rt, "TW", rt.Config.MaxConcurrentEvents)
		}
		return nil
	})

	mailroom.RegisterCron(cancelIVRCallsLock, time.Hour*1, false, func(ctx context.Context, rt *runtime.Runtime) error {
		currentHour := time.Now().In(location).Hour()
		if currentHour == rt.Config.IVRCancelCronStartHour {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*20)
			defer cancel()
			return CancelCalls(ctx, rt)
		}
		return nil
	})

}

// retryCallsInWorkerPoll looks for calls that need to be retried and retries then
func RetryCallsInWorkerPool(ctx context.Context, rt *runtime.Runtime) error {
	log := logrus.WithField("comp", "ivr_cron_retryer")
	start := time.Now()

	conns, err := models.LoadChannelConnectionsToRetry(ctx, rt.DB, rt.Config.IVRConnRetryLimit)
	if err != nil {
		return errors.Wrapf(err, "error loading connections to retry")
	}

	var jobs []Job
	for i := 0; i < len(conns); i++ {
		jobs = append(jobs, Job{Id: i, Conn: conns[i]})
	}

	var (
		wg         sync.WaitGroup
		jobChannel = make(chan Job)
	)

	wg.Add(rt.Config.IVRRetryWorkers)

	for i := 0; i < rt.Config.IVRRetryWorkers; i++ {
		go HandleWork(i, rt, &wg, jobChannel)
	}

	for _, job := range jobs {
		jobChannel <- job
	}

	close(jobChannel)
	wg.Wait()

	log.WithField("count", len(conns)).WithField("elapsed", time.Since(start)).Info("retried errored calls")

	return nil
}

// RetryCalls looks for calls that need to be retried and retries them
func RetryCalls(ctx context.Context, rt *runtime.Runtime) error {
	log := logrus.WithField("comp", "ivr_cron_retryer")
	start := time.Now()

	// find all calls that need restarting
	ctx, cancel := context.WithTimeout(ctx, time.Minute*10)
	defer cancel()

	conns, err := models.LoadChannelConnectionsToRetry(ctx, rt.DB, rt.Config.IVRConnRetryLimit)
	if err != nil {
		return errors.Wrapf(err, "error loading connections to retry")
	}

	throttledChannels := make(map[models.ChannelID]bool)

	// schedules calls for each connection
	for _, conn := range conns {
		log = log.WithField("connection_id", conn.ID())

		// if the channel for this connection is throttled, move on
		/*if throttledChannels[conn.ChannelID()] {
			conn.MarkThrottled(ctx, rt.DB, time.Now())
			log.WithField("channel_id", conn.ChannelID()).Info("skipping connection, throttled")
			continue
		}*/

		// load the org for this connection
		oa, err := models.GetOrgAssets(ctx, rt, conn.OrgID())
		if err != nil {
			log.WithError(err).WithField("org_id", conn.OrgID()).Error("error loading org")
			continue
		}

		// and the associated channel
		channel := oa.ChannelByID(conn.ChannelID())
		if channel == nil {
			// fail this call, channel is no longer active
			err = models.UpdateChannelConnectionStatuses(ctx, rt.DB, []models.ConnectionID{conn.ID()}, models.ConnectionStatusFailed)
			if err != nil {
				log.WithError(err).WithField("channel_id", conn.ChannelID()).Error("error marking call as failed due to missing channel")
			}
			continue
		}

		// finally load the full URN
		urn, err := models.URNForID(ctx, rt.DB, oa, conn.ContactURNID())
		if err != nil {
			log.WithError(err).WithField("urn_id", conn.ContactURNID()).Error("unable to load contact urn")
			continue
		}

		err = ivr.RequestCallStartForConnection(ctx, rt, channel, urn, conn)
		if err != nil {
			log.WithError(err).Error(err)
			continue
		}

		// queued status on a connection we just tried means it is throttled, mark our channel as such
		throttledChannels[conn.ChannelID()] = true
	}

	log.WithField("count", len(conns)).WithField("elapsed", time.Since(start)).Info("retried errored calls")

	return nil
}

func ClearStuckChannelConnections(ctx context.Context, rt *runtime.Runtime) error {
	log := logrus.WithField("comp", "ivr_cron_cleaner")
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	result, err := rt.DB.ExecContext(ctx, clearStuckedChanelConnectionsSQL)
	if err != nil {
		return errors.Wrapf(err, "error cleaning stucked connections")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrapf(err, "error getting rows affected on cleaning stucked connections")
	}
	if rowsAffected > 0 {
		log.WithField("count", rowsAffected).WithField("elapsed", time.Since(start)).Info("stucked channel connections")
	}
	return nil
}

func CancelCalls(ctx context.Context, rt *runtime.Runtime) error {
	log := logrus.WithField("comp", "ivr_cron_canceler")
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, time.Minute*15)
	defer cancel()

	result, err := rt.DB.ExecContext(ctx, cancelQueuedChannelConnectionsSQL)
	if err != nil {
		return errors.Wrapf(err, "error canceling remaining connection calls")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrapf(err, "error getting rows affected on cleaning stucked connections")
	}
	if rowsAffected > 0 {
		log.WithField("count", rowsAffected).WithField("elapsed", time.Since(start)).Info("stucked channel connections")
	}
	return nil
}

func ChangeMaxConnectionsConfig(ctx context.Context, rt *runtime.Runtime, channelType string, maxConcurrentEventsToSet int) error {
	log := logrus.WithField("comp", "ivr_cron_change_max_connections")
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	rows, err := rt.DB.QueryxContext(ctx, selectIVRTWTypeChannelsSQL, channelType)
	if err != nil {
		return errors.Wrapf(err, "error querying for channels")
	}
	defer rows.Close()

	ivrChannels := make([]Channel, 0)

	for rows.Next() {
		ch := Channel{}
		err := dbutil.ScanJSON(rows, &ch)
		if err != nil {
			return errors.Wrapf(err, "error scanning channel")
		}

		ivrChannels = append(ivrChannels, ch)
	}

	for _, ch := range ivrChannels {

		if ch.Config["max_concurrent_events"] == maxConcurrentEventsToSet {
			return nil
		}

		ch.Config["max_concurrent_events"] = maxConcurrentEventsToSet

		configJSON, err := json.Marshal(ch.Config)
		if err != nil {
			return errors.Wrapf(err, "error marshalling channels config")
		}

		_, err = rt.DB.ExecContext(ctx, updateIVRChannelConfigSQL, string(configJSON), ch.ID)
		if err != nil {
			return errors.Wrapf(err, "error updating channels config")
		}
	}

	log.WithField("count", len(ivrChannels)).WithField("elapsed", time.Since(start)).Info("channels that have max_concurrent_events updated")

	return nil
}

const selectIVRTWTypeChannelsSQL = `
	SELECT ROW_TO_JSON(r) FROM (
		SELECT 
			c.id, 
			c.uuid, 
			c.channel_type, 
			COALESCE(c.config, '{}')::json as config, 
			c.is_active 
		FROM 
			channels_channel as c 
		WHERE 
			c.channel_type = $1 AND 
			c.is_active = TRUE ) r;
`

const updateIVRChannelConfigSQL = `
	UPDATE channels_channel
	SET config = $1
	WHERE id = $2
`

const cancelQueuedChannelConnectionsSQL = `
		UPDATE channels_channelconnection
		SET status = 'F'
		WHERE id in (
			SELECT id
			FROM channels_channelconnection
			WHERE
				(status = 'Q' OR status = 'E' OR status = 'P')
		)
`

const clearStuckedChanelConnectionsSQL = `
	UPDATE channels_channelconnection
	SET status = 'F' 
	WHERE id in (
		SELECT id
		FROM channels_channelconnection
		WHERE  
			(status = 'W' OR status = 'R' OR status = 'I') AND
			modified_on < NOW() - INTERVAL '2 DAYS'
		LIMIT  100
	)
`

const selectExpiredRunsSQL = `
	SELECT
		fr.id as run_id,	
		fr.org_id as org_id,
		fr.flow_id as flow_id,
		fr.contact_id as contact_id,
		fr.session_id as session_id,
		fr.expires_on as expires_on,
		cc.id as connection_id
	FROM
		flows_flowrun fr
		JOIN orgs_org o ON fr.org_id = o.id
		JOIN channels_channelconnection cc ON fr.connection_id = cc.id
	WHERE
		fr.is_active = TRUE AND
		fr.expires_on < NOW() AND
		fr.connection_id IS NOT NULL AND
		fr.session_id IS NOT NULL AND
        cc.connection_type = 'V'
	ORDER BY
		expires_on ASC
	LIMIT 100
`

type RunExpiration struct {
	OrgID        models.OrgID        `db:"org_id"`
	FlowID       models.FlowID       `db:"flow_id"`
	ContactID    flows.ContactID     `db:"contact_id"`
	RunID        models.FlowRunID    `db:"run_id"`
	SessionID    models.SessionID    `db:"session_id"`
	ExpiresOn    time.Time           `db:"expires_on"`
	ConnectionID models.ConnectionID `db:"connection_id"`
}

type Channel struct {
	ID          int                    `db:"id" json:"id,omitempty"`
	UUID        string                 `db:"uuid" json:"uuid,omitempty"`
	ChannelType string                 `db:"channel_type" json:"channel_type,omitempty"`
	Config      map[string]interface{} `db:"config" json:"config,omitempty"`
	IsActive    bool                   `db:"is_active" json:"is_active,omitempty"`
}
