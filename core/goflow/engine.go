package goflow

import (
	"context"
	"sync"

	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/engine"
	"github.com/nyaruka/goflow/services/webhooks"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/shopspring/decimal"
)

var eng, simulator flows.Engine
var engInit, simulatorInit sync.Once

var emailFactory func(*runtime.Runtime) engine.EmailServiceFactory
var classificationFactory func(*runtime.Runtime) engine.ClassificationServiceFactory
var airtimeFactory func(*runtime.Runtime) engine.AirtimeServiceFactory

// RegisterEmailServiceFactory can be used by outside callers to register a email factory
// for use by the engine
func RegisterEmailServiceFactory(f func(*runtime.Runtime) engine.EmailServiceFactory) {
	emailFactory = f
}

// RegisterClassificationServiceFactory can be used by outside callers to register a classification factory
// for use by the engine
func RegisterClassificationServiceFactory(f func(*runtime.Runtime) engine.ClassificationServiceFactory) {
	classificationFactory = f
}

// RegisterAirtimeServiceFactory can be used by outside callers to register a airtime factory
// for use by the engine
func RegisterAirtimeServiceFactory(f func(*runtime.Runtime) engine.AirtimeServiceFactory) {
	airtimeFactory = f
}

// Engine returns the global engine instance for use with real sessions
func Engine(rt *runtime.Runtime) flows.Engine {
	engInit.Do(func() {
		webhookHeaders := map[string]string{
			"User-Agent":      "RapidProMailroom/" + rt.Config.Version,
			"X-Mailroom-Mode": "normal",
		}

		httpClient, httpRetries, httpAccess := HTTP(rt.Config)

		eng = engine.NewBuilder().
			WithWebhookServiceFactory(webhooks.NewServiceFactory(httpClient, httpRetries, httpAccess, webhookHeaders, rt.Config.WebhooksMaxBodyBytes)).
			WithClassificationServiceFactory(classificationFactory(rt)).
			WithEmailServiceFactory(emailFactory(rt)).
			WithAirtimeServiceFactory(airtimeFactory(rt)).
			WithMaxStepsPerSprint(rt.Config.MaxStepsPerSprint).
			WithMaxResumesPerSession(rt.Config.MaxResumesPerSession).
			WithMaxFieldChars(rt.Config.MaxValueLength).
			WithMaxResultChars(rt.Config.MaxValueLength).
			Build()
	})

	return eng
}

// Simulator returns the global engine instance for use with simulated sessions
func Simulator(ctx context.Context, rt *runtime.Runtime) flows.Engine {
	simulatorInit.Do(func() {
		webhookHeaders := map[string]string{
			"User-Agent":      "RapidProMailroom/" + rt.Config.Version,
			"X-Mailroom-Mode": "simulation",
		}

		httpClient, _, httpAccess := HTTP(rt.Config) // don't do retries in simulator

		simulator = engine.NewBuilder().
			WithWebhookServiceFactory(webhooks.NewServiceFactory(httpClient, nil, httpAccess, webhookHeaders, rt.Config.WebhooksMaxBodyBytes)).
			WithClassificationServiceFactory(classificationFactory(rt)). // simulated sessions do real classification
			WithEmailServiceFactory(simulatorEmailServiceFactory).       // but faked emails
			WithAirtimeServiceFactory(simulatorAirtimeServiceFactory).   // and faked airtime transfers
			WithMaxStepsPerSprint(rt.Config.MaxStepsPerSprint).
			WithMaxResumesPerSession(rt.Config.MaxResumesPerSession).
			WithMaxFieldChars(rt.Config.MaxValueLength).
			WithMaxResultChars(rt.Config.MaxValueLength).
			Build()
	})

	return simulator
}

func simulatorEmailServiceFactory(flows.SessionAssets) (flows.EmailService, error) {
	return &simulatorEmailService{}, nil
}

type simulatorEmailService struct{}

func (s *simulatorEmailService) Send(addresses []string, subject, body string) error {
	return nil
}

func simulatorAirtimeServiceFactory(flows.SessionAssets) (flows.AirtimeService, error) {
	return &simulatorAirtimeService{}, nil
}

type simulatorAirtimeService struct{}

func (s *simulatorAirtimeService) Transfer(sender urns.URN, recipient urns.URN, amounts map[string]decimal.Decimal, logHTTP flows.HTTPLogCallback) (*flows.AirtimeTransfer, error) {
	transfer := &flows.AirtimeTransfer{
		Sender:    sender,
		Recipient: recipient,
		Amount:    decimal.Zero,
	}

	// pick arbitrary currency/amount pair in map
	for currency, amount := range amounts {
		transfer.Currency = currency
		transfer.Amount = amount
		break
	}

	return transfer, nil
}
