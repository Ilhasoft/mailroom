package testdata

import (
	"github.com/lib/pq"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/goflow/assets"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
)

type Channel struct {
	ID   models.ChannelID
	UUID assets.ChannelUUID
	Type models.ChannelType
}

// InsertChannel inserts a channel
func InsertChannel(rt *runtime.Runtime, org *Org, channelType models.ChannelType, name, address string, schemes []string, role string, config map[string]any) *Channel {
	uuid := assets.ChannelUUID(uuids.New())
	var id models.ChannelID
	must(rt.DB.Get(&id,
		`INSERT INTO channels_channel(uuid, org_id, channel_type, name, address, schemes, role, config, last_seen, is_system, log_policy, is_active, created_on, modified_on, created_by_id, modified_by_id) 
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, NOW(), FALSE, 'A', TRUE, NOW(), NOW(), 1, 1) RETURNING id`, uuid, org.ID, channelType, name, address, pq.Array(schemes), role, models.JSONMap(config),
	))
	return &Channel{ID: id, UUID: uuid, Type: channelType}
}

// InsertChannelEvent inserts a channel event
func InsertChannelEvent(rt *runtime.Runtime, org *Org, eventType models.ChannelEventType, channel *Channel, contact *Contact, status models.ChannelEventStatus) models.ChannelEventID {
	var id models.ChannelEventID
	must(rt.DB.Get(&id,
		`INSERT INTO channels_channelevent(org_id, event_type, status, channel_id, contact_id, contact_urn_id, extra, created_on, occurred_on)
		VALUES($1, $2, $3, $4, $5, $6, '{}', NOW(), NOW()) RETURNING id`, org.ID, eventType, status, channel.ID, contact.ID, contact.URNID,
	))
	return id
}

// InsertCall inserts a call
func InsertCall(rt *runtime.Runtime, org *Org, channel *Channel, contact *Contact) models.CallID {
	var id models.CallID
	must(rt.DB.Get(&id,
		`INSERT INTO ivr_call(created_on, modified_on, external_id, status, direction, error_count, org_id, channel_id, contact_id, contact_urn_id) 
		VALUES(NOW(), NOW(), 'ext1', 'I', 'I', 0, $1, $2, $3, $4) RETURNING id`, org.ID, channel.ID, contact.ID, contact.URNID,
	))
	return id
}
