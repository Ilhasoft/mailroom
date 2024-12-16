package msgs_test

import (
	"testing"
	"time"

	"github.com/nyaruka/gocommon/dbutil/assertdb"
	"github.com/nyaruka/gocommon/i18n"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/goflow/assets"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/events"
	_ "github.com/nyaruka/mailroom/core/handlers"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/core/tasks/msgs"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/nyaruka/mailroom/utils/queues"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendBroadcastTask(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.Reset(testsuite.ResetAll)

	rc := rt.RP.Get()
	defer rc.Close()

	oa, err := models.GetOrgAssets(ctx, rt, testdata.Org1.ID)
	require.NoError(t, err)

	eng := i18n.Language("eng")
	basic := flows.BroadcastTranslations{
		eng: {
			Text:         "hello world",
			Attachments:  nil,
			QuickReplies: nil,
		},
	}

	doctors := assets.NewGroupReference(testdata.DoctorsGroup.UUID, "Doctors")
	cathy := flows.NewContactReference(testdata.Cathy.UUID, "Cathy")

	// add an extra URN fo cathy
	testdata.InsertContactURN(rt, testdata.Org1, testdata.Cathy, urns.URN("tel:+12065551212"), 1001, nil)

	// change george's URN to an invalid twitter URN so it can't be sent
	rt.DB.MustExec(`UPDATE contacts_contacturn SET identity = 'twitter:invalid-urn', scheme = 'twitter', path='invalid-urn' WHERE id = $1`, testdata.George.URNID)
	george := flows.NewContactReference(testdata.George.UUID, "George")
	georgeOnly := []*flows.ContactReference{george}

	tcs := []struct {
		translations       flows.BroadcastTranslations
		baseLanguage       i18n.Language
		groups             []*assets.GroupReference
		contacts           []*flows.ContactReference
		urns               []urns.URN
		queue              *queues.FairSorted
		expectedBatchCount int
		expectedMsgCount   int
		expectedMsgText    string
	}{
		{ // 0
			translations:       basic,
			baseLanguage:       eng,
			groups:             []*assets.GroupReference{doctors},
			contacts:           nil,
			urns:               nil,
			queue:              tasks.BatchQueue,
			expectedBatchCount: 2,
			expectedMsgCount:   121,
			expectedMsgText:    "hello world",
		},
		{ // 1
			translations:       basic,
			baseLanguage:       eng,
			groups:             []*assets.GroupReference{doctors},
			contacts:           georgeOnly,
			urns:               nil,
			queue:              tasks.BatchQueue,
			expectedBatchCount: 2,
			expectedMsgCount:   122,
			expectedMsgText:    "hello world",
		},
		{ // 2
			translations:       basic,
			baseLanguage:       eng,
			groups:             nil,
			contacts:           georgeOnly,
			urns:               nil,
			queue:              tasks.HandlerQueue,
			expectedBatchCount: 1,
			expectedMsgCount:   1,
			expectedMsgText:    "hello world",
		},
		{ // 3
			translations:       basic,
			baseLanguage:       eng,
			groups:             []*assets.GroupReference{doctors},
			contacts:           []*flows.ContactReference{cathy},
			urns:               nil,
			queue:              tasks.BatchQueue,
			expectedBatchCount: 2,
			expectedMsgCount:   121,
			expectedMsgText:    "hello world",
		},
		{ // 4
			translations:       basic,
			baseLanguage:       eng,
			groups:             nil,
			contacts:           []*flows.ContactReference{cathy},
			urns:               nil,
			queue:              tasks.HandlerQueue,
			expectedBatchCount: 1,
			expectedMsgCount:   1,
			expectedMsgText:    "hello world",
		},
		{ // 5
			translations:       basic,
			baseLanguage:       eng,
			groups:             nil,
			contacts:           []*flows.ContactReference{cathy},
			urns:               []urns.URN{urns.URN("tel:+12065551212")},
			queue:              tasks.HandlerQueue,
			expectedBatchCount: 1,
			expectedMsgCount:   1,
			expectedMsgText:    "hello world",
		},
		{ // 6
			translations:       basic,
			baseLanguage:       eng,
			groups:             nil,
			contacts:           []*flows.ContactReference{cathy},
			urns:               []urns.URN{urns.URN("tel:+250700000001")},
			queue:              tasks.HandlerQueue,
			expectedBatchCount: 1,
			expectedMsgCount:   2,
			expectedMsgText:    "hello world",
		},
		{ // 7
			translations:       basic,
			baseLanguage:       eng,
			groups:             nil,
			contacts:           nil,
			urns:               []urns.URN{urns.URN("tel:+250700000001")},
			queue:              tasks.HandlerQueue,
			expectedBatchCount: 1,
			expectedMsgCount:   1,
			expectedMsgText:    "hello world",
		},
	}

	lastNow := time.Now()
	time.Sleep(10 * time.Millisecond)

	for i, tc := range tcs {
		testsuite.ReindexElastic(ctx)

		// handle our start task
		event := events.NewBroadcastCreated(tc.translations, tc.baseLanguage, tc.groups, tc.contacts, "", tc.urns)
		bcast, err := models.NewBroadcastFromEvent(ctx, rt.DB, oa, event)
		assert.NoError(t, err)

		err = tasks.Queue(rc, tc.queue, testdata.Org1.ID, &msgs.SendBroadcastTask{Broadcast: bcast}, queues.DefaultPriority)
		assert.NoError(t, err)

		taskCounts := testsuite.FlushTasks(t, rt)

		// assert our count of batches
		assert.Equal(t, tc.expectedBatchCount, taskCounts["send_broadcast_batch"], "%d: unexpected batch count", i)

		// assert our count of total msgs created
		assertdb.Query(t, rt.DB, `SELECT count(*) FROM msgs_msg WHERE org_id = 1 AND created_on > $1 AND text = $2`, lastNow, tc.expectedMsgText).
			Returns(tc.expectedMsgCount, "%d: unexpected msg count", i)

		lastNow = time.Now()
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBroadcastTask(t *testing.T) {
	ctx, rt := testsuite.Runtime()
	rc := rt.RP.Get()
	defer rc.Close()

	defer testsuite.Reset(testsuite.ResetAll)

	polls := testdata.InsertOptIn(rt, testdata.Org1, "Polls")

	rt.DB.MustExec(`UPDATE orgs_org SET flow_languages = '{"eng", "spa"}' WHERE id = $1`, testdata.Org1.ID)

	oa, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshOrg|models.RefreshOptIns)
	assert.NoError(t, err)

	// add an extra URN for Cathy, change George's language to Spanish, and mark Bob as seen recently
	testdata.InsertContactURN(rt, testdata.Org1, testdata.Cathy, urns.URN("tel:+12065551212"), 1001, nil)
	rt.DB.MustExec(`UPDATE contacts_contact SET language = 'spa', modified_on = NOW() WHERE id = $1`, testdata.George.ID)
	rt.DB.MustExec(`UPDATE contacts_contact SET last_seen_on = NOW() - interval '45 days', modified_on = NOW() WHERE id = $1`, testdata.Bob.ID)

	testsuite.ReindexElastic(ctx)

	tcs := []struct {
		translations    flows.BroadcastTranslations
		baseLanguage    i18n.Language
		expressions     bool
		optIn           *testdata.OptIn
		groupIDs        []models.GroupID
		contactIDs      []models.ContactID
		URNs            []urns.URN
		query           string
		exclusions      models.Exclusions
		createdByID     models.UserID
		queue           *queues.FairSorted
		expectedBatches int
		expectedMsgs    map[string]int
	}{
		{
			translations: flows.BroadcastTranslations{
				"eng": {Text: "hello world"},
			},
			baseLanguage:    "eng",
			expressions:     false,
			optIn:           polls,
			groupIDs:        []models.GroupID{testdata.DoctorsGroup.ID},
			contactIDs:      []models.ContactID{testdata.Cathy.ID},
			exclusions:      models.NoExclusions,
			createdByID:     testdata.Admin.ID,
			queue:           tasks.BatchQueue,
			expectedBatches: 2,
			expectedMsgs:    map[string]int{"hello world": 121},
		},
		{
			translations: flows.BroadcastTranslations{
				"eng": {Text: "hi @(title(contact.name)) from @globals.org_name goflow URN: @urns.tel Gender: @fields.gender"},
			},
			baseLanguage:    "eng",
			expressions:     true,
			contactIDs:      []models.ContactID{testdata.Cathy.ID},
			exclusions:      models.NoExclusions,
			createdByID:     testdata.Agent.ID,
			queue:           tasks.HandlerQueue,
			expectedBatches: 1,
			expectedMsgs:    map[string]int{"hi Cathy from Nyaruka goflow URN: tel:+12065551212 Gender: F": 1},
		},
		{
			translations: flows.BroadcastTranslations{
				"eng": {Text: "hello"},
				"spa": {Text: "hola"},
			},
			baseLanguage:    "eng",
			expressions:     true,
			query:           "name = Cathy OR name = George OR name = Bob",
			exclusions:      models.NoExclusions,
			queue:           tasks.BatchQueue,
			expectedBatches: 1,
			expectedMsgs:    map[string]int{"hello": 2, "hola": 1},
		},
		{
			translations: flows.BroadcastTranslations{
				"eng": {Text: "goodbye"},
				"spa": {Text: "chau"},
			},
			baseLanguage:    "eng",
			expressions:     true,
			query:           "name = Cathy OR name = George OR name = Bob",
			exclusions:      models.Exclusions{NotSeenSinceDays: 60},
			queue:           tasks.BatchQueue,
			expectedBatches: 1,
			expectedMsgs:    map[string]int{"goodbye": 1},
		},
	}

	lastNow := time.Now()
	time.Sleep(10 * time.Millisecond)

	for i, tc := range tcs {
		var optInID models.OptInID
		if tc.optIn != nil {
			optInID = tc.optIn.ID
		}

		bcast := models.NewBroadcast(oa.OrgID(), tc.translations, tc.baseLanguage, tc.expressions, optInID, tc.groupIDs, tc.contactIDs, tc.URNs, tc.query, tc.exclusions, tc.createdByID)

		task := &msgs.SendBroadcastTask{Broadcast: bcast}

		err := tasks.Queue(rc, tasks.BatchQueue, testdata.Org1.ID, task, queues.DefaultPriority)
		assert.NoError(t, err)

		taskCounts := testsuite.FlushTasks(t, rt)

		// assert our count of batches
		assert.Equal(t, tc.expectedBatches, taskCounts["send_broadcast_batch"], "%d: unexpected batch count", i)

		// assert our count of msgs created
		actualMsgs := make(map[string]int)
		rows, err := rt.DB.QueryContext(ctx, `SELECT text, count(*) FROM msgs_msg WHERE org_id = 1 AND created_on > $1 GROUP BY text`, lastNow)
		require.NoError(t, err)
		defer rows.Close()
		for rows.Next() {
			var text string
			var count int
			require.NoError(t, rows.Scan(&text, &count))
			actualMsgs[text] = count
		}

		assert.Equal(t, tc.expectedMsgs, actualMsgs, "%d: msg count mismatch", i)

		if tc.optIn != nil {
			assertdb.Query(t, rt.DB, `SELECT count(*) FROM msgs_msg WHERE org_id = 1 AND created_on > $1 AND optin_id = $2`, lastNow, optInID)
		}

		lastNow = time.Now()
		time.Sleep(5 * time.Millisecond)
	}
}
