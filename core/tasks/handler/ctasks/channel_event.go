package ctasks

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/goflow/excellent/types"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/triggers"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/runner"
	"github.com/nyaruka/mailroom/core/tasks/handler"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/null/v3"
	"github.com/pkg/errors"
)

const TypeChannelEvent = "channel_event"

func init() {
	handler.RegisterContactTask(TypeChannelEvent, func() handler.Task { return &ChannelEventTask{} })
}

type ChannelEventTask struct {
	EventID    models.ChannelEventID   `json:"event_id"`
	EventType  models.ChannelEventType `json:"event_type"`
	ChannelID  models.ChannelID        `json:"channel_id"`
	URNID      models.URNID            `json:"urn_id"`
	OptInID    models.OptInID          `json:"optin_id"`
	Extra      null.Map[any]           `json:"extra"`
	NewContact bool                    `json:"new_contact"`
	CreatedOn  time.Time               `json:"created_on"`
}

func (t *ChannelEventTask) Type() string {
	return TypeChannelEvent
}

func (t *ChannelEventTask) UseReadOnly() bool {
	return !t.NewContact
}

func (t *ChannelEventTask) Perform(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, contact *models.Contact) error {
	_, err := t.handle(ctx, rt, oa, contact, nil)
	if err != nil {
		return err
	}

	return models.MarkChannelEventHandled(ctx, rt.DB, t.EventID)
}

// Handle let's us reuse this task's code for handling incoming calls.. which we need to perform inline in the IVR web
// handler rather than as a queued task.
func (t *ChannelEventTask) Handle(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, contact *models.Contact, call *models.Call) (*models.Session, error) {
	return t.handle(ctx, rt, oa, contact, call)
}

func (t *ChannelEventTask) handle(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, contact *models.Contact, call *models.Call) (*models.Session, error) {
	channel := oa.ChannelByID(t.ChannelID)

	// if contact is deleted or blocked or channel no longer exists, nothing to do
	if contact == nil || contact.Status() == models.ContactStatusBlocked || channel == nil {
		return nil, nil
	}

	if t.EventType == models.EventTypeStopContact {
		err := contact.Stop(ctx, rt.DB, oa)
		if err != nil {
			return nil, errors.Wrapf(err, "error stopping contact")
		}
	}

	if models.ContactSeenEvents[t.EventType] {
		err := contact.UpdateLastSeenOn(ctx, rt.DB, t.CreatedOn)
		if err != nil {
			return nil, errors.Wrap(err, "error updating contact last_seen_on")
		}
	}

	// make sure this URN is our highest priority (this is usually a noop)
	err := contact.UpdatePreferredURN(ctx, rt.DB, oa, t.URNID, channel)
	if err != nil {
		return nil, errors.Wrapf(err, "error changing primary URN")
	}

	// build our flow contact
	flowContact, err := contact.FlowContact(oa)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating flow contact")
	}

	if t.NewContact {
		err = models.CalculateDynamicGroups(ctx, rt.DB, oa, []*flows.Contact{flowContact})
		if err != nil {
			return nil, errors.Wrapf(err, "unable to initialize new contact")
		}
	}

	// do we have associated trigger?
	var trigger *models.Trigger
	var flow *models.Flow

	switch t.EventType {
	case models.EventTypeNewConversation:
		trigger = models.FindMatchingNewConversationTrigger(oa, channel)
	case models.EventTypeReferral:
		referrerID, _ := t.Extra["referrer_id"].(string)
		trigger = models.FindMatchingReferralTrigger(oa, channel, referrerID)
	case models.EventTypeMissedCall:
		trigger = models.FindMatchingMissedCallTrigger(oa, channel)
	case models.EventTypeIncomingCall:
		trigger = models.FindMatchingIncomingCallTrigger(oa, channel, flowContact)
	case models.EventTypeOptIn:
		trigger = models.FindMatchingOptInTrigger(oa, channel)
	case models.EventTypeOptOut:
		trigger = models.FindMatchingOptOutTrigger(oa, channel)
	case models.EventTypeWelcomeMessage, models.EventTypeStopContact:
		trigger = nil
	default:
		return nil, errors.Errorf("unknown channel event type: %s", t.EventType)
	}

	if trigger != nil {
		flow, err = oa.FlowByID(trigger.FlowID())
		if err != nil && err != models.ErrNotFound {
			return nil, errors.Wrap(err, "error loading flow for trigger")
		}
	}

	// no trigger or flow gone, nothing to do
	if flow == nil {
		return nil, nil
	}

	// if this is an IVR flow and we don't have a call, trigger that asynchronously
	if flow.FlowType() == models.FlowTypeVoice && call == nil {
		err = handler.TriggerIVRFlow(ctx, rt, oa.OrgID(), flow.ID(), []models.ContactID{contact.ID()}, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "error while triggering ivr flow")
		}
		return nil, nil
	}

	// create our parameters, we just convert this from JSON
	var params *types.XObject
	if t.Extra != nil {
		asJSON, err := json.Marshal(map[string]any(t.Extra))
		if err != nil {
			return nil, errors.Wrapf(err, "unable to marshal extra from channel event")
		}
		params, err = types.ReadXObject(asJSON)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to read extra from channel event")
		}
	}

	var flowOptIn *flows.OptIn
	if t.EventType == models.EventTypeOptIn || t.EventType == models.EventTypeOptOut {
		optIn := oa.OptInByID(t.OptInID)
		if optIn != nil {
			flowOptIn = oa.SessionAssets().OptIns().Get(optIn.UUID())
		}
	}

	// build our flow trigger
	tb := triggers.NewBuilder(oa.Env(), flow.Reference(), flowContact)
	var trig flows.Trigger

	if t.EventType == models.EventTypeIncomingCall {
		urn := contact.URNForID(t.URNID)
		trig = tb.Channel(channel.Reference(), triggers.ChannelEventTypeIncomingCall).WithCall(urn).Build()
	} else if t.EventType == models.EventTypeOptIn && flowOptIn != nil {
		trig = tb.OptIn(flowOptIn, triggers.OptInEventTypeStarted).Build()
	} else if t.EventType == models.EventTypeOptOut && flowOptIn != nil {
		trig = tb.OptIn(flowOptIn, triggers.OptInEventTypeStopped).Build()
	} else {
		trig = tb.Channel(channel.Reference(), triggers.ChannelEventType(t.EventType)).WithParams(params).Build()
	}

	// if we have a channel connection we set the connection on the session before our event hooks fire
	// so that IVR messages can be created with the right connection reference
	var hook models.SessionCommitHook
	if flow.FlowType() == models.FlowTypeVoice && call != nil {
		hook = func(ctx context.Context, tx *sqlx.Tx, rp *redis.Pool, oa *models.OrgAssets, sessions []*models.Session) error {
			for _, session := range sessions {
				session.SetCall(call)
			}
			return nil
		}
	}

	sessions, err := runner.StartFlowForContacts(ctx, rt, oa, flow, []*models.Contact{contact}, []flows.Trigger{trig}, hook, flow.FlowType().Interrupts())
	if err != nil {
		return nil, errors.Wrapf(err, "error starting flow for contact")
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	return sessions[0], nil
}
