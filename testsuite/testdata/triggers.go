package testdata

import (
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
)

func InsertKeywordTrigger(rt *runtime.Runtime, org *Org, flow *Flow, keyword string, matchType models.MatchType, includeGroups []*Group, excludeGroups []*Group, channel *Channel) models.TriggerID {
	return insertTrigger(rt, org, models.KeywordTriggerType, flow, keyword, matchType, models.NilScheduleID, includeGroups, excludeGroups, nil, "", channel)
}

func InsertIncomingCallTrigger(rt *runtime.Runtime, org *Org, flow *Flow, includeGroups, excludeGroups []*Group, channel *Channel) models.TriggerID {
	return insertTrigger(rt, org, models.IncomingCallTriggerType, flow, "", "", models.NilScheduleID, includeGroups, excludeGroups, nil, "", channel)
}

func InsertMissedCallTrigger(rt *runtime.Runtime, org *Org, flow *Flow, channel *Channel) models.TriggerID {
	return insertTrigger(rt, org, models.MissedCallTriggerType, flow, "", "", models.NilScheduleID, nil, nil, nil, "", channel)
}

func InsertNewConversationTrigger(rt *runtime.Runtime, org *Org, flow *Flow, channel *Channel) models.TriggerID {
	return insertTrigger(rt, org, models.NewConversationTriggerType, flow, "", "", models.NilScheduleID, nil, nil, nil, "", channel)
}

func InsertOptInTrigger(rt *runtime.Runtime, org *Org, flow *Flow, channel *Channel) models.TriggerID {
	return insertTrigger(rt, org, models.OptInTriggerType, flow, "", "", models.NilScheduleID, nil, nil, nil, "", channel)
}

func InsertOptOutTrigger(rt *runtime.Runtime, org *Org, flow *Flow, channel *Channel) models.TriggerID {
	return insertTrigger(rt, org, models.OptOutTriggerType, flow, "", "", models.NilScheduleID, nil, nil, nil, "", channel)
}

func InsertReferralTrigger(rt *runtime.Runtime, org *Org, flow *Flow, referrerID string, channel *Channel) models.TriggerID {
	return insertTrigger(rt, org, models.ReferralTriggerType, flow, "", "", models.NilScheduleID, nil, nil, nil, referrerID, channel)
}

func InsertCatchallTrigger(rt *runtime.Runtime, org *Org, flow *Flow, includeGroups, excludeGroups []*Group, channel *Channel) models.TriggerID {
	return insertTrigger(rt, org, models.CatchallTriggerType, flow, "", "", models.NilScheduleID, includeGroups, excludeGroups, nil, "", channel)
}

func InsertScheduledTrigger(rt *runtime.Runtime, org *Org, flow *Flow, schedID models.ScheduleID, includeGroups, excludeGroups []*Group, includeContacts []*Contact) models.TriggerID {
	return insertTrigger(rt, org, models.ScheduleTriggerType, flow, "", "", schedID, includeGroups, excludeGroups, includeContacts, "", nil)
}

func InsertTicketClosedTrigger(rt *runtime.Runtime, org *Org, flow *Flow) models.TriggerID {
	return insertTrigger(rt, org, models.TicketClosedTriggerType, flow, "", "", models.NilScheduleID, nil, nil, nil, "", nil)
}

func insertTrigger(rt *runtime.Runtime, org *Org, triggerType models.TriggerType, flow *Flow, keyword string, matchType models.MatchType, schedID models.ScheduleID, includeGroups, excludeGroups []*Group, contactIDs []*Contact, referrerID string, channel *Channel) models.TriggerID {
	channelID := models.NilChannelID
	if channel != nil {
		channelID = channel.ID
	}

	var id models.TriggerID
	must(rt.DB.Get(&id,
		`INSERT INTO triggers_trigger(is_active, created_on, modified_on, keyword, referrer_id, is_archived, 
									  flow_id, trigger_type, match_type, schedule_id, created_by_id, modified_by_id, org_id, channel_id)
		VALUES(TRUE, now(), now(), $1, $6, false, $2, $3, $4, $5, 1, 1, $8, $7) RETURNING id`, keyword, flow.ID, triggerType, matchType, schedID, referrerID, channelID, org.ID,
	))

	// insert group associations
	for _, g := range includeGroups {
		rt.DB.MustExec(`INSERT INTO triggers_trigger_groups(trigger_id, contactgroup_id) VALUES($1, $2)`, id, g.ID)
	}
	for _, g := range excludeGroups {
		rt.DB.MustExec(`INSERT INTO triggers_trigger_exclude_groups(trigger_id, contactgroup_id) VALUES($1, $2)`, id, g.ID)
	}

	// insert contact associations
	for _, c := range contactIDs {
		rt.DB.MustExec(`INSERT INTO triggers_trigger_contacts(trigger_id, contact_id) VALUES($1, $2)`, id, c.ID)
	}

	return id
}
