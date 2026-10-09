package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/ai"
	"github.com/hemanth-shankar/expense-agent/internal/audit"
	"github.com/hemanth-shankar/expense-agent/internal/clarification"
	"github.com/hemanth-shankar/expense-agent/internal/duplicate"
	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/policy"
	"github.com/hemanth-shankar/expense-agent/internal/validation"
)

const (
	primaryModel  = "qwen3-vl:4b-instruct"
	fallbackModel = "qwen3-vl:8b-instruct"
)

func main() {

	startedAt :=
		time.Now().UTC()

	if len(os.Args) != 2 {

		log.Fatalf(
			"usage: go run ./cmd/server <receipt-image>",
		)
	}

	imagePath :=
		os.Args[1]

	imageData, err :=
		os.ReadFile(imagePath)

	if err != nil {

		log.Fatalf(
			"failed to read receipt image: %v",
			err,
		)
	}

	receiptHash :=
		calculateSHA256(
			imageData,
		)

	runID :=
		generateRunID()

	employee :=
		models.Employee{
			ID:         "EMP10042",
			Name:       "Ananya Rao",
			Grade:      "B2",
			Department: "Engineering",
			Location:   "Bengaluru",
		}

	fmt.Println("Analyzing receipt...")
	fmt.Println("File:", imagePath)
	fmt.Println("Run ID:", runID)

	fmt.Println()
	fmt.Println("Employee:", employee.Name)
	fmt.Println("Employee ID:", employee.ID)
	fmt.Println("Grade:", employee.Grade)

	client :=
		ai.NewOllamaClient()

	//
	// STEP 1
	// PRIMARY EXTRACTION
	//
	fmt.Println()

	fmt.Println(
		"Running primary extraction with Qwen3-VL 4B...",
	)

	extraction, err :=
		client.ExtractReceipt(
			context.Background(),
			imageData,
		)

	if err != nil {
		log.Fatal(err)
	}

	validation.NormalizeReceipt(
		extraction,
	)

	initialExtraction :=
		cloneExtraction(
			extraction,
		)

	initialValidation :=
		validation.ValidateReceipt(
			extraction,
		)

	validationResult :=
		initialValidation

	finalModel :=
		primaryModel

	fallbackUsed :=
		false

	//
	// STEP 2
	// FALLBACK MODEL
	//
	if !validationResult.Valid {

		fmt.Println()

		fmt.Println(
			"Primary extraction failed validation.",
		)

		printValidationIssues(
			validationResult,
		)

		fmt.Println()

		fmt.Println(
			"Escalating to Qwen3-VL 8B for fresh extraction...",
		)

		fallbackExtraction, fallbackErr :=
			client.ExtractReceiptWithFallback(
				context.Background(),
				imageData,
			)

		if fallbackErr != nil {

			log.Printf(
				"Fallback extraction failed: %v",
				fallbackErr,
			)

		} else {

			fallbackUsed =
				true

			validation.NormalizeReceipt(
				fallbackExtraction,
			)

			fallbackValidation :=
				validation.ValidateReceipt(
					fallbackExtraction,
				)

			extraction =
				fallbackExtraction

			validationResult =
				fallbackValidation

			finalModel =
				fallbackModel
		}
	}

	printFinalExtraction(
		extraction,
		finalModel,
	)

	printFinalValidation(
		validationResult,
	)

	var duplicateResult *duplicate.Result

	var clarificationResult *clarification.Result

	var policyDecision *policy.Decision

	finalStatus :=
		"HUMAN_REVIEW_REQUIRED"

	//
	// STEP 3
	// CONTINUE ONLY IF RECEIPT IS VALID
	//
	if validationResult.Valid {

		fmt.Println()

		fmt.Println(
			"STATUS: RECEIPT VALIDATED",
		)

		fmt.Println(
			"The extracted receipt passed deterministic validation.",
		)

		//
		// STEP 4
		// DUPLICATE DETECTION
		//
		dupResult, dupErr :=
			duplicate.Detect(
				"data/audit",
				receiptHash,
				extraction,
			)

		if dupErr != nil {

			log.Printf(
				"duplicate detection failed: %v",
				dupErr,
			)

			finalStatus =
				"HUMAN_REVIEW_REQUIRED"

		} else {

			duplicateResult =
				&dupResult

			printDuplicateResult(
				dupResult,
			)

			//
			// STEP 5
			// EXACT DUPLICATE
			//
			if dupResult.IsDuplicate &&
				dupResult.MatchType ==
					duplicate.MatchExact {

				finalStatus =
					"DUPLICATE_DETECTED"

				fmt.Println()

				fmt.Println(
					"Exact duplicate detected.",
				)

				fmt.Println(
					"Claim processing has been stopped.",
				)

			} else {

				//
				// STEP 6
				// CLARIFICATION ENGINE
				//
				clarify :=
					clarification.Evaluate(
						extraction,
						validationResult,
						duplicateResult,
					)

				clarificationResult =
					&clarify

				if clarify.Required {

					finalStatus =
						"CLARIFICATION_REQUIRED"

					printClarification(
						clarify,
					)

				} else {

					//
					// STEP 7
					// POLICY ENGINE
					//
					companyPolicy :=
						policy.DemoUSDPolicy()

					decision :=
						policy.Evaluate(
							extraction,
							employee,
							companyPolicy,
						)

					policyDecision =
						&decision

					finalStatus =
						decision.Status

					printPolicyDecision(
						decision,
					)
				}
			}
		}

	} else {

		fmt.Println()

		fmt.Println(
			"STATUS: HUMAN REVIEW REQUIRED",
		)

		fmt.Println(
			"The receipt still contains validation errors after AI escalation.",
		)
	}

	//
	// STEP 8
	// AUDIT TRAIL
	//
	auditRecord :=
		audit.Record{
			RunID: runID,

			ReceiptPath: imagePath,

			ReceiptSHA256: receiptHash,

			StartedAt: startedAt,

			CompletedAt: time.Now().UTC(),

			PrimaryModel: primaryModel,

			FallbackUsed: fallbackUsed,

			InitialExtraction: initialExtraction,

			InitialValidation: initialValidation,

			FinalExtraction: cloneExtraction(
				extraction,
			),

			FinalValidation: validationResult,

			Employee: employee,

			DuplicateResult: duplicateResult,

			ClarificationResult: clarificationResult,

			PolicyDecision: policyDecision,

			FinalStatus: finalStatus,
		}

	if fallbackUsed {

		auditRecord.FallbackModel =
			fallbackModel
	}

	auditPath, err :=
		audit.Save(
			"data/audit",
			auditRecord,
		)

	if err != nil {

		log.Printf(
			"failed to save audit record: %v",
			err,
		)

	} else {

		fmt.Println()

		fmt.Println(
			"Audit record saved:",
			auditPath,
		)
	}

	fmt.Println()

	fmt.Println(
		"FINAL CLAIM STATUS:",
		finalStatus,
	)
}

func calculateSHA256(
	data []byte,
) string {

	hash :=
		sha256.Sum256(data)

	return hex.EncodeToString(
		hash[:],
	)
}

func generateRunID() string {

	return time.Now().
		UTC().
		Format(
			"20060102T150405.000000000Z",
		)
}

func cloneExtraction(
	source *models.ReceiptExtraction,
) *models.ReceiptExtraction {

	if source == nil {
		return nil
	}

	copyValue :=
		*source

	copyValue.LineItems =
		append(
			[]models.ReceiptLineItem(nil),
			source.LineItems...,
		)

	copyValue.Warnings =
		append(
			[]string(nil),
			source.Warnings...,
		)

	return &copyValue
}

func printFinalExtraction(
	extraction *models.ReceiptExtraction,
	model string,
) {

	output, err :=
		json.MarshalIndent(
			extraction,
			"",
			"  ",
		)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Println("Final Receipt Extraction:")
	fmt.Println("Model:", model)
	fmt.Println(string(output))
}

func printFinalValidation(
	result validation.ReceiptValidationResult,
) {

	output, err :=
		json.MarshalIndent(
			result,
			"",
			"  ",
		)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Println("Final Validation Result:")
	fmt.Println(string(output))
}

func printDuplicateResult(
	result duplicate.Result,
) {

	output, err :=
		json.MarshalIndent(
			result,
			"",
			"  ",
		)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Println("Duplicate Detection Result:")
	fmt.Println(string(output))
}

func printClarification(
	result clarification.Result,
) {

	output, err :=
		json.MarshalIndent(
			result,
			"",
			"  ",
		)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()

	fmt.Println(
		"Clarification Required:",
	)

	fmt.Println(
		string(output),
	)

	fmt.Println()

	fmt.Println(
		"Questions for Employee:",
	)

	for i, question := range result.Questions {

		fmt.Printf(
			"%d. %s\n",
			i+1,
			question.Question,
		)

		fmt.Printf(
			"   Reason: %s\n",
			question.Reason,
		)
	}
}

func printPolicyDecision(
	decision policy.Decision,
) {

	output, err :=
		json.MarshalIndent(
			decision,
			"",
			"  ",
		)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Println("Reimbursement Policy Decision:")
	fmt.Println(string(output))

	fmt.Println()

	fmt.Println(
		"CLAIM STATUS:",
		decision.Status,
	)

	fmt.Printf(
		"Claimed Amount: %.2f %s\n",
		decision.ClaimedAmount,
		decision.Currency,
	)

	fmt.Printf(
		"Reimbursable Amount: %.2f %s\n",
		decision.ReimbursableAmount,
		decision.Currency,
	)

	if decision.ExcessAmount > 0 {

		fmt.Printf(
			"Excess Amount: %.2f %s\n",
			decision.ExcessAmount,
			decision.Currency,
		)
	}

	fmt.Println(
		"Reason:",
		decision.Reason,
	)
}

func printValidationIssues(
	result validation.ReceiptValidationResult,
) {

	for _, issue := range result.Issues {

		fmt.Printf(
			"- [%s] %s: %s\n",
			issue.Severity,
			issue.Code,
			issue.Message,
		)
	}
}
