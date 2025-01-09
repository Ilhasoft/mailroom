package hooks

import (
	"context"
	"fmt"

	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"

	"github.com/jmoiron/sqlx"
)

// InsertTicketsHook is our hook for inserting tickets
var InsertTicketsHook models.EventCommitHook = &insertTicketsHook{}

type insertTicketsHook struct{}

// Apply inserts all the airtime transfers that were created
func (h *insertTicketsHook) Apply(ctx context.Context, rt *runtime.Runtime, tx *sqlx.Tx, oa *models.OrgAssets, scenes map[*models.Scene][]any) error {
	// gather all our tickets
	tickets := make([]*models.Ticket, 0, len(scenes))

	for _, ts := range scenes {
		for _, t := range ts {
			tickets = append(tickets, t.(*models.Ticket))
		}
	}

	// insert the tickets
	err := models.InsertTickets(ctx, tx, oa, tickets)
	if err != nil {
		return fmt.Errorf("error inserting tickets: %w", err)
	}

	// generate opened events for each ticket
	openEvents := make([]*models.TicketEvent, len(tickets))
	eventsByTicket := make(map[*models.Ticket]*models.TicketEvent, len(tickets))
	for i, ticket := range tickets {
		evt := models.NewTicketOpenedEvent(ticket, ticket.OpenedByID(), ticket.AssigneeID())
		openEvents[i] = evt
		eventsByTicket[ticket] = evt
	}

	// and insert those too
	err = models.InsertTicketEvents(ctx, tx, openEvents)
	if err != nil {
		return fmt.Errorf("error inserting ticket opened events: %w", err)
	}

	// and insert logs/notifications for those
	err = models.NotificationsFromTicketEvents(ctx, tx, oa, eventsByTicket)
	if err != nil {
		return fmt.Errorf("error inserting notifications: %w", err)
	}

	return nil
}
