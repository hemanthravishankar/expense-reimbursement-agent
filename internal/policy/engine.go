package policy

import (
	"fmt"
	"math"

	"github.com/hemanth-shankar/expense-agent/internal/models"
)

const (
	StatusApproved         = "APPROVED"
	StatusLimitExceeded    = "LIMIT_EXCEEDED"
	StatusNotAllowed       = "NOT_ALLOWED"
	StatusManualReview     = "MANUAL_REVIEW"
	StatusCurrencyMismatch = "CURRENCY_MISMATCH"
)

type CategoryRule struct {
	Allowed   bool    `json:"allowed"`
	MaxAmount float64 `json:"max_amount"`
}

type Policy struct {
	Name     string                             `json:"name"`
	Currency string                             `json:"currency"`
	Limits   map[string]map[string]CategoryRule `json:"limits"`
}

type Decision struct {
	Status             string  `json:"status"`
	EmployeeID         string  `json:"employee_id"`
	EmployeeGrade      string  `json:"employee_grade"`
	Category           string  `json:"category"`
	Currency           string  `json:"currency"`
	ClaimedAmount      float64 `json:"claimed_amount"`
	PolicyLimit        float64 `json:"policy_limit"`
	ReimbursableAmount float64 `json:"reimbursable_amount"`
	ExcessAmount       float64 `json:"excess_amount"`
	Reason             string  `json:"reason"`
}

func DemoUSDPolicy() Policy {
	return Policy{
		Name:     "NovaTech Demo Expense Policy",
		Currency: "USD",

		Limits: map[string]map[string]CategoryRule{
			"B2": {
				"Meals": {
					Allowed:   true,
					MaxAmount: 50,
				},
				"Transportation": {
					Allowed:   true,
					MaxAmount: 100,
				},
				"Accommodation": {
					Allowed:   true,
					MaxAmount: 250,
				},
				"Travel": {
					Allowed:   true,
					MaxAmount: 500,
				},
				"Office Expense": {
					Allowed:   true,
					MaxAmount: 150,
				},
				"Internet": {
					Allowed:   true,
					MaxAmount: 60,
				},
				"Mobile": {
					Allowed:   true,
					MaxAmount: 60,
				},
				"Entertainment": {
					Allowed:   true,
					MaxAmount: 75,
				},
			},
		},
	}
}

func Evaluate(
	receipt *models.ReceiptExtraction,
	employee models.Employee,
	companyPolicy Policy,
) Decision {

	decision := Decision{
		EmployeeID:    employee.ID,
		EmployeeGrade: employee.Grade,
		Category:      receipt.CategorySuggestion,
		Currency:      receipt.Currency,
		ClaimedAmount: roundMoney(receipt.TotalAmount),
	}

	if receipt.Currency != companyPolicy.Currency {
		decision.Status = StatusCurrencyMismatch
		decision.Reason = fmt.Sprintf(
			"Expense currency %s does not match policy currency %s.",
			receipt.Currency,
			companyPolicy.Currency,
		)

		return decision
	}

	gradeRules, exists :=
		companyPolicy.Limits[employee.Grade]

	if !exists {
		decision.Status = StatusManualReview
		decision.Reason = fmt.Sprintf(
			"No reimbursement policy is configured for employee grade %s.",
			employee.Grade,
		)

		return decision
	}

	rule, exists :=
		gradeRules[receipt.CategorySuggestion]

	if !exists {
		decision.Status = StatusManualReview
		decision.Reason = fmt.Sprintf(
			"No reimbursement rule exists for category %s.",
			receipt.CategorySuggestion,
		)

		return decision
	}

	if !rule.Allowed {
		decision.Status = StatusNotAllowed
		decision.Reason = fmt.Sprintf(
			"%s expenses are not reimbursable for grade %s.",
			receipt.CategorySuggestion,
			employee.Grade,
		)

		return decision
	}

	decision.PolicyLimit =
		roundMoney(rule.MaxAmount)

	if receipt.TotalAmount <= rule.MaxAmount {

		decision.Status =
			StatusApproved

		decision.ReimbursableAmount =
			roundMoney(receipt.TotalAmount)

		decision.Reason =
			"Expense is within the applicable reimbursement limit."

		return decision
	}

	decision.Status =
		StatusLimitExceeded

	decision.ReimbursableAmount =
		roundMoney(rule.MaxAmount)

	decision.ExcessAmount =
		roundMoney(
			receipt.TotalAmount -
				rule.MaxAmount,
		)

	decision.Reason = fmt.Sprintf(
		"Claimed amount %.2f exceeds the %.2f limit for %s expenses.",
		receipt.TotalAmount,
		rule.MaxAmount,
		receipt.CategorySuggestion,
	)

	return decision
}

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}
