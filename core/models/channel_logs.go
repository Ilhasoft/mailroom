package models

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/mailroom/utils/clogs"
)

// ChannelLogID is our type for a channel log id
type ChannelLogID int64

const (
	ChannelLogTypeIVRStart    clogs.LogType = "ivr_start"
	ChannelLogTypeIVRIncoming clogs.LogType = "ivr_incoming"
	ChannelLogTypeIVRCallback clogs.LogType = "ivr_callback"
	ChannelLogTypeIVRStatus   clogs.LogType = "ivr_status"
	ChannelLogTypeIVRHangup   clogs.LogType = "ivr_hangup"
)

// ChannelLog stores the HTTP traces and errors generated by an interaction with a channel.
type ChannelLog struct {
	*clogs.Log

	channel  *Channel
	attached bool
}

// NewChannelLog creates a new channel log with the given type and channel
func NewChannelLog(t clogs.LogType, ch *Channel, redactVals []string) *ChannelLog {
	return newChannelLog(t, ch, nil, redactVals)
}

// NewChannelLogForIncoming creates a new channel log for an incoming request
func NewChannelLogForIncoming(t clogs.LogType, ch *Channel, r *httpx.Recorder, redactVals []string) *ChannelLog {
	return newChannelLog(t, ch, r, redactVals)
}

func newChannelLog(t clogs.LogType, ch *Channel, r *httpx.Recorder, redactVals []string) *ChannelLog {
	return &ChannelLog{
		Log:     clogs.NewLog(t, r, redactVals),
		channel: ch,
	}
}

// if we have an error or a non 2XX/3XX http response then log is considered an error
func (l *ChannelLog) isError() bool {
	if len(l.Errors) > 0 {
		return true
	}

	for _, l := range l.HttpLogs {
		if l.StatusCode < 200 || l.StatusCode >= 400 {
			return true
		}
	}

	return false
}

const sqlInsertChannelLog = `
INSERT INTO channels_channellog( uuid,  channel_id,  log_type,  http_logs,  errors,  is_error,  elapsed_ms,  created_on)
                         VALUES(:uuid, :channel_id, :log_type, :http_logs, :errors, :is_error, :elapsed_ms, :created_on)
  RETURNING id`

// channel log to be inserted into the database
type dbChannelLog struct {
	ID        ChannelLogID    `db:"id"`
	UUID      clogs.LogUUID   `db:"uuid"`
	ChannelID ChannelID       `db:"channel_id"`
	Type      clogs.LogType   `db:"log_type"`
	HTTPLogs  json.RawMessage `db:"http_logs"`
	Errors    json.RawMessage `db:"errors"`
	IsError   bool            `db:"is_error"`
	ElapsedMS int             `db:"elapsed_ms"`
	CreatedOn time.Time       `db:"created_on"`
}

// InsertChannelLogs writes the given channel logs to the db
func InsertChannelLogs(ctx context.Context, rt *runtime.Runtime, logs []*ChannelLog) error {
	// write all logs to DynamoDB
	cls := make([]*clogs.Log, len(logs))
	for i, l := range logs {
		cls[i] = l.Log
	}
	if err := clogs.BatchPut(ctx, rt.Dynamo, "ChannelLogs", cls); err != nil {
		return fmt.Errorf("error writing channel logs: %w", err)
	}

	unattached := make([]*dbChannelLog, 0, len(logs))

	for _, l := range logs {
		if !l.attached {
			// if log isn't attached to a message or call we need to write it to the db so that it's retrievable
			unattached = append(unattached, &dbChannelLog{
				UUID:      l.UUID,
				ChannelID: l.channel.ID(),
				Type:      l.Type,
				HTTPLogs:  jsonx.MustMarshal(l.HttpLogs),
				Errors:    jsonx.MustMarshal(l.Errors),
				IsError:   l.isError(),
				CreatedOn: l.CreatedOn,
				ElapsedMS: int(l.Elapsed / time.Millisecond),
			})
		}
	}

	if len(unattached) > 0 {
		err := BulkQuery(ctx, "insert channel log", rt.DB, sqlInsertChannelLog, unattached)
		if err != nil {
			return fmt.Errorf("error inserting unattached channel logs: %w", err)
		}
	}

	return nil
}
