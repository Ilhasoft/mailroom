package starts

import (
	"context"
	"fmt"
	"time"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/runner"
	"github.com/nyaruka/mailroom/core/tasks"
	"github.com/nyaruka/mailroom/runtime"
)

const TypeStartFlowBatch = "start_flow_batch"

func init() {
	tasks.RegisterType(TypeStartFlowBatch, func() tasks.Task { return &StartFlowBatchTask{} })
}

// StartFlowBatchTask is the start flow batch task
type StartFlowBatchTask struct {
	*models.FlowStartBatch
}

func (t *StartFlowBatchTask) Type() string {
	return TypeStartFlowBatch
}

// Timeout is the maximum amount of time the task can run for
func (t *StartFlowBatchTask) Timeout() time.Duration {
	return time.Minute * 15
}

func (t *StartFlowBatchTask) WithAssets() models.Refresh {
	return models.RefreshNone
}

func (t *StartFlowBatchTask) Perform(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets) error {
	// start these contacts in our flow
	_, err := runner.StartFlowBatch(ctx, rt, oa, t.FlowStartBatch)
	if err != nil {
		return fmt.Errorf("error starting flow batch: %w", err)
	}
	return nil
}
