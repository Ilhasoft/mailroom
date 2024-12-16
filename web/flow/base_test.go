package flow_test

import (
	"testing"

	"github.com/nyaruka/mailroom/testsuite"
)

func TestChangeLanguage(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	testsuite.RunWebTests(t, ctx, rt, "testdata/change_language.json", nil)
}

func TestClone(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	testsuite.RunWebTests(t, ctx, rt, "testdata/clone.json", nil)
}

func TestInspect(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	testsuite.RunWebTests(t, ctx, rt, "testdata/inspect.json", nil)
}

func TestMigrate(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	testsuite.RunWebTests(t, ctx, rt, "testdata/migrate.json", nil)
}

func TestStartPreview(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	testsuite.RunWebTests(t, ctx, rt, "testdata/start_preview.json", nil)
}
