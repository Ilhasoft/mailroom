package transferto

import (
	"encoding/json"
	"fmt"

	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/goflow/extensions/transferto/client"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/actions"
	"github.com/nyaruka/goflow/flows/events"
	"github.com/nyaruka/goflow/utils"

	"github.com/shopspring/decimal"
)

func init() {
	actions.RegisterType(TypeTransferAirtime, func() flows.Action { return &TransferAirtimeAction{} })
}

type transferToConfig struct {
	APIToken string `json:"api_token"`
	Login    string `json:"login"`
	Currency string `json:"currency"`
	Disabled bool   `json:"disabled"`
}

// TypeTransferAirtime is the type constant for our airtime action
var TypeTransferAirtime = "transfer_airtime"

// TransferAirtimeAction attempts to make a TransferTo airtime transfer to the contact
type TransferAirtimeAction struct {
	actions.BaseAction

	Amounts    map[string]decimal.Decimal `json:"amounts"`
	ResultName string                     `json:"result_name,omitempty"`
}

// Type returns the type of this router
func (a *TransferAirtimeAction) Type() string { return TypeTransferAirtime }

// Validate validates our action is valid and has all the assets it needs
func (a *TransferAirtimeAction) Validate(assets flows.SessionAssets) error {
	return nil
}

// AllowedFlowTypes returns the flow types which this action is allowed to occur in
func (a *TransferAirtimeAction) AllowedFlowTypes() []flows.FlowType {
	return []flows.FlowType{flows.FlowTypeMessaging, flows.FlowTypeVoice}
}

// Execute runs this action
func (a *TransferAirtimeAction) Execute(run flows.FlowRun, step flows.Step) error {
	contact := run.Contact()
	if contact == nil {
		run.LogEvent(step, events.NewErrorEvent(fmt.Errorf("can't execute action in session without a contact")))
		return nil
	}

	// check that our contact has a tel URN
	telURNs := contact.URNs().WithScheme(urns.TelScheme)
	if len(telURNs) == 0 {
		run.LogEvent(step, events.NewErrorEvent(fmt.Errorf("can't transfer airtime to contact without a tel URN")))
		return nil
	}
	recipient := telURNs[0].Path()

	// log error and return if we don't have a configuration
	rawConfig := run.Session().Environment().Extension("transferto")
	if rawConfig == nil {
		run.LogEvent(step, events.NewErrorEvent(fmt.Errorf("missing transferto configuration")))
		return nil
	}

	config := &transferToConfig{}
	if err := json.Unmarshal(rawConfig, config); err != nil {
		return fmt.Errorf("unable to read config: %s", err)
	}

	transfer, err := attemptTransfer(contact.PreferredChannel(), config, a.Amounts, recipient, run.Session().HTTPClient())

	if err != nil {
		run.LogEvent(step, events.NewErrorEvent(err))
	} else {
		run.LogEvent(step, NewAirtimeTransferredEvent(transfer))
	}

	if a.ResultName != "" && transfer != nil {
		value := transfer.actualAmount.String()
		category := statusCategories[transfer.status]
		result := flows.NewResult(a.ResultName, value, category, "", step.NodeUUID(), nil, nil, utils.Now())

		run.SaveResult(result)
		run.LogEvent(step, events.NewRunResultChangedEvent(result))
	}
	return nil
}

type transferStatus string

const (
	transferStatusSuccess transferStatus = "success"
	transferStatusFailed  transferStatus = "failed"
)

type transfer struct {
	recipient     string
	currency      string
	desiredAmount decimal.Decimal
	actualAmount  decimal.Decimal
	status        transferStatus
}

var statusCategories = map[transferStatus]string{
	transferStatusSuccess: "Success",
	transferStatusFailed:  "Failure",
}

// attempts to make the transfer, returning the actual transfer or an error
func attemptTransfer(channel *flows.Channel, config *transferToConfig, amounts map[string]decimal.Decimal, recipient string, httpClient *utils.HTTPClient) (*transfer, error) {
	// if airtime transferred are disabled, return a mock transfer
	if config.Disabled {
		amount := decimal.RequireFromString("1")
		return &transfer{recipient: recipient, currency: config.Currency, desiredAmount: amount, actualAmount: amount, status: transferStatusSuccess}, nil
	}

	cl := client.NewTransferToClient(config.Login, config.APIToken, httpClient)
	t := &transfer{recipient: recipient, status: transferStatusFailed}

	info, err := cl.MSISDNInfo(recipient, config.Currency, "1")
	if err != nil {
		return t, err
	}

	t.currency = info.DestinationCurrency

	// look up the amount to send in this currency
	amount, hasAmount := amounts[t.currency]
	if !hasAmount {
		return t, fmt.Errorf("no amount configured for transfers in %s", t.currency)
	}
	t.desiredAmount = amount

	if info.OpenRange {
		// TODO add support for open-range topups once we can find numbers to test this with
		// see https://shop.transferto.com/shop/v3/doc/TransferTo_API_OR.pdf
		return t, fmt.Errorf("transferto account is configured for open-range which is not yet supported")
	}

	// find the product closest to our desired amount
	var useProduct string
	useAmount := decimal.Zero
	for p, product := range info.ProductList {
		price := info.LocalInfoValueList[p]
		if price.GreaterThan(useAmount) && price.LessThanOrEqual(amount) {
			useProduct = product
			useAmount = price
		}
	}
	t.actualAmount = useAmount

	reservedID, err := cl.ReserveID()
	if err != nil {
		return t, err
	}

	var fromMSISDN string
	if channel != nil {
		fromMSISDN = channel.Address()
	}

	topup, err := cl.Topup(reservedID, fromMSISDN, recipient, useProduct, "")
	if err != nil {
		return t, err
	}
	t.actualAmount = topup.ActualProductSent
	t.status = transferStatusSuccess

	return t, nil
}
