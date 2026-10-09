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
	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/validation"
)

const (
	primaryModel  = "qwen3-vl:4b-instruct"
	fallbackModel = "qwen3-vl:8b-instruct"
)

func main() {

	startedAt := time.Now().UTC()

	if len(os.Args) != 2 {
		log.Fatalf(
			"usage: go run ./cmd/server <receipt-image>",
		)
	}

	imagePath := os.Args[1]

	imageData, err :=
		os.ReadFile(imagePath)

	if err != nil {
		log.Fatalf(
			"failed to read receipt image: %v",
			err,
		)
	}

	receiptHash :=
		calculateSHA256(imageData)

	runID :=
		generateRunID()

	fmt.Println(
		"Analyzing receipt...",
	)

	fmt.Println(
		"File:",
		imagePath,
	)

	fmt.Println(
		"Run ID:",
		runID,
	)

	client :=
		ai.NewOllamaClient()

	fmt.Println()

	fmt.Println(
		"Running primary extraction with Qwen3-VL 4B...",
	)

	//
	// PRIMARY EXTRACTION
	//
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

	//
	// Preserve initial extraction for audit.
	//
	initialExtraction :=
		cloneExtraction(extraction)

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

	finalStatus :=
		"HUMAN_REVIEW_REQUIRED"

	if validationResult.Valid {
		finalStatus =
			"VALIDATED"
	}

	//
	// PRINT FINAL RESULT
	//
	printFinalExtraction(
		extraction,
		finalModel,
	)

	printFinalValidation(
		validationResult,
	)

	fmt.Println()

	if validationResult.Valid {

		fmt.Println(
			"STATUS: RECEIPT VALIDATED",
		)

		fmt.Println(
			"The extracted receipt passed deterministic validation.",
		)

	} else {

		fmt.Println(
			"STATUS: HUMAN REVIEW REQUIRED",
		)

		fmt.Println(
			"The receipt still contains validation errors after AI escalation.",
		)
	}

	//
	// AUDIT TRAIL
	//
	auditRecord :=
		audit.Record{
			RunID:         runID,
			ReceiptPath:   imagePath,
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

	fmt.Println(
		"Final Receipt Extraction:",
	)

	fmt.Println(
		"Model:",
		model,
	)

	fmt.Println(
		string(output),
	)
}

func printFinalValidation(
	result validation.ReceiptValidationResult,
) {

	validationJSON, err :=
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
		"Final Validation Result:",
	)

	fmt.Println(
		string(validationJSON),
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
