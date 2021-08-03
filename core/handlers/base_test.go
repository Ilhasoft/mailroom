package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/goflow/assets"
	"github.com/nyaruka/goflow/envs"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/definition"
	"github.com/nyaruka/goflow/flows/routers"
	"github.com/nyaruka/goflow/flows/triggers"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/runner"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"

	"github.com/gomodule/redigo/redis"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ContactActionMap map[*testdata.Contact][]flows.Action
type ContactMsgMap map[*testdata.Contact]*flows.MsgIn
type ContactModifierMap map[*testdata.Contact][]flows.Modifier

type modifyResult struct {
	Contact *flows.Contact `json:"contact"`
	Events  []flows.Event  `json:"events"`
}

type TestCase struct {
	FlowType      flows.FlowType
	Actions       ContactActionMap
	Msgs          ContactMsgMap
	Modifiers     ContactModifierMap
	Assertions    []Assertion
	SQLAssertions []SQLAssertion
}

type Assertion func(t *testing.T, rt *runtime.Runtime) error

type SQLAssertion struct {
	SQL   string
	Args  []interface{}
	Count int
}

func NewActionUUID() flows.ActionUUID {
	return flows.ActionUUID(uuids.New())
}

func TestMain(m *testing.M) {
	testsuite.Reset()
	os.Exit(m.Run())
}

// createTestFlow creates a flow that starts with a split by contact id
// and then routes the contact to a node where all the actions in the
// test case are present.
//
// It returns the completed flow.
func createTestFlow(t *testing.T, uuid assets.FlowUUID, tc TestCase) flows.Flow {
	categoryUUIDs := make([]flows.CategoryUUID, len(tc.Actions))
	exitUUIDs := make([]flows.ExitUUID, len(tc.Actions))
	i := 0
	for range tc.Actions {
		categoryUUIDs[i] = flows.CategoryUUID(uuids.New())
		exitUUIDs[i] = flows.ExitUUID(uuids.New())
		i++
	}
	defaultCategoryUUID := flows.CategoryUUID(uuids.New())
	defaultExitUUID := flows.ExitUUID(uuids.New())

	cases := make([]*routers.Case, len(tc.Actions))
	categories := make([]flows.Category, len(tc.Actions))
	exits := make([]flows.Exit, len(tc.Actions))
	exitNodes := make([]flows.Node, len(tc.Actions))
	i = 0
	for contact, actions := range tc.Actions {
		cases[i] = routers.NewCase(
			uuids.New(),
			"has_any_word",
			[]string{fmt.Sprintf("%d", contact.ID)},
			categoryUUIDs[i],
		)

		exitNodes[i] = definition.NewNode(
			flows.NodeUUID(uuids.New()),
			actions,
			nil,
			[]flows.Exit{definition.NewExit(flows.ExitUUID(uuids.New()), "")},
		)

		categories[i] = routers.NewCategory(
			categoryUUIDs[i],
			fmt.Sprintf("Contact %d", contact.ID),
			exitUUIDs[i],
		)

		exits[i] = definition.NewExit(
			exitUUIDs[i],
			exitNodes[i].UUID(),
		)
		i++
	}

	// create our router
	categories = append(categories, routers.NewCategory(
		defaultCategoryUUID,
		"Other",
		defaultExitUUID,
	))
	exits = append(exits, definition.NewExit(
		defaultExitUUID,
		flows.NodeUUID(""),
	))

	router := routers.NewSwitch(nil, "", categories, "@contact.id", cases, defaultCategoryUUID)

	// and our entry node
	entry := definition.NewNode(
		flows.NodeUUID(uuids.New()),
		nil,
		router,
		exits,
	)

	nodes := []flows.Node{entry}
	nodes = append(nodes, exitNodes...)

	flowType := tc.FlowType
	if flowType == "" {
		flowType = flows.FlowTypeMessaging
	}

	// we have our nodes, lets create our flow
	flow, err := definition.NewFlow(
		uuid,
		"Test Flow",
		envs.Language("eng"),
		flowType,
		1,
		300,
		definition.NewLocalization(),
		nodes,
		nil,
	)
	require.NoError(t, err)

	return flow
}

func RunTestCases(t *testing.T, tcs []TestCase) {
	ctx, rt, db, _ := testsuite.Get()

	models.FlushCache()

	oa, err := models.GetOrgAssets(ctx, db, models.OrgID(1))
	assert.NoError(t, err)

	// reuse id from one of our real flows
	flowUUID := testdata.Favorites.UUID

	for i, tc := range tcs {
		msgsByContactID := make(map[models.ContactID]*flows.MsgIn)
		for contact, msg := range tc.Msgs {
			msgsByContactID[contact.ID] = msg
		}

		// build our flow for this test case
		testFlow := createTestFlow(t, flowUUID, tc)
		flowDef, err := json.Marshal(testFlow)
		require.NoError(t, err)

		oa, err = oa.CloneForSimulation(ctx, db, map[assets.FlowUUID]json.RawMessage{flowUUID: flowDef}, nil)
		assert.NoError(t, err)

		flow, err := oa.Flow(flowUUID)
		require.NoError(t, err)

		options := runner.NewStartOptions()
		options.CommitHook = func(ctx context.Context, tx *sqlx.Tx, rp *redis.Pool, oa *models.OrgAssets, session []*models.Session) error {
			for _, s := range session {
				msg := msgsByContactID[s.ContactID()]
				if msg != nil {
					s.SetIncomingMsg(msg.ID(), "")
				}
			}
			return nil
		}
		options.TriggerBuilder = func(contact *flows.Contact) flows.Trigger {
			msg := msgsByContactID[models.ContactID(contact.ID())]
			if msg == nil {
				return triggers.NewBuilder(oa.Env(), testFlow.Reference(), contact).Manual().Build()
			}
			return triggers.NewBuilder(oa.Env(), testFlow.Reference(), contact).Msg(msg).Build()
		}

		for _, c := range []*testdata.Contact{testdata.Cathy, testdata.Bob, testdata.George, testdata.Alexandria} {
			_, err := runner.StartFlow(ctx, rt, oa, flow.(*models.Flow), []models.ContactID{c.ID}, options)
			require.NoError(t, err)
		}

		results := make(map[models.ContactID]modifyResult)

		// create scenes for our contacts
		scenes := make([]*models.Scene, 0, len(tc.Modifiers))
		for contact, mods := range tc.Modifiers {
			contacts, err := models.LoadContacts(ctx, db, oa, []models.ContactID{contact.ID})
			assert.NoError(t, err)

			contact := contacts[0]
			flowContact, err := contact.FlowContact(oa)
			assert.NoError(t, err)

			result := modifyResult{
				Contact: flowContact,
				Events:  make([]flows.Event, 0, len(mods)),
			}

			scene := models.NewSceneForContact(flowContact)

			// apply our modifiers
			for _, mod := range mods {
				mod.Apply(oa.Env(), oa.SessionAssets(), flowContact, func(e flows.Event) { result.Events = append(result.Events, e) })
			}

			results[contact.ID()] = result
			scenes = append(scenes, scene)

		}

		tx, err := db.BeginTxx(ctx, nil)
		assert.NoError(t, err)

		for _, scene := range scenes {
			err := models.HandleEvents(ctx, tx, rt.RP, oa, scene, results[scene.ContactID()].Events)
			assert.NoError(t, err)
		}

		err = models.ApplyEventPreCommitHooks(ctx, tx, rt.RP, oa, scenes)
		assert.NoError(t, err)

		err = tx.Commit()
		assert.NoError(t, err)

		tx, err = db.BeginTxx(ctx, nil)
		assert.NoError(t, err)

		err = models.ApplyEventPostCommitHooks(ctx, tx, rt.RP, oa, scenes)
		assert.NoError(t, err)

		err = tx.Commit()
		assert.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		// now check our assertions
		for j, a := range tc.SQLAssertions {
			testsuite.AssertQuery(t, db, a.SQL, a.Args...).Returns(a.Count, "%d:%d: mismatch in expected count for query: %s", i, j, a.SQL)
		}

		for j, a := range tc.Assertions {
			err := a(t, rt)
			assert.NoError(t, err, "%d:%d error checking assertion", i, j)
		}
	}
}
