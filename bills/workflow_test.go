package bills

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type WorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *WorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(BillWorkflow)
}

func (s *WorkflowTestSuite) TearDownTest() {
	s.env.AssertExpectations(s.T())
}

func TestWorkflows(t *testing.T) {
	suite.Run(t, new(WorkflowTestSuite))
}

func (s *WorkflowTestSuite) Test_CloseEmptyBill() {
	input := BillInput{BillID: "bill-1", CustomerID: "cust-1", PeriodStart: "2026-08"}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalCloseBill, CloseBillSignal{})
	}, time.Millisecond)

	s.env.ExecuteWorkflow(BillWorkflow, input)

	s.True(s.env.IsWorkflowCompleted())
	var state BillState
	s.NoError(s.env.GetWorkflowResult(&state))
	s.Equal(StatusClosed, state.Status)
	s.Empty(state.LineItems)
	s.Empty(state.Totals)
	s.NotNil(state.ClosedAt)
}

func (s *WorkflowTestSuite) Test_MultiCurrencyTotals() {
	input := BillInput{BillID: "bill-2", CustomerID: "cust-1", PeriodStart: "2026-08"}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, AddLineItemSignal{
			Item: LineItem{ID: "item-1", Description: "USD fee", Amount: Money{Amount: 1000, Currency: USD}},
		})
	}, time.Millisecond)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, AddLineItemSignal{
			Item: LineItem{ID: "item-2", Description: "GEL fee", Amount: Money{Amount: 500, Currency: GEL}},
		})
	}, time.Millisecond*2)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalCloseBill, CloseBillSignal{})
	}, time.Millisecond*3)

	s.env.ExecuteWorkflow(BillWorkflow, input)

	s.True(s.env.IsWorkflowCompleted())
	var state BillState
	s.NoError(s.env.GetWorkflowResult(&state))
	s.Equal(StatusClosed, state.Status)
	s.Len(state.LineItems, 2)
	s.Equal(int64(1000), state.Totals[USD])
	s.Equal(int64(500), state.Totals[GEL])
}

func (s *WorkflowTestSuite) Test_IdempotentLineItem() {
	input := BillInput{BillID: "bill-3", CustomerID: "cust-1", PeriodStart: "2026-08"}

	// Send the same item ID twice.
	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, AddLineItemSignal{
			Item: LineItem{ID: "idempotent-key", Description: "Fee", Amount: Money{Amount: 1000, Currency: USD}},
		})
	}, time.Millisecond)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalAddLineItem, AddLineItemSignal{
			Item: LineItem{ID: "idempotent-key", Description: "Fee", Amount: Money{Amount: 1000, Currency: USD}},
		})
	}, time.Millisecond*2)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalCloseBill, CloseBillSignal{})
	}, time.Millisecond*3)

	s.env.ExecuteWorkflow(BillWorkflow, input)

	s.True(s.env.IsWorkflowCompleted())
	var state BillState
	s.NoError(s.env.GetWorkflowResult(&state))
	s.Len(state.LineItems, 1)               // duplicate was dropped
	s.Equal(int64(1000), state.Totals[USD]) // counted only once
}
