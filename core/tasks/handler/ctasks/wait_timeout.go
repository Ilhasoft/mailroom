package ctasks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nyaruka/goflow/flows/engine"
	"github.com/nyaruka/goflow/flows/resumes"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/core/runner"
	"github.com/nyaruka/mailroom/core/tasks/handler"
	"github.com/nyaruka/mailroom/runtime"
)

const TypeWaitTimeout = "timeout_event"

func init() {
	handler.RegisterContactTask(TypeWaitTimeout, func() handler.Task { return &WaitTimeoutTask{} })
}

type WaitTimeoutTask struct {
	SessionID models.SessionID `json:"session_id"`
	Time      time.Time        `json:"time"`
}

func NewWaitTimeout(sessionID models.SessionID, time time.Time) *WaitTimeoutTask {
	return &WaitTimeoutTask{SessionID: sessionID, Time: time}
}

func (t *WaitTimeoutTask) Type() string {
	return TypeWaitTimeout
}

func (t *WaitTimeoutTask) UseReadOnly() bool {
	return true
}

func (t *WaitTimeoutTask) Perform(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, contact *models.Contact) error {
	log := slog.With("contact_id", contact.ID(), "session_id", t.SessionID)

	// build our flow contact
	flowContact, err := contact.FlowContact(oa)
	if err != nil {
		return fmt.Errorf("error creating flow contact: %w", err)
	}

	// look for a waiting session for this contact
	session, err := models.FindWaitingSessionForContact(ctx, rt.DB, rt.SessionStorage, oa, models.FlowTypeMessaging, flowContact)
	if err != nil {
		return fmt.Errorf("error loading waiting session for contact: %w", err)
	}

	// if we didn't find a session or it is another session then this session has already been interrupted
	if session == nil || session.ID() != t.SessionID {
		return nil
	}

	if session.WaitTimeoutOn() == nil {
		log.Info("ignoring session timeout, has no timeout set")
		return nil
	}

	// check that the timeout is the same
	timeout := *session.WaitTimeoutOn()
	if !timeout.Equal(t.Time) {
		log.Info("ignoring timeout, has been updated", "event_timeout", t.Time, "session_timeout", timeout)
		return nil
	}

	resume := resumes.NewWaitTimeout(oa.Env(), flowContact)

	_, err = runner.ResumeFlow(ctx, rt, oa, session, contact, resume, nil)
	if err != nil {
		// if we errored, and it's the wait rejecting the timeout event, it's because it no longer exists on the flow, so clear it
		// on the session
		var eerr *engine.Error
		if errors.As(err, &eerr) && eerr.Code() == engine.ErrorResumeRejectedByWait && resume.Type() == resumes.TypeWaitTimeout {
			log.Info("clearing session timeout which is no longer set in flow")

			if err := session.ClearWaitTimeout(ctx, rt.DB); err != nil {
				return fmt.Errorf("error clearing session timeout: %w", err)
			}

			return nil
		}

		return fmt.Errorf("error resuming flow for timeout: %w", err)
	}

	return nil
}
