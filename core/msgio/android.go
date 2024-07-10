package msgio

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	fcm "github.com/appleboy/go-fcm"
	"google.golang.org/api/option"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
)

type FCMClient interface {
	Send(ctx context.Context, message ...*messaging.Message) (*messaging.BatchResponse, error)
}

// SyncAndroidChannel tries to trigger sync of the given Android channel via FCM
func SyncAndroidChannel(ctx context.Context, rt *runtime.Runtime, fc FCMClient, channel *models.Channel) error {
	if fc == nil {
		return errors.New("instance has no FCM configuration")
	}

	assert(channel.IsAndroid(), "can't sync a non-android channel")

	// no FCM ID for this channel, noop, we can't trigger a sync
	fcmID := channel.ConfigValue(models.ChannelConfigFCMID, "")
	if fcmID == "" {
		return nil
	}

	sync := &messaging.Message{
		Token: fcmID,
		Android: &messaging.AndroidConfig{
			Priority:    "high",
			CollapseKey: "sync",
		},
		Data: map[string]string{"msg": "sync"},
	}

	start := time.Now()

	if _, err := fc.Send(ctx, sync); err != nil {
		VerifyTokenIDs(ctx, rt, channel, fcmID)

		return fmt.Errorf("error syncing channel: %w", err)
	}

	slog.Debug("android sync complete", "elapsed", time.Since(start), "channel_uuid", channel.UUID())
	return nil
}

// CreateFCMClient creates an FCM client based on the configured FCM API key
func CreateFCMClient(ctx context.Context, cfg *runtime.Config) *fcm.Client {
	if cfg.AndroidFCMServiceAccountFile == "" {
		return nil
	}
	client, err := fcm.NewClient(ctx, fcm.WithCredentialsFile(cfg.AndroidFCMServiceAccountFile))
	if err != nil {
		panic(fmt.Errorf("unable to create FCM client: %w", err))
	}
	return client
}

func VerifyTokenIDs(ctx context.Context, rt *runtime.Runtime, channel *models.Channel, fcmID string) error {
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(rt.Config.AndroidFCMServiceAccountFile))
	if err != nil {
		return err
	}

	firebaseAuthClient, err := app.Auth(ctx)
	if err != nil {
		return err
	}
	// verify the FCM ID
	_, err = firebaseAuthClient.VerifyIDToken(ctx, fcmID)
	if err != nil {
		// clear the FCM ID in the DB
		_, errDB := rt.DB.ExecContext(ctx, `UPDATE channels_channel SET config = config || '{"FCM_ID": ""}'::jsonb WHERE uuid = $1`, channel.UUID())
		if errDB != nil {
			return errDB
		}

		return err
	}
	return nil
}
