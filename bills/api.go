package bills

import (
	"context"
	"fmt"
	"time"

	"encore.dev/beta/errs"
	"github.com/google/uuid"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

type CreateBillRequest struct {
	CustomerID  string `json:"customer_id"`
	PeriodStart string `json:"period_start"`
}

type CreateBillResponse struct {
	BillID string `json:"bill_id"`
}

//encore:api public method=POST path=/bills
func (s *Service) CreateBill(ctx context.Context, req *CreateBillRequest) (*CreateBillResponse, error) {
	if req.CustomerID == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("customer_id is required").Err()
	}
	if req.PeriodStart == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("period_start is required").Err()
	}

	// Derive a deterministic UUID from customer + period so CreateBill is idempotent:
	// the same inputs always produce the same bill_id and hit the same workflow ID.
	billID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(req.CustomerID+"/"+req.PeriodStart)).String()
	input := BillInput{
		BillID:      billID,
		CustomerID:  req.CustomerID,
		PeriodStart: req.PeriodStart,
	}

	_, err := s.tc.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       wfID(billID),
		TaskQueue:                TaskQueue,
		WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
	}, BillWorkflow, input)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to create bill").Err()
	}
	return &CreateBillResponse{BillID: billID}, nil
}

type AddLineItemRequest struct {
	ClientItemID string   `json:"client_item_id,omitempty"`
	Description  string   `json:"description"`
	Amount       int64    `json:"amount"`
	Currency     Currency `json:"currency"`
}

type AddLineItemResponse struct {
	ItemID string `json:"item_id"`
}

//encore:api public method=POST path=/bills/:billID/items
func (s *Service) AddLineItem(ctx context.Context, billID string, req *AddLineItemRequest) (*AddLineItemResponse, error) {
	if req.Currency != USD && req.Currency != GEL {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("currency must be USD or GEL").Err()
	}
	if req.Amount <= 0 {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("amount must be positive").Err()
	}

	itemID := req.ClientItemID
	if itemID == "" {
		itemID = uuid.New().String()
	}

	sig := AddLineItemSignal{
		Item: LineItem{
			ID:          itemID,
			Description: req.Description,
			Amount:      Money{Amount: req.Amount, Currency: req.Currency},
			AddedAt:     time.Now(),
		},
	}

	if err := s.tc.SignalWorkflow(ctx, wfID(billID), "", SignalAddLineItem, sig); err != nil {
		return nil, billNotFoundOrClosed(billID)
	}
	return &AddLineItemResponse{ItemID: itemID}, nil
}

//encore:api public method=POST path=/bills/:billID/close
func (s *Service) CloseBill(ctx context.Context, billID string) (*BillState, error) {
	if err := s.tc.SignalWorkflow(ctx, wfID(billID), "", SignalCloseBill, CloseBillSignal{}); err != nil {
		return nil, billNotFoundOrClosed(billID)
	}

	run := s.tc.GetWorkflow(ctx, wfID(billID), "")
	var state BillState
	if err := run.Get(ctx, &state); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to retrieve final bill state").Err()
	}
	return &state, nil
}

//encore:api public method=GET path=/bills/:billID
func (s *Service) GetBill(ctx context.Context, billID string) (*BillState, error) {
	desc, err := s.tc.DescribeWorkflowExecution(ctx, wfID(billID), "")
	if err != nil {
		return nil, billNotFoundOrClosed(billID)
	}

	if desc.WorkflowExecutionInfo.Status == enums.WORKFLOW_EXECUTION_STATUS_RUNNING {
		qr, err := s.tc.QueryWorkflow(ctx, wfID(billID), "", QueryGetBill)
		if err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("failed to query bill state").Err()
		}
		var state BillState
		if err := qr.Get(&state); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("failed to decode bill state").Err()
		}
		return &state, nil
	}

	run := s.tc.GetWorkflow(ctx, wfID(billID), "")
	var state BillState
	if err := run.Get(ctx, &state); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to retrieve bill result").Err()
	}
	return &state, nil
}

func wfID(billID string) string {
	return fmt.Sprintf("bill:%s", billID)
}

func billNotFoundOrClosed(billID string) error {
	return errs.B().
		Code(errs.NotFound).
		Msg(fmt.Sprintf("bill %s not found or already closed", billID)).
		Err()
}
