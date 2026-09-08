package models

import (
	"context"
	"database/sql"

	"github.com/nyaruka/goflow/flows"
	"github.com/pkg/errors"
)

// WAConversationHandover is a pending WhatsApp Conversation Orchestration handover
// waiting to be attached to the next inbound that reaches the brain router.
type WAConversationHandover struct {
	ID          int64  `db:"id"`
	ContextText string `db:"context_text"`
}

const lookupPendingWAConversationHandoverSQL = `
SELECT id, context_text
FROM wa_conversation_handover
WHERE channel_id = $1 AND contact_id = $2 AND consumed_on IS NULL
`

// LookupPendingWAConversationHandover returns the unconsumed handover for the
// channel+contact pair, or nil if none exists.
func LookupPendingWAConversationHandover(ctx context.Context, db Queryer, channelID ChannelID, contactID ContactID) (*WAConversationHandover, error) {
	h := &WAConversationHandover{}
	err := db.GetContext(ctx, h, lookupPendingWAConversationHandoverSQL, channelID, contactID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "error looking up pending wa conversation handover")
	}
	return h, nil
}

const consumePendingWAConversationHandoverSQL = `
UPDATE wa_conversation_handover
SET consumed_on = NOW(), consumed_msg_id = $1
WHERE id = $2 AND consumed_on IS NULL
`

// ConsumePendingWAConversationHandover marks the handover consumed by msgID.
// Returns false when another worker already consumed the row.
func ConsumePendingWAConversationHandover(ctx context.Context, db Queryer, id int64, msgID flows.MsgID) (bool, error) {
	res, err := db.ExecContext(ctx, consumePendingWAConversationHandoverSQL, msgID, id)
	if err != nil {
		return false, errors.Wrapf(err, "error consuming pending wa conversation handover")
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, errors.Wrapf(err, "error reading rows affected for wa conversation handover consume")
	}
	return n > 0, nil
}
