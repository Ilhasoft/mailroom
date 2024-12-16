package models_test

import (
	"testing"

	"github.com/nyaruka/goflow/assets"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannels(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.Reset(testsuite.ResetAll)

	// add some tel specific config to channel 2
	rt.DB.MustExec(`UPDATE channels_channel SET config = '{"matching_prefixes": ["250", "251"], "allow_international": true}' WHERE id = $1`, testdata.VonageChannel.ID)

	oa, err := models.GetOrgAssetsWithRefresh(ctx, rt, 1, models.RefreshChannels)
	require.NoError(t, err)

	channels, err := oa.Channels()
	require.NoError(t, err)

	tcs := []struct {
		ID                 models.ChannelID
		UUID               assets.ChannelUUID
		Name               string
		Address            string
		Schemes            []string
		Roles              []assets.ChannelRole
		Features           []assets.ChannelFeature
		Prefixes           []string
		AllowInternational bool
	}{
		{
			testdata.TwilioChannel.ID,
			testdata.TwilioChannel.UUID,
			"Twilio",
			"+13605551212",
			[]string{"tel"},
			[]assets.ChannelRole{"send", "receive", "call", "answer"},
			[]assets.ChannelFeature{},
			nil,
			false,
		},
		{
			testdata.VonageChannel.ID,
			testdata.VonageChannel.UUID,
			"Vonage",
			"5789",
			[]string{"tel"},
			[]assets.ChannelRole{"send", "receive"},
			[]assets.ChannelFeature{},
			[]string{"250", "251"},
			true,
		},
		{
			testdata.FacebookChannel.ID,
			testdata.FacebookChannel.UUID,
			"Facebook",
			"12345",
			[]string{"facebook"},
			[]assets.ChannelRole{"send", "receive"},
			[]assets.ChannelFeature{"optins"},
			nil,
			false,
		},
	}

	assert.Equal(t, len(tcs), len(channels))
	for i, tc := range tcs {
		channel := channels[i].(*models.Channel)
		assert.Equal(t, tc.UUID, channel.UUID())
		assert.Equal(t, tc.ID, channel.ID())
		assert.Equal(t, tc.Name, channel.Name())
		assert.Equal(t, tc.Address, channel.Address())
		assert.Equal(t, tc.Roles, channel.Roles())
		assert.Equal(t, tc.Features, channel.Features())
		assert.Equal(t, tc.Schemes, channel.Schemes())
		assert.Equal(t, tc.Prefixes, channel.MatchPrefixes())
		assert.Equal(t, tc.AllowInternational, channel.AllowInternational())
	}
}

func TestGetChannelByID(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.Reset(testsuite.ResetAll)

	ch, err := models.GetChannelByID(ctx, rt.DB.DB, testdata.TwilioChannel.ID)
	assert.NoError(t, err)
	assert.Equal(t, testdata.TwilioChannel.ID, ch.ID())
	assert.Equal(t, testdata.TwilioChannel.UUID, ch.UUID())

	_, err = models.GetChannelByID(ctx, rt.DB.DB, 1234567890)
	assert.EqualError(t, err, "error fetching channel by id 1234567890: error scanning row JSON: sql: no rows in result set")

}

func TestGetAndroidChannelsToSync(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.Reset(testsuite.ResetData)

	testChannel1 := testdata.InsertChannel(rt, testdata.Org1, "A", "Android 1", "123", []string{"tel"}, "SR", map[string]any{"FCM_ID": ""})
	testChannel2 := testdata.InsertChannel(rt, testdata.Org1, "A", "Android 2", "234", []string{"tel"}, "SR", map[string]any{"FCM_ID": "FCMID2"})
	testChannel3 := testdata.InsertChannel(rt, testdata.Org1, "A", "Android 3", "456", []string{"tel"}, "SR", map[string]any{"FCM_ID": "FCMID3"})
	testChannel4 := testdata.InsertChannel(rt, testdata.Org1, "A", "Android 4", "567", []string{"tel"}, "SR", map[string]any{"FCM_ID": "FCMID4"})
	testChannel5 := testdata.InsertChannel(rt, testdata.Org1, "A", "Android 5", "678", []string{"tel"}, "SR", map[string]any{"FCM_ID": "FCMID5"})

	rt.DB.MustExec(`UPDATE channels_channel SET last_seen = NOW() - INTERVAL '30 minutes' WHERE id = $1`, testChannel1.ID)
	rt.DB.MustExec(`UPDATE channels_channel SET last_seen = NOW() - INTERVAL '30 minutes' WHERE id = $1`, testChannel2.ID)
	rt.DB.MustExec(`UPDATE channels_channel SET last_seen = NOW() WHERE id = $1`, testChannel3.ID)
	rt.DB.MustExec(`UPDATE channels_channel SET last_seen = NOW() - INTERVAL '20 minutes' WHERE id = $1`, testChannel4.ID)
	rt.DB.MustExec(`UPDATE channels_channel SET last_seen = NOW() - INTERVAL '10 days' WHERE id = $1`, testChannel5.ID)

	oldSeenAndroidChannels, err := models.GetAndroidChannelsToSync(ctx, rt.DB)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(oldSeenAndroidChannels))

	assert.Equal(t, testChannel4.ID, oldSeenAndroidChannels[0].ID())
	assert.Equal(t, testChannel2.ID, oldSeenAndroidChannels[1].ID())
	assert.Equal(t, testChannel1.ID, oldSeenAndroidChannels[2].ID())

}
