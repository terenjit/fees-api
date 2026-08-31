package bills

import "time"

type Currency string

const (
	USD Currency = "USD"
	GEL Currency = "GEL"
)

type BillStatus string

const (
	StatusOpen   BillStatus = "open"
	StatusClosed BillStatus = "closed"
)

type Money struct {
	Amount   int64    `json:"amount"`
	Currency Currency `json:"currency"`
}

type LineItem struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Amount      Money     `json:"amount"`
	AddedAt     time.Time `json:"added_at"`
}

type BillInput struct {
	BillID      string `json:"bill_id"`
	CustomerID  string `json:"customer_id"`
	PeriodStart string `json:"period_start"`
}

type BillState struct {
	ID          string             `json:"id"`
	CustomerID  string             `json:"customer_id"`
	PeriodStart string             `json:"period_start"`
	Status      BillStatus         `json:"status"`
	LineItems   []LineItem         `json:"line_items"`
	Totals      map[Currency]int64 `json:"totals"`
	CreatedAt   time.Time          `json:"created_at"`
	ClosedAt    *time.Time         `json:"closed_at,omitempty"`
	seenItemIDs map[string]bool
}
