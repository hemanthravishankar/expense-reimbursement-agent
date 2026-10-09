package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

type fakeProcessor struct {
	called bool
}

func (f *fakeProcessor) ProcessReceipt(
	ctx context.Context,
	receiptPath string,
	rawDocument []byte,
	employee models.Employee,
) (*workflow.Result, error) {

	f.called = true

	return &workflow.Result{
		RunID: "test-run",

		ReceiptPath: receiptPath,

		DocumentMIMEType: "image/jpeg",

		PageCount: 1,

		Employee: employee,

		FinalStatus: "APPROVED",
	}, nil
}

func TestHealth(
	t *testing.T,
) {

	processor :=
		&fakeProcessor{}

	handler :=
		NewHandler(
			processor,
		)

	mux :=
		http.NewServeMux()

	handler.RegisterRoutes(
		mux,
	)

	request :=
		httptest.NewRequest(
			http.MethodGet,
			"/healthz",
			nil,
		)

	response :=
		httptest.NewRecorder()

	mux.ServeHTTP(
		response,
		request,
	)

	if response.Code !=
		http.StatusOK {

		t.Fatalf(
			"expected 200, got %d",
			response.Code,
		)
	}
}

func TestAnalyzeExpense(
	t *testing.T,
) {

	processor :=
		&fakeProcessor{}

	handler :=
		NewHandler(
			processor,
		)

	mux :=
		http.NewServeMux()

	handler.RegisterRoutes(
		mux,
	)

	var body bytes.Buffer

	writer :=
		multipart.NewWriter(
			&body,
		)

	fileWriter, err :=
		writer.CreateFormFile(
			"receipt",
			"receipt.jpg",
		)

	if err != nil {
		t.Fatal(err)
	}

	if _, err :=
		fileWriter.Write(
			[]byte(
				"fake-image-data",
			),
		); err != nil {

		t.Fatal(err)
	}

	fields :=
		map[string]string{
			"employee_id": "EMP10042",

			"employee_name": "Ananya Rao",

			"employee_grade": "B2",

			"department": "Engineering",

			"location": "Bengaluru",
		}

	for key, value := range fields {

		if err :=
			writer.WriteField(
				key,
				value,
			); err != nil {

			t.Fatal(err)
		}
	}

	if err :=
		writer.Close(); err != nil {

		t.Fatal(err)
	}

	request :=
		httptest.NewRequest(
			http.MethodPost,
			"/api/v1/expenses/analyze",
			&body,
		)

	request.Header.Set(
		"Content-Type",
		writer.FormDataContentType(),
	)

	response :=
		httptest.NewRecorder()

	mux.ServeHTTP(
		response,
		request,
	)

	if response.Code !=
		http.StatusOK {

		t.Fatalf(
			"expected 200, got %d: %s",
			response.Code,
			response.Body.String(),
		)
	}

	if !processor.called {

		t.Fatal(
			"expected workflow processor to be called",
		)
	}

	var result workflow.Result

	if err :=
		json.Unmarshal(
			response.Body.Bytes(),
			&result,
		); err != nil {

		t.Fatal(err)
	}

	if result.FinalStatus !=
		"APPROVED" {

		t.Fatalf(
			"expected APPROVED, got %s",
			result.FinalStatus,
		)
	}
}

func TestAnalyzeMissingReceipt(
	t *testing.T,
) {

	processor :=
		&fakeProcessor{}

	handler :=
		NewHandler(
			processor,
		)

	mux :=
		http.NewServeMux()

	handler.RegisterRoutes(
		mux,
	)

	var body bytes.Buffer

	writer :=
		multipart.NewWriter(
			&body,
		)

	_ = writer.WriteField(
		"employee_id",
		"EMP10042",
	)

	_ = writer.WriteField(
		"employee_name",
		"Ananya Rao",
	)

	_ = writer.WriteField(
		"employee_grade",
		"B2",
	)

	if err :=
		writer.Close(); err != nil {

		t.Fatal(err)
	}

	request :=
		httptest.NewRequest(
			http.MethodPost,
			"/api/v1/expenses/analyze",
			&body,
		)

	request.Header.Set(
		"Content-Type",
		writer.FormDataContentType(),
	)

	response :=
		httptest.NewRecorder()

	mux.ServeHTTP(
		response,
		request,
	)

	if response.Code !=
		http.StatusBadRequest {

		t.Fatalf(
			"expected 400, got %d",
			response.Code,
		)
	}
}
