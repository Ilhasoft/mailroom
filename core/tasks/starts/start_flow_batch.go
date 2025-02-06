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
	var start *models.FlowStart
	var err error

	// if this batch belongs to a persisted start, fetch it
	if t.StartID != models.NilStartID {
		start, err = models.GetFlowStartByID(ctx, rt.DB, t.StartID)
		if err != nil {
			return fmt.Errorf("error loading flow start for batch: %w", err)
		}
	} else {
		start = t.Start // otherwise use start from the task
	}

	// if this start was interrupted, we're done
	if start.Status == models.StartStatusInterrupted {
		return nil
	}

	// if this is our first batch, mark as started
	if t.IsFirst {
		if err := start.SetStarted(ctx, rt.DB); err != nil {
			return fmt.Errorf("error marking start as started: %w", err)
		}
	}

	// start these contacts in our flow
	_, err = runner.StartFlowBatch(ctx, rt, oa, start, t.FlowStartBatch)
	if err != nil {
		return fmt.Errorf("error starting flow batch: %w", err)
	}

	// if this is our last batch, mark start as done
	if t.IsLast {
		if err := start.SetCompleted(ctx, rt.DB); err != nil {
			return fmt.Errorf("error marking start as complete: %w", err)
		}
	}

	return nil
}
