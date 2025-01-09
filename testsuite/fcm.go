package testsuite

import (
	"context"
	"errors"
	"slices"

	"firebase.google.com/go/v4/messaging"
)

type MockFCMClient struct {
	// list of valid FCM tokens
	ValidTokens []string

	// log of messages sent to this client
	Messages []*messaging.Message
}

func (fc *MockFCMClient) Send(ctx context.Context, messages ...*messaging.Message) (*messaging.BatchResponse, error) {
	successCount := 0
	failureCount := 0
	sendResponses := make([]*messaging.SendResponse, len(messages))
	var err error

	for _, message := range messages {
		fc.Messages = append(fc.Messages, message)

		if slices.Contains(fc.ValidTokens, message.Token) {
			successCount += 1
			sendResponses = append(sendResponses, &messaging.SendResponse{Success: true})
		} else {
			failureCount += 1
			err = errors.New("401 error: 401 Unauthorized")
			sendResponses = append(sendResponses, &messaging.SendResponse{Error: err})
		}
	}
	return &messaging.BatchResponse{SuccessCount: successCount, FailureCount: failureCount, Responses: sendResponses}, err
}
