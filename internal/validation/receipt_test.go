package validation

import (
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/models"
)

func TestValidateReceiptDetectsLineItemMismatch(t *testing.T) {
	receipt := models.ReceiptExtraction{
		Merchant:    "FourthStreetMill",
		ExpenseDate: "2018-09-29",
		Subtotal:    49.50,
		TaxAmount:   4.70,
		TotalAmount: 54.20,
		Currency:    "$",
		Confidence:  0.98,
		LineItems: []models.ReceiptLineItem{
			{Name: "Lamb Burger", Quantity: 1, Amount: 13},
			{Name: "Kids Grilled Cheese", Quantity: 1, Amount: 5},
			{Name: "Kids Fries", Quantity: 1, Amount: 3},
			{Name: "Sweet Potato Fries", Quantity: 1, Amount: 8},
			{Name: "Kids Cheeseburger", Quantity: 1, Amount: 7},
			{Name: "Sub sourdough", Quantity: 1, Amount: 10},
			{Name: "Beef Burger", Quantity: 1, Amount: 10},
			{Name: "Sides", Quantity: 1, Amount: 3.50},
		},
	}

	NormalizeReceipt(&receipt)

	if receipt.Currency != "USD" {
		t.Fatalf(
			"expected currency USD, got %s",
			receipt.Currency,
		)
	}

	result := ValidateReceipt(&receipt)

	if result.Valid {
		t.Fatal(
			"expected receipt validation to fail",
		)
	}

	if result.LineItemSum != 59.50 {
		t.Fatalf(
			"expected line item sum 59.50, got %.2f",
			result.LineItemSum,
		)
	}

	foundMismatch := false

	for _, issue := range result.Issues {
		if issue.Code == "LINE_ITEM_SUM_MISMATCH" {
			foundMismatch = true
		}
	}

	if !foundMismatch {
		t.Fatal(
			"expected LINE_ITEM_SUM_MISMATCH issue",
		)
	}
}

func TestValidateCorrectReceipt(t *testing.T) {
	receipt := models.ReceiptExtraction{
		Merchant:    "FourthStreetMill",
		ExpenseDate: "2018-09-29",
		Subtotal:    49.50,
		TaxAmount:   4.70,
		TotalAmount: 54.20,
		Currency:    "USD",
		Confidence:  0.98,
		LineItems: []models.ReceiptLineItem{
			{Name: "Lamb Burger", Quantity: 1, Amount: 13},
			{Name: "Kids Grilled Cheese", Quantity: 1, Amount: 5},
			{Name: "Kids Fries", Quantity: 1, Amount: 3},
			{Name: "Sweet Potato Fries", Quantity: 1, Amount: 8},
			{Name: "Kids Cheeseburger", Quantity: 1, Amount: 7},
			{Name: "Beef Burger", Quantity: 1, Amount: 10},
			{Name: "Sides", Quantity: 1, Amount: 3.50},
		},
	}

	result := ValidateReceipt(&receipt)

	if !result.Valid {
		t.Fatalf(
			"expected valid receipt, got issues: %+v",
			result.Issues,
		)
	}
}
