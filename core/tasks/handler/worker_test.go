package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/goflow/assets"
	"github.com/nyaruka/goflow/envs"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/utils"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/queue"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ensureWAConversationHandoverTable(t *testing.T, db *sqlx.DB) {
	t.Helper()
	db.MustExec(`
		CREATE TABLE IF NOT EXISTS wa_conversation_handover (
			id BIGSERIAL PRIMARY KEY,
			org_id INTEGER NOT NULL REFERENCES orgs_org(id),
			channel_id INTEGER NOT NULL REFERENCES channels_channel(id),
			contact_id INTEGER NOT NULL REFERENCES contacts_contact(id),
			contact_urn VARCHAR(255) NOT NULL,
			context_type VARCHAR(16) NOT NULL CHECK (context_type IN ('history', 'summary')),
			context_text TEXT NOT NULL,
			context_payload JSONB,
			previous_owner_app_id VARCHAR(64),
			previous_owner_app_role VARCHAR(64),
			previous_owner_business_id VARCHAR(64),
			handover_metadata VARCHAR(255),
			occurred_on TIMESTAMP WITH TIME ZONE NOT NULL,
			created_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			consumed_on TIMESTAMP WITH TIME ZONE,
			consumed_msg_id BIGINT
		)`)
	db.MustExec(`
		CREATE UNIQUE INDEX IF NOT EXISTS uq_wa_conv_handover_pending
		ON wa_conversation_handover (channel_id, contact_id) WHERE consumed_on IS NULL`)
}

func insertPendingHandover(t *testing.T, db *sqlx.DB, org *testdata.Org, channel *testdata.Channel, contact *testdata.Contact, contextType, contextText string) int64 {
	t.Helper()
	db.MustExec(`DELETE FROM wa_conversation_handover WHERE channel_id = $1 AND contact_id = $2`, channel.ID, contact.ID)
	var id int64
	err := db.Get(&id, `
		INSERT INTO wa_conversation_handover(org_id, channel_id, contact_id, contact_urn, context_type, context_text, occurred_on, created_on)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW()) RETURNING id`,
		org.ID, channel.ID, contact.ID, contact.URN.String(), contextType, contextText,
	)
	require.NoError(t, err)
	return id
}

func TestFormatRouterTextWithHandover(t *testing.T) {
	tests := []struct {
		name        string
		msgText     string
		contextText string
		expected    string
	}{
		{"summary with message", "oi", "Customer wants a refund", "oi; Context: Customer wants a refund"},
		{"history transcript with message", "oi", "[user] help\n[business] sure", "oi; Context: [user] help\n[business] sure"},
		{"empty message media only", "", "Prior chat summary", "Context: Prior chat summary"},
		{"whitespace message media only", "  \t ", "Prior chat summary", "Context: Prior chat summary"},
		{"empty context unchanged", "hello", "", "hello"},
		{"whitespace context unchanged", "hello", "  ", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatRouterTextWithHandover(tt.msgText, tt.contextText))
		})
	}
}

func TestConsumeHandoverContext(t *testing.T) {
	ctx, rt, db, _ := testsuite.Get()
	defer testsuite.Reset(testsuite.ResetAll)

	ensureWAConversationHandoverTable(t, db)

	channel := testdata.InsertChannel(db, testdata.Org1, "WA", "Handover WA", []string{"whatsapp"}, "SR", map[string]interface{}{})
	contact := testdata.InsertContact(db, testdata.Org1, flows.ContactUUID(uuids.New()), "Handover WA Contact", envs.Language("eng"))

	t.Run("no pending returns original text", func(t *testing.T) {
		event := &MsgEvent{ChannelID: channel.ID, ContactID: contact.ID, MsgID: flows.MsgID(1), Text: "oi"}
		routerText, err := consumeHandoverContext(ctx, rt.DB, event)
		require.NoError(t, err)
		assert.Equal(t, "oi", routerText)
	})

	t.Run("summary pending attaches and consumes", func(t *testing.T) {
		handoverID := insertPendingHandover(t, db, testdata.Org1, channel, contact, "summary", "AI summary here")
		event := &MsgEvent{ChannelID: channel.ID, ContactID: contact.ID, MsgID: flows.MsgID(42), Text: "oi"}
		routerText, err := consumeHandoverContext(ctx, rt.DB, event)
		require.NoError(t, err)
		assert.Equal(t, "oi; Context: AI summary here", routerText)
		testsuite.AssertQuery(t, db, `SELECT consumed_msg_id FROM wa_conversation_handover WHERE id = $1`, handoverID).
			Columns(map[string]interface{}{"consumed_msg_id": int64(42)})
	})

	t.Run("history pending attaches transcript", func(t *testing.T) {
		transcript := "[user] need help\n[business] what is your order?"
		insertPendingHandover(t, db, testdata.Org1, channel, contact, "history", transcript)
		event := &MsgEvent{ChannelID: channel.ID, ContactID: contact.ID, MsgID: flows.MsgID(43), Text: "oi"}
		routerText, err := consumeHandoverContext(ctx, rt.DB, event)
		require.NoError(t, err)
		assert.Equal(t, "oi; Context: "+transcript, routerText)
	})

	t.Run("empty text with pending", func(t *testing.T) {
		insertPendingHandover(t, db, testdata.Org1, channel, contact, "summary", "context only")
		event := &MsgEvent{ChannelID: channel.ID, ContactID: contact.ID, MsgID: flows.MsgID(44), Text: ""}
		routerText, err := consumeHandoverContext(ctx, rt.DB, event)
		require.NoError(t, err)
		assert.Equal(t, "Context: context only", routerText)
	})

	t.Run("second inbound does not reattach", func(t *testing.T) {
		insertPendingHandover(t, db, testdata.Org1, channel, contact, "summary", "once only")
		first := &MsgEvent{ChannelID: channel.ID, ContactID: contact.ID, MsgID: flows.MsgID(50), Text: "first"}
		second := &MsgEvent{ChannelID: channel.ID, ContactID: contact.ID, MsgID: flows.MsgID(51), Text: "second"}

		routerText, err := consumeHandoverContext(ctx, rt.DB, first)
		require.NoError(t, err)
		assert.Equal(t, "first; Context: once only", routerText)

		routerText, err = consumeHandoverContext(ctx, rt.DB, second)
		require.NoError(t, err)
		assert.Equal(t, "second", routerText)
	})

	t.Run("race already consumed does not attach", func(t *testing.T) {
		handoverID := insertPendingHandover(t, db, testdata.Org1, channel, contact, "summary", "taken")
		consumed, err := models.ConsumePendingWAConversationHandover(ctx, db, handoverID, flows.MsgID(99))
		require.NoError(t, err)
		require.True(t, consumed)

		event := &MsgEvent{ChannelID: channel.ID, ContactID: contact.ID, MsgID: flows.MsgID(100), Text: "late"}
		routerText, err := consumeHandoverContext(ctx, rt.DB, event)
		require.NoError(t, err)
		assert.Equal(t, "late", routerText)
	})
}

func TestBrainOnWithPendingHandover(t *testing.T) {
	ctx, rt, db, rp := testsuite.Get()
	rc := rp.Get()
	defer rc.Close()
	defer testsuite.Reset(testsuite.ResetAll)

	ensureWAConversationHandoverTable(t, db)

	db.MustExec(`CREATE TABLE IF NOT EXISTS internal_project (
		id SERIAL PRIMARY KEY,
		project_uuid UUID NOT NULL,
		org_ptr_id INTEGER NOT NULL
	)`)
	db.MustExec(`DELETE FROM internal_project WHERE org_ptr_id = $1`, testdata.Org1.ID)
	db.MustExec(`INSERT INTO internal_project (project_uuid, org_ptr_id) VALUES ($1, $2)`, uuids.New(), testdata.Org1.ID)
	db.MustExec(`UPDATE orgs_org SET brain_on = TRUE WHERE id = $1`, testdata.Org1.ID)

	channel := testdata.InsertChannel(db, testdata.Org1, "WA", "Brain Handover", []string{"whatsapp"}, "SR", map[string]interface{}{"version": 2})
	contact := testdata.InsertContact(db, testdata.Org1, flows.ContactUUID(uuids.New()), "Brain Handover Contact", envs.Language("eng"))
	urn := urns.URN("whatsapp:250700000099")
	urnID := testdata.InsertContactURN(db, testdata.Org1, contact, urn, 1000)
	insertPendingHandover(t, db, testdata.Org1, channel, contact, "summary", "Handover summary for brain")

	models.FlushCache()

	dbMsg := testdata.InsertIncomingMsg(db, testdata.Org1, channel, contact, "oi", models.MsgStatusPending)

	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	rt.Config.RouterBaseURL = server.URL
	rt.Config.RouterAuthToken = "router-token"

	event := &MsgEvent{
		ContactID: contact.ID,
		OrgID:     testdata.Org1.ID,
		ChannelID: channel.ID,
		MsgID:     dbMsg.ID(),
		MsgUUID:   dbMsg.UUID(),
		URN:       urn,
		URNID:     urnID,
		Text:      "oi",
	}
	eventJSON, err := json.Marshal(event)
	require.NoError(t, err)

	task := &queue.Task{Type: MsgEventType, OrgID: int(testdata.Org1.ID), Task: eventJSON}
	require.NoError(t, QueueHandleTask(rc, contact.ID, task))
	task, err = queue.PopNextTask(rc, queue.HandlerQueue)
	require.NoError(t, err)
	require.NoError(t, HandleEvent(ctx, rt, task))

	var payload struct {
		Text     string          `json:"text"`
		MsgEvent json.RawMessage `json:"msg_event"`
	}
	require.NoError(t, json.Unmarshal(capturedBody, &payload))
	assert.Equal(t, "oi; Context: Handover summary for brain", payload.Text)

	var msgEvent MsgEvent
	require.NoError(t, json.Unmarshal(payload.MsgEvent, &msgEvent))
	assert.Equal(t, "oi", msgEvent.Text)

	testsuite.AssertQuery(t, db, `SELECT count(*) FROM wa_conversation_handover WHERE channel_id = $1 AND contact_id = $2 AND consumed_on IS NOT NULL`, channel.ID, contact.ID).Returns(1)
	testsuite.AssertQuery(t, db, `SELECT consumed_msg_id FROM wa_conversation_handover WHERE channel_id = $1 AND contact_id = $2`, channel.ID, contact.ID).
		Columns(map[string]interface{}{"consumed_msg_id": int64(dbMsg.ID())})
}

func TestPendingHandoverNotConsumedWithOpenTicket(t *testing.T) {
	ctx, rt, db, rp := testsuite.Get()
	rc := rp.Get()
	defer rc.Close()
	defer testsuite.Reset(testsuite.ResetAll)

	ensureWAConversationHandoverTable(t, db)

	db.MustExec(`CREATE TABLE IF NOT EXISTS internal_project (
		id SERIAL PRIMARY KEY,
		project_uuid UUID NOT NULL,
		org_ptr_id INTEGER NOT NULL
	)`)
	db.MustExec(`DELETE FROM internal_project WHERE org_ptr_id = $1`, testdata.Org1.ID)
	db.MustExec(`INSERT INTO internal_project (project_uuid, org_ptr_id) VALUES ($1, $2)`, uuids.New(), testdata.Org1.ID)
	db.MustExec(`UPDATE orgs_org SET brain_on = TRUE WHERE id = $1`, testdata.Org1.ID)

	channel := testdata.InsertChannel(db, testdata.Org1, "WA", "Brain Ticket", []string{"whatsapp"}, "SR", map[string]interface{}{})
	handoverID := insertPendingHandover(t, db, testdata.Org1, channel, testdata.Cathy, "summary", "should stay pending")

	testdata.InsertOpenTicket(db, testdata.Org1, testdata.Cathy, testdata.Mailgun, testdata.DefaultTopic, "open", "", nil)
	models.FlushCache()

	dbMsg := testdata.InsertIncomingMsg(db, testdata.Org1, channel, testdata.Cathy, "with ticket", models.MsgStatusPending)

	event := &MsgEvent{
		ContactID: testdata.Cathy.ID,
		OrgID:     testdata.Org1.ID,
		ChannelID: channel.ID,
		MsgID:     dbMsg.ID(),
		MsgUUID:   dbMsg.UUID(),
		URN:       testdata.Cathy.URN,
		URNID:     testdata.Cathy.URNID,
		Text:      "with ticket",
	}
	eventJSON, err := json.Marshal(event)
	require.NoError(t, err)

	task := &queue.Task{Type: MsgEventType, OrgID: int(testdata.Org1.ID), Task: eventJSON}
	require.NoError(t, QueueHandleTask(rc, testdata.Cathy.ID, task))
	task, err = queue.PopNextTask(rc, queue.HandlerQueue)
	require.NoError(t, err)
	require.NoError(t, HandleEvent(ctx, rt, task))

	testsuite.AssertQuery(t, db, `SELECT consumed_on IS NULL FROM wa_conversation_handover WHERE id = $1`, handoverID).Columns(map[string]interface{}{"?column?": true})
}

func TestPendingHandoverNotConsumedWithWaitingSession(t *testing.T) {
	ctx, rt, db, rp := testsuite.Get()
	rc := rp.Get()
	defer rc.Close()
	defer testsuite.Reset(testsuite.ResetAll)

	ensureWAConversationHandoverTable(t, db)

	channel := testdata.InsertChannel(db, testdata.Org1, "WA", "Session Handover", []string{"whatsapp"}, "SR", map[string]interface{}{})
	handoverID := insertPendingHandover(t, db, testdata.Org1, channel, testdata.Cathy, "summary", "should stay pending")

	db.MustExec(`INSERT INTO flows_flowsession(uuid, org_id, contact_id, status, responded, created_on, session_type, current_flow_id)
		VALUES($1, $2, $3, 'W', FALSE, NOW(), 'M', $4)`, uuids.New(), testdata.Org1.ID, testdata.Cathy.ID, testdata.Favorites.ID)
	models.FlushCache()

	dbMsg := testdata.InsertIncomingMsg(db, testdata.Org1, channel, testdata.Cathy, "resume flow", models.MsgStatusPending)

	event := &MsgEvent{
		ContactID: testdata.Cathy.ID,
		OrgID:     testdata.Org1.ID,
		ChannelID: channel.ID,
		MsgID:     dbMsg.ID(),
		MsgUUID:   dbMsg.UUID(),
		URN:       testdata.Cathy.URN,
		URNID:     testdata.Cathy.URNID,
		Text:      "resume flow",
	}
	eventJSON, err := json.Marshal(event)
	require.NoError(t, err)

	task := &queue.Task{Type: MsgEventType, OrgID: int(testdata.Org1.ID), Task: eventJSON}
	require.NoError(t, QueueHandleTask(rc, testdata.Cathy.ID, task))
	task, err = queue.PopNextTask(rc, queue.HandlerQueue)
	require.NoError(t, err)
	require.NoError(t, HandleEvent(ctx, rt, task))

	testsuite.AssertQuery(t, db, `SELECT consumed_on IS NULL FROM wa_conversation_handover WHERE id = $1`, handoverID).Columns(map[string]interface{}{"?column?": true})
}

func TestBrainOnWithPendingHandoverMediaOnly(t *testing.T) {
	ctx, rt, db, rp := testsuite.Get()
	rc := rp.Get()
	defer rc.Close()
	defer testsuite.Reset(testsuite.ResetAll)

	ensureWAConversationHandoverTable(t, db)

	db.MustExec(`CREATE TABLE IF NOT EXISTS internal_project (
		id SERIAL PRIMARY KEY,
		project_uuid UUID NOT NULL,
		org_ptr_id INTEGER NOT NULL
	)`)
	projectUUID := uuids.New()
	db.MustExec(`DELETE FROM internal_project WHERE org_ptr_id = $1`, testdata.Org1.ID)
	db.MustExec(`INSERT INTO internal_project (project_uuid, org_ptr_id) VALUES ($1, $2)`, projectUUID, testdata.Org1.ID)
	db.MustExec(`UPDATE orgs_org SET brain_on = TRUE WHERE id = $1`, testdata.Org1.ID)

	channel := testdata.InsertChannel(db, testdata.Org1, "WA", "Brain Media", []string{"whatsapp"}, "SR", map[string]interface{}{"version": 2})
	contact := testdata.InsertContact(db, testdata.Org1, flows.ContactUUID(uuids.New()), "Brain Media Contact", envs.Language("eng"))
	urn := urns.URN("whatsapp:250700000088")
	urnID := testdata.InsertContactURN(db, testdata.Org1, contact, urn, 1000)
	insertPendingHandover(t, db, testdata.Org1, channel, contact, "summary", "Image context")

	models.FlushCache()

	dbMsg := testdata.InsertIncomingMsg(db, testdata.Org1, channel, contact, "", models.MsgStatusPending)

	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	rt.Config.RouterBaseURL = server.URL
	rt.Config.RouterAuthToken = "router-token"

	oa, err := models.GetOrgAssets(ctx, rt, testdata.Org1.ID)
	require.NoError(t, err)
	modelContact, err := models.LoadContact(ctx, db, oa, contact.ID)
	require.NoError(t, err)
	flowContact, err := modelContact.FlowContact(oa)
	require.NoError(t, err)
	channelModel := oa.ChannelByID(channel.ID)
	require.NotNil(t, channelModel)

	event := &MsgEvent{
		ContactID:   contact.ID,
		OrgID:       testdata.Org1.ID,
		ChannelID:   channel.ID,
		MsgID:       dbMsg.ID(),
		MsgUUID:     dbMsg.UUID(),
		URN:         urn,
		URNID:       urnID,
		Text:        "",
		Attachments: []utils.Attachment{"image/jpeg:https://example.com/photo.jpg"},
	}

	routerText, err := consumeHandoverContext(ctx, rt.DB, event)
	require.NoError(t, err)
	assert.Equal(t, "Context: Image context", routerText)

	require.NoError(t, requestToRouter(event, rt.Config, flowContact, projectUUID, channelModel, routerText))

	var payload struct {
		Text        string             `json:"text"`
		Attachments []utils.Attachment `json:"attachments"`
	}
	require.NoError(t, json.Unmarshal(capturedBody, &payload))
	assert.Equal(t, "Context: Image context", payload.Text)
	assert.Len(t, payload.Attachments, 1)
}

func TestParseMsgInMetadata(t *testing.T) {
	newMsgIn := func() *flows.MsgIn {
		return flows.NewMsgIn(flows.MsgUUID(uuids.New()), urns.URN("tel:+1234567890"), nil, "", nil)
	}

	tests := []struct {
		name         string
		metadata     json.RawMessage
		isIGComment  bool
		hasOrder     bool
		hasNFMReply  bool
		hasIGComment bool
	}{
		{
			name:     "nil metadata",
			metadata: nil,
		},
		{
			name:     "metadata without known keys",
			metadata: json.RawMessage(`{"foo": "bar"}`),
		},
		{
			name:     "metadata with order",
			metadata: json.RawMessage(`{"order": {"catalog_id": "cat1", "text": "buy now"}}`),
			hasOrder: true,
		},
		{
			name:        "metadata with nfm_reply",
			metadata:    json.RawMessage(`{"nfm_reply": {"name": "test", "response_json": {}}}`),
			hasNFMReply: true,
		},
		{
			name:         "metadata with ig_comment",
			metadata:     json.RawMessage(`{"ig_comment": {"text": "cool!", "id": "123"}}`),
			isIGComment:  true,
			hasIGComment: true,
		},
		{
			name:         "metadata with all keys",
			metadata:     json.RawMessage(`{"order": {"catalog_id": "cat1"}, "nfm_reply": {"name": "test"}, "ig_comment": {"text": "cool!"}}`),
			isIGComment:  true,
			hasOrder:     true,
			hasNFMReply:  true,
			hasIGComment: true,
		},
		{
			name:     "malformed top-level JSON",
			metadata: json.RawMessage(`not json`),
		},
		{
			// ig_comment is a plain string; Unmarshal into *IGComment fails, so isIGComment stays false
			name:        "ig_comment with non-object value",
			metadata:    json.RawMessage(`{"ig_comment": "not an object"}`),
			isIGComment: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &MsgEvent{Metadata: tt.metadata}
			msgIn := newMsgIn()

			isIGComment := parseMsgInMetadata(event, msgIn)

			assert.Equal(t, tt.isIGComment, isIGComment)

			if tt.hasOrder {
				assert.NotNil(t, msgIn.Order())
			} else {
				assert.Nil(t, msgIn.Order())
			}
			if tt.hasNFMReply {
				assert.NotNil(t, msgIn.NFMReply())
			} else {
				assert.Nil(t, msgIn.NFMReply())
			}
			if tt.hasIGComment {
				assert.NotNil(t, msgIn.IGComment())
			} else {
				assert.Nil(t, msgIn.IGComment())
			}
		})
	}
}

func TestShouldFireTrigger(t *testing.T) {
	ctx, rt, db, _ := testsuite.Get()
	defer testsuite.Reset(testsuite.ResetAll)

	testdata.InsertKeywordTrigger(db, testdata.Org1, testdata.Favorites, "start", models.MatchOnly, nil, nil)
	testdata.InsertCatchallTrigger(db, testdata.Org1, testdata.SingleMessage, nil, nil)

	// configure IVRFlow to ignore triggers so we can test that path
	db.MustExec(`UPDATE flows_flow SET ignore_triggers = TRUE WHERE id = $1`, testdata.IVRFlow.ID)

	models.FlushCache()

	oa, err := models.GetOrgAssets(ctx, rt, testdata.Org1.ID)
	require.NoError(t, err)

	var keywordTrigger, catchallTrigger *models.Trigger
	for _, tr := range oa.Triggers() {
		switch tr.TriggerType() {
		case models.KeywordTriggerType:
			keywordTrigger = tr
		case models.CatchallTriggerType:
			catchallTrigger = tr
		}
	}
	require.NotNil(t, keywordTrigger, "expected a keyword trigger")
	require.NotNil(t, catchallTrigger, "expected a catchall trigger")

	activeFlow, err := oa.FlowByID(testdata.Favorites.ID)
	require.NoError(t, err)

	ignoringFlow, err := oa.FlowByID(testdata.IVRFlow.ID)
	require.NoError(t, err)
	require.True(t, ignoringFlow.IgnoreTriggers(), "IVRFlow should have ignore_triggers=true after DB update")

	tests := []struct {
		description string
		trigger     *models.Trigger
		flow        *models.Flow
		isBrain     bool
		expected    bool
	}{
		{"nil trigger always returns false", nil, nil, false, false},
		{"brain active suppresses any trigger", keywordTrigger, nil, true, false},
		{"keyword trigger with no active session flow", keywordTrigger, nil, false, true},
		{"keyword trigger, active flow not ignoring triggers", keywordTrigger, activeFlow, false, true},
		{"keyword trigger, active flow ignoring triggers", keywordTrigger, ignoringFlow, false, false},
		{"catchall trigger with no active session flow", catchallTrigger, nil, false, true},
		{"catchall trigger does not interrupt an active session", catchallTrigger, activeFlow, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			assert.Equal(t, tt.expected, shouldFireTrigger(tt.trigger, tt.flow, tt.isBrain))
		})
	}
}

func TestRequestToRouter(t *testing.T) {
	ctx, rt, db, _ := testsuite.Get()
	defer testsuite.Reset(testsuite.ResetAll)

	channel := testdata.InsertChannel(db, testdata.Org1, "TW", "Router Channel", []string{"tel"}, "SR", map[string]interface{}{"version": "2"})
	contact := testdata.InsertContact(db, testdata.Org1, flows.ContactUUID(uuids.New()), "Router Contact", envs.Language("eng"))
	urn := urns.URN("tel:+250700000010")
	urnID := testdata.InsertContactURN(db, testdata.Org1, contact, urn, 1000)

	models.FlushCache()

	oa, err := models.GetOrgAssets(ctx, rt, testdata.Org1.ID)
	require.NoError(t, err)

	modelContact, err := models.LoadContact(ctx, db, oa, contact.ID)
	require.NoError(t, err)
	require.NotNil(t, modelContact)

	flowContact, err := modelContact.FlowContact(oa)
	require.NoError(t, err)

	channelModel := oa.ChannelByID(channel.ID)
	require.NotNil(t, channelModel)

	type capturedRequest struct {
		method string
		path   string
		token  string
		body   []byte
	}

	reqCh := make(chan capturedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		reqCh <- capturedRequest{
			method: r.Method,
			path:   r.URL.Path,
			token:  r.URL.Query().Get("token"),
			body:   body,
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rt.Config.RouterBaseURL = server.URL
	rt.Config.RouterAuthToken = "router-token"

	metadata := json.RawMessage(`{"foo":"bar"}`)
	event := &MsgEvent{
		ContactID: contact.ID,
		OrgID:     testdata.Org1.ID,
		ChannelID: channel.ID,
		MsgID:     flows.MsgID(123),
		MsgUUID:   flows.MsgUUID(uuids.New()),
		URN:       urn,
		URNID:     urnID,
		Text:      "hello router",
		Metadata:  metadata,
	}

	projectUUID := uuids.New()
	err = requestToRouter(event, rt.Config, flowContact, projectUUID, channelModel, event.Text)
	require.NoError(t, err)

	captured := <-reqCh
	assert.Equal(t, http.MethodPost, captured.method)
	assert.Equal(t, "/messages", captured.path)
	assert.Equal(t, "router-token", captured.token)

	var payload struct {
		ProjectUUID   string                 `json:"project_uuid"`
		ContactURN    string                 `json:"contact_urn"`
		Text          string                 `json:"text"`
		Metadata      json.RawMessage        `json:"metadata"`
		MsgEvent      json.RawMessage        `json:"msg_event"`
		ContactFields map[string]interface{} `json:"contact_fields"`
		ChannelUUID   string                 `json:"channel_uuid"`
		ChannelType   string                 `json:"channel_type"`
		ContactName   string                 `json:"contact_name"`
		StreamSupport bool                   `json:"stream_support"`
	}
	require.NoError(t, json.Unmarshal(captured.body, &payload))

	assert.Equal(t, string(projectUUID), payload.ProjectUUID)
	assert.Equal(t, string(event.URN.Identity()), payload.ContactURN)
	assert.Equal(t, event.Text, payload.Text)
	assert.Equal(t, string(channelModel.UUID()), payload.ChannelUUID)
	assert.Equal(t, string(channelModel.Type()), payload.ChannelType)
	assert.Equal(t, flowContact.Name(), payload.ContactName)
	assert.True(t, payload.StreamSupport)
	require.NotNil(t, payload.ContactFields)
	assert.Contains(t, payload.ContactFields, "age")
	assert.Contains(t, payload.ContactFields, "district")
	assert.Contains(t, payload.ContactFields, "gender")
	assert.Contains(t, payload.ContactFields, "joined")
	assert.Contains(t, payload.ContactFields, "state")
	assert.Contains(t, payload.ContactFields, "ward")
	assert.Nil(t, payload.ContactFields["age"])
	assert.Nil(t, payload.ContactFields["district"])
	assert.Nil(t, payload.ContactFields["gender"])
	assert.Nil(t, payload.ContactFields["joined"])
	assert.Nil(t, payload.ContactFields["state"])
	assert.Nil(t, payload.ContactFields["ward"])
	assert.JSONEq(t, string(metadata), string(payload.Metadata))

	var msgEvent MsgEvent
	require.NoError(t, json.Unmarshal(payload.MsgEvent, &msgEvent))
	assert.Equal(t, event.ContactID, msgEvent.ContactID)
	assert.Equal(t, event.OrgID, msgEvent.OrgID)
	assert.Equal(t, event.ChannelID, msgEvent.ChannelID)
	assert.Equal(t, event.MsgID, msgEvent.MsgID)
	assert.Equal(t, event.MsgUUID, msgEvent.MsgUUID)
	assert.Equal(t, event.URN, msgEvent.URN)
	assert.Equal(t, event.URNID, msgEvent.URNID)
	assert.Equal(t, event.Text, msgEvent.Text)
	assert.JSONEq(t, string(metadata), string(msgEvent.Metadata))
}

func TestRequestToRouterStreamSupportByVersion(t *testing.T) {
	ctx, rt, db, _ := testsuite.Get()
	defer testsuite.Reset(testsuite.ResetAll)

	contact := testdata.InsertContact(db, testdata.Org1, flows.ContactUUID(uuids.New()), "Stream Contact", envs.Language("eng"))
	urn := urns.URN("tel:+250700000020")
	urnID := testdata.InsertContactURN(db, testdata.Org1, contact, urn, 1000)

	reqCh := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		reqCh <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rt.Config.RouterBaseURL = server.URL
	rt.Config.RouterAuthToken = "router-token"

	tests := []struct {
		name          string
		config        map[string]interface{}
		streamSupport bool
	}{
		{"version 0 as int disables stream support", map[string]interface{}{"version": 0}, false},
		{"version 1 as int disables stream support", map[string]interface{}{"version": 1}, false},
		{"version 2 as int enables stream support", map[string]interface{}{"version": 2}, true},
		{"version 3 as int enables stream support", map[string]interface{}{"version": 3}, true},
		{"version 10 as int enables stream support", map[string]interface{}{"version": 10}, true},
		{"version 1 as string disables stream support", map[string]interface{}{"version": "1"}, false},
		{"version 2 as string enables stream support", map[string]interface{}{"version": "2"}, true},
		{"version 3 as string enables stream support", map[string]interface{}{"version": "3"}, true},
		{"version 10 as string enables stream support", map[string]interface{}{"version": "10"}, true},
		{"missing version disables stream support", map[string]interface{}{}, false},
		{"non-numeric version disables stream support", map[string]interface{}{"version": "abc"}, false},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := testdata.InsertChannel(
				db, testdata.Org1, "TW",
				fmt.Sprintf("Router Channel %d", i),
				[]string{"tel"}, "SR",
				tt.config,
			)

			models.FlushCache()

			oa, err := models.GetOrgAssets(ctx, rt, testdata.Org1.ID)
			require.NoError(t, err)

			channelModel := oa.ChannelByID(channel.ID)
			require.NotNil(t, channelModel)

			modelContact, err := models.LoadContact(ctx, db, oa, contact.ID)
			require.NoError(t, err)

			flowContact, err := modelContact.FlowContact(oa)
			require.NoError(t, err)

			event := &MsgEvent{
				ContactID: contact.ID,
				OrgID:     testdata.Org1.ID,
				ChannelID: channel.ID,
				MsgID:     flows.MsgID(123),
				MsgUUID:   flows.MsgUUID(uuids.New()),
				URN:       urn,
				URNID:     urnID,
				Text:      "hello router",
			}

			err = requestToRouter(event, rt.Config, flowContact, uuids.New(), channelModel, event.Text)
			require.NoError(t, err)

			body := <-reqCh

			var payload struct {
				StreamSupport bool `json:"stream_support"`
			}
			require.NoError(t, json.Unmarshal(body, &payload))
			assert.Equal(t, tt.streamSupport, payload.StreamSupport)
		})
	}
}

func TestApplyContactFieldModifiers(t *testing.T) {
	ctx, rt, db, _ := testsuite.Get()
	defer testsuite.Reset(testsuite.ResetAll)

	oa, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshAll)
	require.NoError(t, err)

	_, flowContact := testdata.Cathy.Load(db, oa)

	msg := testdata.InsertIncomingMsg(db, testdata.Org1, testdata.TwilioChannel, testdata.Cathy, "hello", models.MsgStatusPending)

	t.Run("existing field applies normally", func(t *testing.T) {
		event := &MsgEvent{
			ContactID:        testdata.Cathy.ID,
			OrgID:            testdata.Org1.ID,
			MsgID:            msg.ID(),
			MsgUUID:          flows.MsgUUID(uuids.New()),
			NewContactFields: map[string]string{"gender": "Female"},
		}

		modelContact, updatedContact, recalcGroups, err := applyContactFieldModifiers(ctx, rt, oa, event, flowContact, models.NilTopupID)
		require.NoError(t, err)
		require.NotNil(t, modelContact)
		require.NotNil(t, updatedContact)
		assert.True(t, recalcGroups)
	})

	t.Run("unknown non-whitelisted field is skipped", func(t *testing.T) {
		event := &MsgEvent{
			ContactID:        testdata.Cathy.ID,
			OrgID:            testdata.Org1.ID,
			MsgID:            msg.ID(),
			MsgUUID:          flows.MsgUUID(uuids.New()),
			NewContactFields: map[string]string{"nonexistent_field": "some value"},
		}

		modelContact, updatedContact, recalcGroups, err := applyContactFieldModifiers(ctx, rt, oa, event, flowContact, models.NilTopupID)
		require.NoError(t, err)
		require.NotNil(t, modelContact)
		require.NotNil(t, updatedContact)
		assert.False(t, recalcGroups, "no modifiers applied, so no group recalc needed")
	})

	t.Run("segment field is auto-created and applied", func(t *testing.T) {
		require.Nil(t, oa.SessionAssets().Fields().Get("segment"), "segment field should not exist yet")

		event := &MsgEvent{
			ContactID:        testdata.Cathy.ID,
			OrgID:            testdata.Org1.ID,
			MsgID:            msg.ID(),
			MsgUUID:          flows.MsgUUID(uuids.New()),
			NewContactFields: map[string]string{"segment": "premium"},
		}

		modelContact, updatedContact, recalcGroups, err := applyContactFieldModifiers(ctx, rt, oa, event, flowContact, models.NilTopupID)
		require.NoError(t, err)
		require.NotNil(t, modelContact)
		require.NotNil(t, updatedContact)
		assert.True(t, recalcGroups)

		testsuite.AssertQuery(t, db,
			`SELECT count(*) FROM contacts_contactfield WHERE org_id = $1 AND key = 'segment' AND is_active = TRUE AND value_type = 'T'`,
			testdata.Org1.ID,
		).Returns(1)

		refreshedOA, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshFields)
		require.NoError(t, err)
		field := refreshedOA.FieldByKey("segment")
		require.NotNil(t, field)
		assert.Equal(t, "Segment", field.Name())
		assert.Equal(t, assets.FieldTypeText, field.Type())
	})

	t.Run("orderform field is auto-created and applied", func(t *testing.T) {
		require.Nil(t, oa.SessionAssets().Fields().Get("orderform"), "orderform field should not exist yet")

		event := &MsgEvent{
			ContactID:        testdata.Cathy.ID,
			OrgID:            testdata.Org1.ID,
			MsgID:            msg.ID(),
			MsgUUID:          flows.MsgUUID(uuids.New()),
			NewContactFields: map[string]string{"orderform": "order-123"},
		}

		modelContact, updatedContact, recalcGroups, err := applyContactFieldModifiers(ctx, rt, oa, event, flowContact, models.NilTopupID)
		require.NoError(t, err)
		require.NotNil(t, modelContact)
		require.NotNil(t, updatedContact)
		assert.True(t, recalcGroups)

		testsuite.AssertQuery(t, db,
			`SELECT count(*) FROM contacts_contactfield WHERE org_id = $1 AND key = 'orderform' AND is_active = TRUE AND value_type = 'T'`,
			testdata.Org1.ID,
		).Returns(1)

		refreshedOA, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshFields)
		require.NoError(t, err)
		field := refreshedOA.FieldByKey("orderform")
		require.NotNil(t, field)
		assert.Equal(t, "Orderform", field.Name())
		assert.Equal(t, assets.FieldTypeText, field.Type())
	})

	t.Run("email field is auto-created and applied", func(t *testing.T) {
		require.Nil(t, oa.SessionAssets().Fields().Get("email"), "email field should not exist yet")

		event := &MsgEvent{
			ContactID:        testdata.Cathy.ID,
			OrgID:            testdata.Org1.ID,
			MsgID:            msg.ID(),
			MsgUUID:          flows.MsgUUID(uuids.New()),
			NewContactFields: map[string]string{"email": "test@example.com"},
		}

		modelContact, updatedContact, recalcGroups, err := applyContactFieldModifiers(ctx, rt, oa, event, flowContact, models.NilTopupID)
		require.NoError(t, err)
		require.NotNil(t, modelContact)
		require.NotNil(t, updatedContact)
		assert.True(t, recalcGroups)

		testsuite.AssertQuery(t, db,
			`SELECT count(*) FROM contacts_contactfield WHERE org_id = $1 AND key = 'email' AND is_active = TRUE AND value_type = 'T'`,
			testdata.Org1.ID,
		).Returns(1)

		refreshedOA, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshFields)
		require.NoError(t, err)
		field := refreshedOA.FieldByKey("email")
		require.NotNil(t, field)
		assert.Equal(t, "Email", field.Name())
		assert.Equal(t, assets.FieldTypeText, field.Type())
	})

	t.Run("session field is auto-created and applied", func(t *testing.T) {
		require.Nil(t, oa.SessionAssets().Fields().Get("session"), "session field should not exist yet")

		event := &MsgEvent{
			ContactID:        testdata.Cathy.ID,
			OrgID:            testdata.Org1.ID,
			MsgID:            msg.ID(),
			MsgUUID:          flows.MsgUUID(uuids.New()),
			NewContactFields: map[string]string{"session": "abc123"},
		}

		modelContact, updatedContact, recalcGroups, err := applyContactFieldModifiers(ctx, rt, oa, event, flowContact, models.NilTopupID)
		require.NoError(t, err)
		require.NotNil(t, modelContact)
		require.NotNil(t, updatedContact)
		assert.True(t, recalcGroups)

		testsuite.AssertQuery(t, db,
			`SELECT count(*) FROM contacts_contactfield WHERE org_id = $1 AND key = 'session' AND is_active = TRUE AND value_type = 'T'`,
			testdata.Org1.ID,
		).Returns(1)

		refreshedOA, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshFields)
		require.NoError(t, err)
		field := refreshedOA.FieldByKey("session")
		require.NotNil(t, field)
		assert.Equal(t, "Session", field.Name())
		assert.Equal(t, assets.FieldTypeText, field.Type())
	})

	t.Run("vtex_account field is auto-created and applied", func(t *testing.T) {
		require.Nil(t, oa.SessionAssets().Fields().Get("vtex_account"), "vtex_account field should not exist yet")

		event := &MsgEvent{
			ContactID:        testdata.Cathy.ID,
			OrgID:            testdata.Org1.ID,
			MsgID:            msg.ID(),
			MsgUUID:          flows.MsgUUID(uuids.New()),
			NewContactFields: map[string]string{"vtex_account": "mystore"},
		}

		modelContact, updatedContact, recalcGroups, err := applyContactFieldModifiers(ctx, rt, oa, event, flowContact, models.NilTopupID)
		require.NoError(t, err)
		require.NotNil(t, modelContact)
		require.NotNil(t, updatedContact)
		assert.True(t, recalcGroups)

		testsuite.AssertQuery(t, db,
			`SELECT count(*) FROM contacts_contactfield WHERE org_id = $1 AND key = 'vtex_account' AND is_active = TRUE AND value_type = 'T'`,
			testdata.Org1.ID,
		).Returns(1)

		refreshedOA, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshFields)
		require.NoError(t, err)
		field := refreshedOA.FieldByKey("vtex_account")
		require.NotNil(t, field)
		assert.Equal(t, "Vtex account", field.Name())
		assert.Equal(t, assets.FieldTypeText, field.Type())
	})

	t.Run("marketing_opt_in field is auto-created and applied", func(t *testing.T) {
		require.Nil(t, oa.SessionAssets().Fields().Get("marketing_opt_in"), "marketing_opt_in field should not exist yet")

		event := &MsgEvent{
			ContactID:        testdata.Cathy.ID,
			OrgID:            testdata.Org1.ID,
			MsgID:            msg.ID(),
			MsgUUID:          flows.MsgUUID(uuids.New()),
			NewContactFields: map[string]string{"marketing_opt_in": "false"},
		}

		modelContact, updatedContact, recalcGroups, err := applyContactFieldModifiers(ctx, rt, oa, event, flowContact, models.NilTopupID)
		require.NoError(t, err)
		require.NotNil(t, modelContact)
		require.NotNil(t, updatedContact)
		assert.True(t, recalcGroups)

		testsuite.AssertQuery(t, db,
			`SELECT count(*) FROM contacts_contactfield WHERE org_id = $1 AND key = 'marketing_opt_in' AND is_active = TRUE AND value_type = 'T'`,
			testdata.Org1.ID,
		).Returns(1)

		refreshedOA, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshFields)
		require.NoError(t, err)
		field := refreshedOA.FieldByKey("marketing_opt_in")
		require.NotNil(t, field)
		assert.Equal(t, "Marketing opt-in", field.Name())
		assert.Equal(t, assets.FieldTypeText, field.Type())
	})


	fields := []struct {
		key   string
		value string
		label string
	}{
		{"whatsapp_username", "johndoe", "WhatsApp username"},
		{"instagram_username", "janedoe", "Instagram username"},
		{"ctwa_clid", "clid-abc123", "CTWA CLID"},
	}

	for _, f := range fields {
		f := f
		t.Run(f.key+" field is auto-created and applied", func(t *testing.T) {
			require.Nil(t, oa.SessionAssets().Fields().Get(f.key))

			event := &MsgEvent{
				ContactID:        testdata.Cathy.ID,
				OrgID:            testdata.Org1.ID,
				MsgID:            msg.ID(),
				MsgUUID:          flows.MsgUUID(uuids.New()),
				NewContactFields: map[string]string{f.key: f.value},
			}

			modelContact, updatedContact, recalcGroups, err := applyContactFieldModifiers(ctx, rt, oa, event, flowContact, models.NilTopupID)
			require.NoError(t, err)
			require.NotNil(t, modelContact)
			require.NotNil(t, updatedContact)
			assert.True(t, recalcGroups)

			testsuite.AssertQuery(t, db,
				`SELECT count(*) FROM contacts_contactfield WHERE org_id = $1 AND key = $2 AND is_active = TRUE AND value_type = 'T'`,
				testdata.Org1.ID, f.key,
			).Returns(1)

			refreshedOA, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshFields)
			require.NoError(t, err)
			field := refreshedOA.FieldByKey(f.key)
			require.NotNil(t, field)
			assert.Equal(t, f.label, field.Name())
			assert.Equal(t, assets.FieldTypeText, field.Type())
		})
	}


	t.Run("both whitelisted fields in one event", func(t *testing.T) {
		db.MustExec(`DELETE FROM contacts_contactfield WHERE org_id = $1 AND key IN ('segment', 'orderform', 'email', 'session', 'vtex_account')`, testdata.Org1.ID)
		models.FlushCache()

		freshOA, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshAll)
		require.NoError(t, err)

		_, freshContact := testdata.Cathy.Load(db, freshOA)

		event := &MsgEvent{
			ContactID:        testdata.Cathy.ID,
			OrgID:            testdata.Org1.ID,
			MsgID:            msg.ID(),
			MsgUUID:          flows.MsgUUID(uuids.New()),
			NewContactFields: map[string]string{"segment": "vip", "orderform": "order-456", "email": "cathy@example.com", "session": "sess-789", "vtex_account": "mystore"},
		}

		modelContact, updatedContact, recalcGroups, err := applyContactFieldModifiers(ctx, rt, freshOA, event, freshContact, models.NilTopupID)
		require.NoError(t, err)
		require.NotNil(t, modelContact)
		require.NotNil(t, updatedContact)
		assert.True(t, recalcGroups)

		testsuite.AssertQuery(t, db,
			`SELECT count(*) FROM contacts_contactfield WHERE org_id = $1 AND key IN ('segment', 'orderform', 'email', 'session', 'vtex_account') AND is_active = TRUE`,
			testdata.Org1.ID,
		).Returns(5)
	})

	t.Run("mix of existing, whitelisted, and unknown fields", func(t *testing.T) {
		db.MustExec(`DELETE FROM contacts_contactfield WHERE org_id = $1 AND key IN ('segment', 'orderform', 'email', 'session', 'vtex_account')`, testdata.Org1.ID)
		models.FlushCache()

		freshOA, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshAll)
		require.NoError(t, err)

		_, freshContact := testdata.Cathy.Load(db, freshOA)

		event := &MsgEvent{
			ContactID: testdata.Cathy.ID,
			OrgID:     testdata.Org1.ID,
			MsgID:     msg.ID(),
			MsgUUID:   flows.MsgUUID(uuids.New()),
			NewContactFields: map[string]string{
				"gender":      "Male",
				"segment":     "enterprise",
				"nonexistent": "ignored",
			},
		}

		modelContact, updatedContact, recalcGroups, err := applyContactFieldModifiers(ctx, rt, freshOA, event, freshContact, models.NilTopupID)
		require.NoError(t, err)
		require.NotNil(t, modelContact)
		require.NotNil(t, updatedContact)
		assert.True(t, recalcGroups, "gender and segment should produce modifiers")

		testsuite.AssertQuery(t, db,
			`SELECT count(*) FROM contacts_contactfield WHERE org_id = $1 AND key = 'segment' AND is_active = TRUE`,
			testdata.Org1.ID,
		).Returns(1)

		testsuite.AssertQuery(t, db,
			`SELECT count(*) FROM contacts_contactfield WHERE org_id = $1 AND key = 'nonexistent' AND is_active = TRUE`,
			testdata.Org1.ID,
		).Returns(0)
	})
}
