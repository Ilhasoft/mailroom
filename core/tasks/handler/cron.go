package handler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/redisx"
)

func init() {
	tasks.RegisterCron("retry_msgs", NewRetryPendingCron())
}

type RetryPendingCron struct {
	marker *redisx.IntervalSet
}

func NewRetryPendingCron() *RetryPendingCron {
	return &RetryPendingCron{
		marker: redisx.NewIntervalSet("retried_msgs", time.Hour*24, 2),
	}
}

func (c *RetryPendingCron) Next(last time.Time) time.Time {
	return tasks.CronNext(last, time.Minute*5)
}

func (c *RetryPendingCron) AllInstances() bool {
	return false
}

// looks for any pending msgs older than five minutes and queues them to be handled again
func (c *RetryPendingCron) Run(ctx context.Context, rt *runtime.Runtime) (map[string]any, error) {
	if !rt.Config.RetryPendingMessages {
		return nil, nil
	}

	log := slog.With("comp", "handler_retrier")

	rc := rt.RP.Get()
	defer rc.Close()

	// check the size of our handle queue
	handlerSize, err := tasks.HandlerQueue.Size(rc)
	if err != nil {
		return nil, fmt.Errorf("error finding size of handler queue: %w", err)
	}

	// if our queue has items in it, don't queue anything else in there, wait for it to be empty
	if handlerSize > 0 {
		log.Info("not retrying any messages, have messages in handler queue")
		return nil, nil
	}

	// get all incoming messages that are still empty
	rows, err := rt.DB.Queryx(unhandledMsgsQuery)
	if err != nil {
		return nil, fmt.Errorf("error querying for unhandled messages: %w", err)
	}
	defer rows.Close()

	retried := 0
	for rows.Next() {
		var orgID models.OrgID
		var contactID models.ContactID
		var eventJSON []byte
		var msgID models.MsgID

		err = rows.Scan(&orgID, &contactID, &msgID, &eventJSON)
		if err != nil {
			return nil, fmt.Errorf("error scanning msg row: %w", err)
		}

		// our key is built such that we will only retry once an hour
		key := fmt.Sprintf("%d_%d", msgID, time.Now().Hour())

		dupe, err := c.marker.IsMember(rc, key)
		if err != nil {
			return nil, fmt.Errorf("error checking for dupe retry: %w", err)
		}

		// we already retried this, skip
		if dupe {
			continue
		}

		task, err := readTask("msg_event", eventJSON) // TODO find a better way to do this
		if err != nil {
			return nil, fmt.Errorf("error reading msg data as task: %w", err)
		}

		// queue this event up for handling
		err = QueueTask(rc, orgID, contactID, task)
		if err != nil {
			return nil, fmt.Errorf("error queuing retry for task: %w", err)
		}

		// mark it as queued
		err = c.marker.Add(rc, key)
		if err != nil {
			return nil, fmt.Errorf("error marking task for retry: %w", err)
		}

		retried++
	}

	return map[string]any{"retried": retried}, nil
}

const unhandledMsgsQuery = `
SELECT org_id, contact_id, msg_id, ROW_TO_JSON(r) FROM (SELECT
	m.contact_id AS contact_id,
	m.org_id AS org_id, 
	c.id AS channel_id,
	c.uuid AS channel_uuid,
	c.channel_type AS channel_type,
	m.id AS msg_id,
	m.uuid AS msg_uuid,
	m.external_id AS msg_external_id,
	u.identity AS urn,
	m.contact_urn_id AS urn_id,
	m.text AS text,
	m.attachments AS attachments
FROM
	msgs_msg m
	INNER JOIN channels_channel c ON c.id = m.channel_id 
	INNER JOIN contacts_contacturn u ON u.id = m.contact_urn_id
WHERE
	m.direction = 'I' AND m.status = 'P' AND m.created_on < now() - INTERVAL '5 min'
) r;
`
