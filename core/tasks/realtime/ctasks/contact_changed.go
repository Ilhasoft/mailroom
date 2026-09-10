package ctasks

import (
	"context"
	"fmt"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/runner"
	"github.com/nyaruka/mailroom/core/tasks/realtime"
	"github.com/nyaruka/mailroom/runtime"
)

const TypeContactChanged = "contact_changed"

func init() {
	realtime.RegisterContactTask(TypeContactChanged, func() realtime.Task { return &ContactChangedTask{} })
}

type ContactChangedTask struct {
	ChannelID models.ChannelID `json:"channel_id"`
	NewURN    *NewURNSpec      `json:"new_urn,omitempty"`
}

func (t *ContactChangedTask) Type() string {
	return TypeContactChanged
}

func (t *ContactChangedTask) UseReadOnly() bool {
	return false
}

func (t *ContactChangedTask) Perform(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, mc *models.Contact) error {
	contact, err := mc.EngineContact(oa)
	if err != nil {
		return fmt.Errorf("error creating engine contact: %w", err)
	}

	scene := runner.NewScene(mc, contact)

	if t.NewURN != nil {
		if err := t.NewURN.Apply(ctx, rt, oa, scene, oa.ChannelByID(t.ChannelID)); err != nil {
			return fmt.Errorf("error applying new URN: %w", err)
		}
	}

	if err := scene.Commit(ctx, rt, oa); err != nil {
		return fmt.Errorf("error committing scene: %w", err)
	}

	return nil
}
