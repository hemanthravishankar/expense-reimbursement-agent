package validation

import (
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/models"
)

func TestTransportationReconciliationRepairsRideBreakdown(t *testing.T) {
	receipt := &models.ReceiptExtraction{
		DocumentType:       "receipt",
		Merchant:           "Uber",
		ExpenseDate:        "2024-02-12",
		Subtotal:           14.50,
		TaxAmount:          0,
		ServiceCharge:      0,
		TipAmount:          0,
		DiscountAmount:     0,
		OtherFees:          1.50,
		TotalAmount:        26.00,
		Currency:           "USD",
		ReceiptNumber:      "UBR123456789",
		PaymentMethod:      "Visa",
		CategorySuggestion: "Transportation",
		LineItems: []models.ReceiptLineItem{
			{
				Name:     "Base Fare",
				Quantity: 1,
				Amount:   14.50,
			},
			{
				Name:     "Time (18 min)",
				Quantity: 1,
				Amount:   3.20,
			},
			{
				Name:     "Distance (12.4 km)",
				Quantity: 1,
				Amount:   4.80,
			},
			{
				Name:     "Airport Fee",
				Quantity: 1,
				Amount:   2.00,
			},
			{
				Name:     "Taxes & Other Fees",
				Quantity: 1,
				Amount:   1.50,
			},
		},
		Confidence: 0.98,
	}

	result :=
		ValidateReceipt(
			receipt,
		)

	if !result.Valid {
		t.Fatalf(
			"expected reconciled transportation receipt to validate, got issues: %+v",
			result.Issues,
		)
	}

	if !moneyEqual(
		receipt.Subtotal,
		22.50,
	) {
		t.Fatalf(
			"expected subtotal 22.50, got %.2f",
			receipt.Subtotal,
		)
	}

	if !moneyEqual(
		receipt.OtherFees,
		3.50,
	) {
		t.Fatalf(
			"expected other fees 3.50, got %.2f",
			receipt.OtherFees,
		)
	}

	if !moneyEqual(
		receipt.TaxAmount,
		0,
	) {
		t.Fatalf(
			"expected tax 0.00 for combined tax-and-fee row, got %.2f",
			receipt.TaxAmount,
		)
	}

	if len(receipt.LineItems) != 3 {
		t.Fatalf(
			"expected 3 subtotal line items after reconciliation, got %d",
			len(receipt.LineItems),
		)
	}

	if !moneyEqual(
		result.LineItemSum,
		22.50,
	) {
		t.Fatalf(
			"expected line item sum 22.50, got %.2f",
			result.LineItemSum,
		)
	}
}

func TestTransportationReconciliationLeavesAlreadyValidReceiptUnchanged(t *testing.T) {
	receipt := &models.ReceiptExtraction{
		DocumentType:       "receipt",
		Merchant:           "Ride Co",
		ExpenseDate:        "2024-02-12",
		Subtotal:           22.50,
		TaxAmount:          0,
		ServiceCharge:      0,
		TipAmount:          0,
		DiscountAmount:     0,
		OtherFees:          3.50,
		TotalAmount:        26.00,
		Currency:           "USD",
		ReceiptNumber:      "RIDE-001",
		PaymentMethod:      "Visa",
		CategorySuggestion: "Transportation",
		LineItems: []models.ReceiptLineItem{
			{
				Name:     "Base Fare",
				Quantity: 1,
				Amount:   14.50,
			},
			{
				Name:     "Time",
				Quantity: 1,
				Amount:   3.20,
			},
			{
				Name:     "Distance",
				Quantity: 1,
				Amount:   4.80,
			},
		},
		Confidence: 0.98,
	}

	result :=
		ValidateReceipt(
			receipt,
		)

	if !result.Valid {
		t.Fatalf(
			"expected already-valid transportation receipt to remain valid, got issues: %+v",
			result.Issues,
		)
	}

	if len(receipt.Warnings) != 0 {
		t.Fatalf(
			"expected no reconciliation warning for an already-valid receipt, got %+v",
			receipt.Warnings,
		)
	}
}

func TestTransportationReconciliationDoesNotChangeMealReceipt(t *testing.T) {
	receipt := &models.ReceiptExtraction{
		DocumentType:       "receipt",
		Merchant:           "THE DAILY GRILL",
		ExpenseDate:        "2024-02-12",
		Subtotal:           48.00,
		TaxAmount:          2.40,
		ServiceCharge:      4.80,
		TipAmount:          0,
		DiscountAmount:     0,
		OtherFees:          0,
		TotalAmount:        55.20,
		Currency:           "USD",
		ReceiptNumber:      "78452",
		CategorySuggestion: "Meals",
		LineItems: []models.ReceiptLineItem{
			{
				Name:     "Grilled Chicken Salad",
				Quantity: 1,
				Amount:   18.00,
			},
			{
				Name:     "Pasta Alfredo",
				Quantity: 1,
				Amount:   16.00,
			},
			{
				Name:     "Iced Tea",
				Quantity: 2,
				Amount:   6.00,
			},
			{
				Name:     "Tiramisu",
				Quantity: 1,
				Amount:   8.00,
			},
		},
		Confidence: 0.98,
	}

	result :=
		ValidateReceipt(
			receipt,
		)

	if !result.Valid {
		t.Fatalf(
			"expected meal receipt to remain valid, got issues: %+v",
			result.Issues,
		)
	}

	if !moneyEqual(
		receipt.Subtotal,
		48.00,
	) {
		t.Fatalf(
			"expected meal subtotal to remain 48.00, got %.2f",
			receipt.Subtotal,
		)
	}

	if !moneyEqual(
		receipt.ServiceCharge,
		4.80,
	) {
		t.Fatalf(
			"expected service charge to remain 4.80, got %.2f",
			receipt.ServiceCharge,
		)
	}
}
