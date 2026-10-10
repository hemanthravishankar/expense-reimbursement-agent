package models

import "time"

const (
	ReportStatusProcessing        = "PROCESSING"
	ReportStatusCompleted         = "COMPLETED"
	ReportStatusActionRequired    = "ACTION_REQUIRED"
	ReportStatusReviewRequired    = "REVIEW_REQUIRED"
	ReportStatusPartiallyRejected = "PARTIALLY_REJECTED"
	ReportStatusRejected          = "REJECTED"
)

type ExpenseReport struct {
	ReportID string `json:"report_id"`

	Title   string `json:"title"`
	Purpose string `json:"purpose"`

	Employee Employee `json:"employee"`

	Status string `json:"status"`

	ItemCount int `json:"item_count"`

	TotalClaimed      float64 `json:"total_claimed"`
	TotalReimbursable float64 `json:"total_reimbursable"`

	Items []ExpenseReportItem `json:"items,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ExpenseReportItem struct {
	ClaimID string `json:"claim_id"`

	Position int `json:"position"`

	Status string `json:"status"`

	Merchant string `json:"merchant,omitempty"`
	Category string `json:"category,omitempty"`
	Currency string `json:"currency,omitempty"`

	ClaimedAmount      float64 `json:"claimed_amount"`
	ReimbursableAmount float64 `json:"reimbursable_amount"`

	DocumentName string `json:"document_name,omitempty"`
}
