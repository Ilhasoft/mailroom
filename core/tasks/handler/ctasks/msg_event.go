package ctasks

import (
	"context"
	"fmt"

	"github.com/gomodule/redigo/redis"
	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/events"
	"github.com/nyaruka/goflow/flows/resumes"
	"github.com/nyaruka/goflow/flows/triggers"
	"github.com/nyaruka/goflow/utils"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/msgio"
	"github.com/nyaruka/mailroom/core/runner"
	"github.com/nyaruka/mailroom/core/tasks/handler"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/mailroom/utils/clogs"
	"github.com/nyaruka/null/v3"
)

const TypeMsgEvent = "msg_event"

func init() {
	handler.RegisterContactTask(TypeMsgEvent, func() handler.Task { return &MsgEventTask{} })
}

type MsgEventTask struct {
	MsgID         models.MsgID     `json:"msg_id"`
	MsgUUID       flows.MsgUUID    `json:"msg_uuid"`
	MsgExternalID null.String      `json:"msg_external_id"`
	ChannelID     models.ChannelID `json:"channel_id"`
	URN           urns.URN         `json:"urn"`
	URNID         models.URNID     `json:"urn_id"`
	Text          string           `json:"text"`
	Attachments   []string         `json:"attachments,omitempty"`
	NewContact    bool             `json:"new_contact"`
}

func (t *MsgEventTask) Type() string {
	return TypeMsgEvent
}

func (t *MsgEventTask) UseReadOnly() bool {
	return !t.NewContact
}

func (t *MsgEventTask) Perform(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, contact *models.Contact) error {
	channel := oa.ChannelByID(t.ChannelID)

	// fetch the attachments on the message (i.e. ask courier to fetch them)
	attachments := make([]utils.Attachment, 0, len(t.Attachments))
	logUUIDs := make([]clogs.LogUUID, 0, len(t.Attachments))

	// no channel, no attachments
	if channel != nil {
		for _, attURL := range t.Attachments {
			// if courier has already fetched this attachment, use it as is
			if utils.Attachment(attURL).ContentType() != "" {
				attachments = append(attachments, utils.Attachment(attURL))
			} else {
				attachment, logUUID, err := msgio.FetchAttachment(ctx, rt, channel, attURL, t.MsgID)
				if err != nil {
					return fmt.Errorf("error fetching attachment '%s': %w", attURL, err)
				}

				attachments = append(attachments, attachment)
				logUUIDs = append(logUUIDs, logUUID)
			}
		}
	}

	// if contact is blocked, or channel no longer exists, ignore this message but mark it as handled
	if contact.Status() == models.ContactStatusBlocked || channel == nil {
		err := models.MarkMessageHandled(ctx, rt.DB, t.MsgID, models.MsgStatusHandled, models.VisibilityArchived, models.NilFlowID, models.NilTicketID, attachments, logUUIDs)
		if err != nil {
			return fmt.Errorf("error updating message for deleted contact: %w", err)
		}
		return nil
	}

	// if we have URNs make sure the message URN is our highest priority (this is usually a noop)
	if len(contact.URNs()) > 0 {
		err := contact.UpdatePreferredURN(ctx, rt.DB, oa, t.URNID, channel)
		if err != nil {
			return fmt.Errorf("error changing primary URN: %w", err)
		}
	}

	// stopped contact? they are unstopped if they send us an incoming message
	recalcGroups := t.NewContact
	if contact.Status() == models.ContactStatusStopped {
		err := contact.Unstop(ctx, rt.DB)
		if err != nil {
			return fmt.Errorf("error unstopping contact: %w", err)
		}

		recalcGroups = true
	}

	// build our flow contact
	flowContact, err := contact.FlowContact(oa)
	if err != nil {
		return fmt.Errorf("error creating flow contact: %w", err)
	}

	// if this is a new or newly unstopped contact, we need to calculate dynamic groups and campaigns
	if recalcGroups {
		err = models.CalculateDynamicGroups(ctx, rt.DB, oa, []*flows.Contact{flowContact})
		if err != nil {
			return fmt.Errorf("unable to initialize new contact: %w", err)
		}
	}

	// look up any open tickes for this contact and forward this message to that
	ticket, err := models.LoadOpenTicketForContact(ctx, rt.DB, contact)
	if err != nil {
		return fmt.Errorf("unable to look up open tickets for contact: %w", err)
	}

	// find any matching triggers
	trigger, keyword := models.FindMatchingMsgTrigger(oa, channel, flowContact, t.Text)

	// look for a waiting session for this contact
	session, err := models.FindWaitingSessionForContact(ctx, rt, oa, models.FlowTypeMessaging, flowContact)
	if err != nil {
		return fmt.Errorf("error loading active session for contact: %w", err)
	}

	// we have a session and it has an active flow, check whether we should honor triggers
	var flow *models.Flow
	if session != nil && session.CurrentFlowID() != models.NilFlowID {
		flow, err = oa.FlowByID(session.CurrentFlowID())

		// flow this session is in is gone, interrupt our session and reset it
		if err == models.ErrNotFound {
			err = models.ExitSessions(ctx, rt.DB, []models.SessionID{session.ID()}, models.SessionStatusFailed)
			session = nil
		}

		if err != nil {
			return fmt.Errorf("error loading flow for session: %w", err)
		}
	}

	// flow will only see the attachments we were able to fetch
	availableAttachments := make([]utils.Attachment, 0, len(attachments))
	for _, att := range attachments {
		if att.ContentType() != utils.UnavailableType {
			availableAttachments = append(availableAttachments, att)
		}
	}

	msgIn := flows.NewMsgIn(t.MsgUUID, t.URN, channel.Reference(), t.Text, availableAttachments)
	msgIn.SetExternalID(string(t.MsgExternalID))
	msgIn.SetID(flows.MsgID(t.MsgID))

	// build our hook to mark a flow message as handled
	flowMsgHook := func(ctx context.Context, tx *sqlx.Tx, rp *redis.Pool, oa *models.OrgAssets, sessions []*models.Session) error {
		// set our incoming message event on our session
		if len(sessions) != 1 {
			return fmt.Errorf("handle hook called with more than one session")
		}
		sessions[0].SetIncomingMsg(t.MsgID, t.MsgExternalID)

		return markMsgHandled(ctx, tx, msgIn, flow, attachments, ticket, logUUIDs)
	}

	// we found a trigger and their session is nil or doesn't ignore keywords
	if (trigger != nil && trigger.TriggerType() != models.CatchallTriggerType && (flow == nil || !flow.IgnoreTriggers())) ||
		(trigger != nil && trigger.TriggerType() == models.CatchallTriggerType && (flow == nil)) {
		// load our flow
		flow, err = oa.FlowByID(trigger.FlowID())
		if err != nil && err != models.ErrNotFound {
			return fmt.Errorf("error loading flow for trigger: %w", err)
		}

		// trigger flow is still active, start it
		if flow != nil {
			// if this is an IVR flow, we need to trigger that start (which happens in a different queue)
			if flow.FlowType() == models.FlowTypeVoice {
				ivrMsgHook := func(ctx context.Context, tx *sqlx.Tx) error {
					return markMsgHandled(ctx, tx, msgIn, flow, attachments, ticket, logUUIDs)
				}
				err = handler.TriggerIVRFlow(ctx, rt, oa.OrgID(), flow.ID(), []models.ContactID{contact.ID()}, ivrMsgHook)
				if err != nil {
					return fmt.Errorf("error while triggering ivr flow: %w", err)
				}
				return nil
			}

			tb := triggers.NewBuilder(oa.Env(), flow.Reference(), flowContact).Msg(msgIn)
			if keyword != "" {
				tb = tb.WithMatch(&triggers.KeywordMatch{Type: trigger.KeywordMatchType(), Keyword: keyword})
			}

			// otherwise build the trigger and start the flow directly
			trigger := tb.Build()
			_, err = runner.StartFlowForContacts(ctx, rt, oa, flow, []*models.Contact{contact}, []flows.Trigger{trigger}, flowMsgHook, flow.FlowType().Interrupts(), models.NilStartID)
			if err != nil {
				return fmt.Errorf("error starting flow for contact: %w", err)
			}
			return nil
		}
	}

	// if there is a session, resume it
	if session != nil && flow != nil {
		resume := resumes.NewMsg(oa.Env(), flowContact, msgIn)
		_, err = runner.ResumeFlow(ctx, rt, oa, session, contact, resume, flowMsgHook)
		if err != nil {
			return fmt.Errorf("error resuming flow for contact: %w", err)
		}
		return nil
	}

	// this message didn't trigger and new sessions or resume any existing ones, so handle as inbox
	err = handleAsInbox(ctx, rt, oa, flowContact, msgIn, attachments, logUUIDs, ticket)
	if err != nil {
		return fmt.Errorf("error handling inbox message: %w", err)
	}
	return nil
}

// handles a message as an inbox message, i.e. no flow
func handleAsInbox(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, contact *flows.Contact, msg *flows.MsgIn, attachments []utils.Attachment, logUUIDs []clogs.LogUUID, ticket *models.Ticket) error {
	// usually last_seen_on is updated by handling the msg_received event in the engine sprint, but since this is an inbox
	// message we manually create that event and handle it
	msgEvent := events.NewMsgReceived(msg)
	contact.SetLastSeenOn(msgEvent.CreatedOn())
	contactEvents := map[*flows.Contact][]flows.Event{contact: {msgEvent}}

	err := models.HandleAndCommitEvents(ctx, rt, oa, models.NilUserID, contactEvents)
	if err != nil {
		return fmt.Errorf("error handling inbox message events: %w", err)
	}

	return markMsgHandled(ctx, rt.DB, msg, nil, attachments, ticket, logUUIDs)
}

// utility to mark as message as handled and update any open contact tickets
func markMsgHandled(ctx context.Context, db models.DBorTx, msg *flows.MsgIn, flow *models.Flow, attachments []utils.Attachment, ticket *models.Ticket, logUUIDs []clogs.LogUUID) error {
	flowID := models.NilFlowID
	if flow != nil {
		flowID = flow.ID()
	}
	ticketID := models.NilTicketID
	if ticket != nil {
		ticketID = ticket.ID()
	}

	err := models.MarkMessageHandled(ctx, db, models.MsgID(msg.ID()), models.MsgStatusHandled, models.VisibilityVisible, flowID, ticketID, attachments, logUUIDs)
	if err != nil {
		return fmt.Errorf("error marking message as handled: %w", err)
	}

	if ticket != nil {
		err = models.UpdateTicketLastActivity(ctx, db, []*models.Ticket{ticket})
		if err != nil {
			return fmt.Errorf("error updating last activity for open ticket: %w", err)
		}
	}

	return nil
}
