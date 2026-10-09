package models

import "time"

type Expense struct {
	ID                    string    `json:"id"`
	EmployeeID            string    `json:"employee_id"`
	Merchant              string    `json:"merchant"`
	ExpenseDate           time.Time `json:"expense_date"`
	Amount                float64   `json:"amount"`
	Currency              string    `json:"currency"`
	Category              string    `json:"category"`
	Description           string    `json:"description"`
	ReceiptID             string    `json:"receipt_id"`
	ExtractionConfidence  float64   `json:"extraction_confidence"`
	PolicyStatus          string    `json:"policy_status"`
	ReimbursableAmount    float64   `json:"reimbursable_amount"`
	RequiresClarification bool      `json:"requires_clarification"`
	PossibleDuplicate     bool      `json:"possible_duplicate"`
}
