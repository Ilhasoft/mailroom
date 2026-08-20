package zendesk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasClosedByMergeTag(t *testing.T) {
	assert.False(t, hasClosedByMergeTag(""))
	assert.False(t, hasClosedByMergeTag("open pending"))
	assert.True(t, hasClosedByMergeTag("closed_by_merge"))
	assert.True(t, hasClosedByMergeTag("foo closed_by_merge bar"))
	assert.True(t, hasClosedByMergeTag("CLOSED_BY_MERGE"))
}
