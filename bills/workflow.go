package bills

import "go.temporal.io/sdk/workflow"

const (
	TaskQueue         = "bills-task-queue"
	SignalAddLineItem = "add-line-item"
	SignalCloseBill   = "close-bill"
	QueryGetBill      = "get-bill"
)

type AddLineItemSignal struct {
	Item LineItem `json:"item"`
}

type CloseBillSignal struct{}

func BillWorkflow(ctx workflow.Context, input BillInput) (*BillState, error) {
	state := &BillState{
		ID:          input.BillID,
		CustomerID:  input.CustomerID,
		PeriodStart: input.PeriodStart,
		Status:      StatusOpen,
		LineItems:   []LineItem{},
		Totals:      map[Currency]int64{},
		CreatedAt:   workflow.Now(ctx),
		seenItemIDs: map[string]bool{},
	}

	_ = workflow.SetQueryHandler(ctx, QueryGetBill, func() (*BillState, error) {
		return state, nil
	})

	addCh := workflow.GetSignalChannel(ctx, SignalAddLineItem)
	closeCh := workflow.GetSignalChannel(ctx, SignalCloseBill)

	for state.Status == StatusOpen {
		sel := workflow.NewSelector(ctx)

		sel.AddReceive(addCh, func(c workflow.ReceiveChannel, _ bool) {
			var sig AddLineItemSignal
			c.Receive(ctx, &sig)
			if state.seenItemIDs[sig.Item.ID] {
				return
			}
			state.seenItemIDs[sig.Item.ID] = true
			state.LineItems = append(state.LineItems, sig.Item)
			state.Totals[sig.Item.Amount.Currency] += sig.Item.Amount.Amount
		})

		sel.AddReceive(closeCh, func(c workflow.ReceiveChannel, _ bool) {
			var sig CloseBillSignal
			c.Receive(ctx, &sig)
			now := workflow.Now(ctx)
			state.Status = StatusClosed
			state.ClosedAt = &now
		})

		sel.Select(ctx)
	}

	return state, nil
}
