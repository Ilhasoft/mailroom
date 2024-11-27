package models_test

import (
	"testing"

	"github.com/nyaruka/gocommon/random"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/test"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/nyaruka/redisx/assertredis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordFlowStatistics(t *testing.T) {
	ctx, rt := testsuite.Runtime()
	rc := rt.RP.Get()
	defer rc.Close()

	defer testsuite.Reset(testsuite.ResetRedis | testsuite.ResetData)

	defer random.SetGenerator(random.DefaultGenerator)
	random.SetGenerator(random.NewSeededGenerator(123))

	testFlows := testdata.ImportFlows(rt, testdata.Org1, "testdata/flow_stats_test.json")
	flow := testFlows[0]

	oa, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshFlows)
	require.NoError(t, err)

	sa1, session1, session1Sprint1 := test.NewSessionBuilder().WithAssets(oa.SessionAssets()).WithFlow(flow.UUID).
		WithContact("4ad4f0a6-fb95-4845-b4cb-335f67eafe96", 123, "Bob", "eng", "").MustBuild()
	sa2, session2, session2Sprint1 := test.NewSessionBuilder().WithAssets(oa.SessionAssets()).WithFlow(flow.UUID).
		WithContact("5cfe8b70-0d4a-4862-8fb5-e72603d832a9", 234, "Ann", "eng", "").MustBuild()
	sa3, session3, session3Sprint1 := test.NewSessionBuilder().WithAssets(oa.SessionAssets()).WithFlow(flow.UUID).
		WithContact("367c8ef2-aac7-4264-9a03-40877371995d", 345, "Jim", "eng", "").MustBuild()

	err = models.RecordFlowStatistics(ctx, rt, rt.DB, []flows.Session{session1, session2, session3}, []flows.Sprint{session1Sprint1, session2Sprint1, session3Sprint1})
	require.NoError(t, err)

	// should have a single record of all 3 contacts going through the first segment
	var counts []*models.FlowActivityCount
	err = rt.DB.SelectContext(ctx, &counts, "SELECT flow_id, scope, count FROM flows_flowactivitycount ORDER BY flow_id, scope")
	require.NoError(t, err)
	assert.Len(t, counts, 1)
	assert.Equal(t, &models.FlowActivityCount{flow.ID, "segment:5fd2e537-0534-4c12-8425-bef87af09d46:072b95b3-61c3-4e0e-8dd1-eb7481083f94", 3}, counts[0])

	assertFlowActivityCounts(t, rt, flow.ID, map[string]int{"segment:5fd2e537-0534-4c12-8425-bef87af09d46:072b95b3-61c3-4e0e-8dd1-eb7481083f94": 3})

	assertredis.Keys(t, rc, "*", []string{
		"recent_contacts:5fd2e537-0534-4c12-8425-bef87af09d46:072b95b3-61c3-4e0e-8dd1-eb7481083f94", // "what's your fav color" -> color split
	})

	// all 3 contacts went from first msg to the color split - no operands recorded for this segment
	assertredis.ZRange(t, rc, "recent_contacts:5fd2e537-0534-4c12-8425-bef87af09d46:072b95b3-61c3-4e0e-8dd1-eb7481083f94", 0, -1,
		[]string{"LZbbzXDPJH|123|", "reuPYVP90u|234|", "qWARtWDACk|345|"},
	)

	_, session1Sprint2, err := test.ResumeSession(session1, sa1, "blue")
	require.NoError(t, err)
	_, session2Sprint2, err := test.ResumeSession(session2, sa2, "BLUE")
	require.NoError(t, err)
	session3, session3Sprint2, err := test.ResumeSession(session3, sa3, "teal")
	require.NoError(t, err)

	err = models.RecordFlowStatistics(ctx, rt, rt.DB, []flows.Session{session1, session2, session3}, []flows.Sprint{session1Sprint2, session2Sprint2, session3Sprint2})
	require.NoError(t, err)

	assertFlowActivityCounts(t, rt, flow.ID, map[string]int{
		"segment:5fd2e537-0534-4c12-8425-bef87af09d46:072b95b3-61c3-4e0e-8dd1-eb7481083f94": 3, // "what's your fav color" -> color split
		"segment:c02fc3ba-369a-4c87-9bc4-c3b376bda6d2:57b50d33-2b5a-4726-82de-9848c61eff6e": 2, // color split :: Blue exit -> next node
		"segment:ea6c38dc-11e2-4616-9f3e-577e44765d44:8712db6b-25ff-4789-892c-581f24eeeb95": 1, // color split :: Other exit -> next node
		"segment:2b698218-87e5-4ab8-922e-e65f91d12c10:88d8bf00-51ce-4e5e-aae8-4f957a0761a0": 2, // split by expression :: Other exit -> next node
		"segment:0a4f2ea9-c47f-4e9c-a242-89ae5b38d679:072b95b3-61c3-4e0e-8dd1-eb7481083f94": 1, // "sorry I don't know that color" -> color split
		"segment:97cd44ce-dec2-4e19-8ca2-4e20db51dc08:0e1fe072-6f03-4f29-98aa-7bedbe930dab": 2, // "X is a great color" -> split by expression
		"segment:614e7451-e0bd-43d9-b317-2aded3c8d790:a1e649db-91e0-47c4-ab14-eba0d1475116": 2, // "you have X tickets" -> group split
	})

	_, session3Sprint3, err := test.ResumeSession(session3, sa3, "azure")
	require.NoError(t, err)

	err = models.RecordFlowStatistics(ctx, rt, rt.DB, []flows.Session{session3}, []flows.Sprint{session3Sprint3})
	require.NoError(t, err)

	assertFlowActivityCounts(t, rt, flow.ID, map[string]int{
		"segment:5fd2e537-0534-4c12-8425-bef87af09d46:072b95b3-61c3-4e0e-8dd1-eb7481083f94": 3, // "what's your fav color" -> color split
		"segment:c02fc3ba-369a-4c87-9bc4-c3b376bda6d2:57b50d33-2b5a-4726-82de-9848c61eff6e": 2, // color split :: Blue exit -> next node
		"segment:ea6c38dc-11e2-4616-9f3e-577e44765d44:8712db6b-25ff-4789-892c-581f24eeeb95": 2, // color split :: Other exit -> next node
		"segment:2b698218-87e5-4ab8-922e-e65f91d12c10:88d8bf00-51ce-4e5e-aae8-4f957a0761a0": 2, // split by expression :: Other exit -> next node
		"segment:0a4f2ea9-c47f-4e9c-a242-89ae5b38d679:072b95b3-61c3-4e0e-8dd1-eb7481083f94": 2, // "sorry I don't know that color" -> color split
		"segment:97cd44ce-dec2-4e19-8ca2-4e20db51dc08:0e1fe072-6f03-4f29-98aa-7bedbe930dab": 2, // "X is a great color" -> split by expression
		"segment:614e7451-e0bd-43d9-b317-2aded3c8d790:a1e649db-91e0-47c4-ab14-eba0d1475116": 2, // "you have X tickets" -> group split
	})

	assertredis.Keys(t, rc, "*", []string{
		"recent_contacts:5fd2e537-0534-4c12-8425-bef87af09d46:072b95b3-61c3-4e0e-8dd1-eb7481083f94", // "what's your fav color" -> color split
		"recent_contacts:c02fc3ba-369a-4c87-9bc4-c3b376bda6d2:57b50d33-2b5a-4726-82de-9848c61eff6e", // color split :: Blue exit -> next node
		"recent_contacts:ea6c38dc-11e2-4616-9f3e-577e44765d44:8712db6b-25ff-4789-892c-581f24eeeb95", // color split :: Other exit -> next node
		"recent_contacts:2b698218-87e5-4ab8-922e-e65f91d12c10:88d8bf00-51ce-4e5e-aae8-4f957a0761a0", // split by expression :: Other exit -> next node
		"recent_contacts:0a4f2ea9-c47f-4e9c-a242-89ae5b38d679:072b95b3-61c3-4e0e-8dd1-eb7481083f94", // "sorry I don't know that color" -> color split
		"recent_contacts:97cd44ce-dec2-4e19-8ca2-4e20db51dc08:0e1fe072-6f03-4f29-98aa-7bedbe930dab", // "X is a great color" -> split by expression
		"recent_contacts:614e7451-e0bd-43d9-b317-2aded3c8d790:a1e649db-91e0-47c4-ab14-eba0d1475116", // "you have X tickets" -> group split
	})

	// check recent operands for color split :: Blue exit -> next node
	assertredis.ZRange(t, rc, "recent_contacts:c02fc3ba-369a-4c87-9bc4-c3b376bda6d2:57b50d33-2b5a-4726-82de-9848c61eff6e", 0, -1,
		[]string{"2SS5dyuJzp|123|blue", "2MsZZ/N3TH|234|BLUE"},
	)

	// check recent operands for color split :: Other exit -> next node
	assertredis.ZRange(t, rc, "recent_contacts:ea6c38dc-11e2-4616-9f3e-577e44765d44:8712db6b-25ff-4789-892c-581f24eeeb95", 0, -1,
		[]string{"uI8bPiuaeA|345|teal", "2Vz/MpdX9s|345|azure"},
	)

	// check recent operands for split by expression :: Other exit -> next node
	assertredis.ZRange(t, rc, "recent_contacts:2b698218-87e5-4ab8-922e-e65f91d12c10:88d8bf00-51ce-4e5e-aae8-4f957a0761a0", 0, -1,
		[]string{"PLQQFoOgV9|123|0", "/cgnkcW6vA|234|0"},
	)
}

func assertFlowActivityCounts(t *testing.T, rt *runtime.Runtime, flowID models.FlowID, expected map[string]int) {
	var counts []*models.FlowActivityCount
	err := rt.DB.Select(&counts, "SELECT flow_id, scope, SUM(count) AS count FROM flows_flowactivitycount WHERE flow_id = $1 GROUP BY flow_id, scope", flowID)
	require.NoError(t, err)

	actual := make(map[string]int)
	for _, c := range counts {
		actual[c.Scope] = c.Count
	}

	assert.Equal(t, expected, actual)
}
