import { useState } from "react";

import {
  analyzeExpense,
  getClaimEvents,
  resolveClarification,
} from "./api";

import "./App.css";

const DUPLICATE_QUESTION =
  "CONFIRM_POSSIBLE_DUPLICATE";

const initialEmployee = {
  employee_id: "",
  employee_name: "",
  employee_grade: "B2",
  department: "",
  location: "",
};

function App() {
  const [employee, setEmployee] =
    useState(initialEmployee);

  const [file, setFile] =
    useState(null);

  const [result, setResult] =
    useState(null);

  const [events, setEvents] =
    useState([]);

  const [answer, setAnswer] =
    useState("");

  const [explanation, setExplanation] =
    useState("");

  const [processing, setProcessing] =
    useState(false);

  const [clarifying, setClarifying] =
    useState(false);

  const [error, setError] =
    useState("");

  const updateEmployee = (event) => {
    const { name, value } =
      event.target;

    setEmployee((current) => ({
      ...current,
      [name]: value,
    }));
  };

  const loadEvents =
    async (claimID) => {
      if (!claimID) {
        return;
      }

      try {
        const timeline =
          await getClaimEvents(
            claimID,
          );

        setEvents(
          Array.isArray(timeline)
            ? timeline
            : [],
        );
      } catch (eventError) {
        console.error(
          "Unable to load claim events:",
          eventError,
        );
      }
    };

  const submitExpense =
    async (event) => {
      event.preventDefault();

      setError("");
      setResult(null);
      setEvents([]);
      setAnswer("");
      setExplanation("");

      if (!file) {
        setError(
          "Please select a receipt or invoice.",
        );

        return;
      }

      const formData =
        new FormData();

      formData.append(
        "receipt",
        file,
      );

      Object.entries(
        employee,
      ).forEach(
        ([key, value]) => {
          formData.append(
            key,
            value.trim(),
          );
        },
      );

      try {
        setProcessing(true);

        const processed =
          await analyzeExpense(
            formData,
          );

        setResult(processed);

        await loadEvents(
          processed.run_id,
        );
      } catch (requestError) {
        setError(
          requestError.message,
        );
      } finally {
        setProcessing(false);
      }
    };

  const submitClarification =
    async (questionCode) => {
      setError("");

      if (!answer) {
        setError(
          "Please select Yes or No.",
        );

        return;
      }

      if (
        answer === "yes" &&
        !explanation.trim()
      ) {
        setError(
          "Please explain why this is a separate transaction.",
        );

        return;
      }

      try {
        setClarifying(true);

        const updated =
          await resolveClarification(
            result.run_id,
            {
              question_code:
                questionCode,

              answer,

              explanation:
                explanation.trim(),
            },
          );

        setResult(updated);

        await loadEvents(
          updated.run_id,
        );
      } catch (requestError) {
        setError(
          requestError.message,
        );
      } finally {
        setClarifying(false);
      }
    };

  const reset = () => {
    setEmployee(
      initialEmployee,
    );

    setFile(null);
    setResult(null);
    setEvents([]);
    setAnswer("");
    setExplanation("");
    setError("");
  };

  return (
    <div className="app-shell">
      <header className="topbar">
        <div>
          <div className="brand">
            Expense Agent
          </div>

          <div className="brand-subtitle">
            AI-assisted expense &
            reimbursement processing
          </div>
        </div>

        <div className="system-status">
          <span className="status-dot" />
          Agent online
        </div>
      </header>

      <main className="main-content">
        <section className="hero">
          <div>
            <div className="eyebrow">
              EXPENSE INTELLIGENCE
            </div>

            <h1>
              Submit an expense.
              <br />
              Let the agent do the rest.
            </h1>

            <p>
              The agent understands
              receipts, verifies its own
              extraction, detects
              duplicate submissions,
              requests clarification when
              necessary and applies
              reimbursement policy
              deterministically.
            </p>
          </div>

          <div className="pipeline-card">
            <PipelineItem
              number="01"
              title="Document"
              text="JPG, PNG or PDF"
            />

            <PipelineItem
              number="02"
              title="AI extraction"
              text="Qwen3-VL"
            />

            <PipelineItem
              number="03"
              title="Verification"
              text="Deterministic validation"
            />

            <PipelineItem
              number="04"
              title="Decision"
              text="Duplicate + policy"
            />
          </div>
        </section>

        <div className="workspace">
          <section className="panel">
            <div className="panel-heading">
              <div>
                <div className="section-number">
                  01
                </div>

                <h2>
                  Submit expense
                </h2>
              </div>
            </div>

            <form
              onSubmit={
                submitExpense
              }
            >
              <div className="form-grid">
                <Field
                  label="Employee ID"
                  name="employee_id"
                  value={
                    employee.employee_id
                  }
                  onChange={
                    updateEmployee
                  }
                  placeholder="EMP30025"
                  required
                />

                <Field
                  label="Employee name"
                  name="employee_name"
                  value={
                    employee.employee_name
                  }
                  onChange={
                    updateEmployee
                  }
                  placeholder="Priya Nair"
                  required
                />

                <Field
                  label="Grade"
                  name="employee_grade"
                  value={
                    employee.employee_grade
                  }
                  onChange={
                    updateEmployee
                  }
                  placeholder="B2"
                  required
                />

                <Field
                  label="Department"
                  name="department"
                  value={
                    employee.department
                  }
                  onChange={
                    updateEmployee
                  }
                  placeholder="Operations"
                />

                <Field
                  label="Location"
                  name="location"
                  value={
                    employee.location
                  }
                  onChange={
                    updateEmployee
                  }
                  placeholder="Bengaluru"
                />
              </div>

              <div className="upload-section">
                <label className="upload-box">
                  <input
                    type="file"
                    accept=".jpg,.jpeg,.png,.pdf,image/jpeg,image/png,application/pdf"
                    onChange={(event) =>
                      setFile(
                        event.target
                          .files?.[0] ||
                          null,
                      )
                    }
                  />

                  <div className="upload-icon">
                    ↑
                  </div>

                  <strong>
                    {file
                      ? file.name
                      : "Choose receipt or invoice"}
                  </strong>

                  <span>
                    JPG, PNG or PDF ·
                    maximum 20 MB
                  </span>

                  {file && (
                    <span className="file-meta">
                      {formatFileSize(
                        file.size,
                      )}
                    </span>
                  )}
                </label>
              </div>

              {error && (
                <div className="error-message">
                  {error}
                </div>
              )}

              <button
                className="primary-button"
                type="submit"
                disabled={
                  processing
                }
              >
                {processing ? (
                  <>
                    <span className="spinner" />
                    Agent is processing…
                  </>
                ) : (
                  "Analyze expense"
                )}
              </button>
            </form>
          </section>

          <section className="panel result-panel">
            <div className="panel-heading">
              <div>
                <div className="section-number">
                  02
                </div>

                <h2>
                  Agent decision
                </h2>
              </div>

              {result && (
                <StatusBadge
                  status={
                    result.final_status
                  }
                />
              )}
            </div>

            {!result ? (
              <EmptyResult />
            ) : (
              <ResultView
                result={result}
                events={events}
                answer={answer}
                setAnswer={
                  setAnswer
                }
                explanation={
                  explanation
                }
                setExplanation={
                  setExplanation
                }
                clarifying={
                  clarifying
                }
                onClarify={
                  submitClarification
                }
              />
            )}
          </section>
        </div>

        {result && (
          <div className="new-claim-row">
            <button
              type="button"
              className="secondary-button"
              onClick={reset}
            >
              Start new claim
            </button>
          </div>
        )}
      </main>

      <footer>
        Expense Agent · AI
        Innovation Lab
      </footer>
    </div>
  );
}

function ResultView({
  result,
  events,
  answer,
  setAnswer,
  explanation,
  setExplanation,
  clarifying,
  onClarify,
}) {
  const extraction =
    result.final_extraction;

  const validation =
    result.final_validation;

  const decision =
    result.policy_decision;

  const clarification =
    result.clarification_result;

  return (
    <div className="result-content">
      <div className="agent-summary">
        <InfoBox
          label="Claim"
          value={shortID(
            result.run_id,
          )}
        />

        <InfoBox
          label="Pages"
          value={
            result.page_count || 1
          }
        />

        <InfoBox
          label="Model"
          value={
            result.ai_processing_skipped
              ? "Skipped"
              : shortModelName(
                  result.final_model,
                )
          }
        />

        <InfoBox
          label="Validation"
          value={
            result.ai_processing_skipped
              ? "N/A"
              : validation?.valid
                ? "Passed"
                : "Failed"
          }
        />
      </div>

      <ProcessingTrace
        result={result}
      />

      {extraction && (
        <>
          <SectionTitle>
            Extracted expense
          </SectionTitle>

          <div className="detail-grid">
            <Detail
              label="Merchant"
              value={
                extraction.merchant
              }
            />

            <Detail
              label="Expense date"
              value={
                extraction.expense_date
              }
            />

            <Detail
              label="Category"
              value={
                extraction.category_suggestion
              }
            />

            <Detail
              label="Subtotal"
              value={formatMoney(
                extraction.subtotal,
                extraction.currency,
              )}
            />

            <Detail
              label="Tax"
              value={formatMoney(
                extraction.tax_amount,
                extraction.currency,
              )}
            />

            <Detail
              label="Total"
              value={formatMoney(
                extraction.total_amount,
                extraction.currency,
              )}
            />

            <Detail
              label="Receipt number"
              value={
                extraction.receipt_number ||
                "—"
              }
            />

            <Detail
              label="Payment method"
              value={
                extraction.payment_method ||
                "—"
              }
            />

            <Detail
              label="AI confidence"
              value={
                extraction.confidence
                  ? `${Math.round(
                      extraction.confidence *
                        100,
                    )}%`
                  : "—"
              }
            />
          </div>

          <LineItems
            extraction={
              extraction
            }
          />
        </>
      )}

      {!result.ai_processing_skipped &&
        validation && (
          <ValidationPanel
            validation={
              validation
            }
            currency={
              extraction?.currency
            }
          />
        )}

      {result.duplicate_result
        ?.is_duplicate && (
        <>
          <SectionTitle>
            Duplicate analysis
          </SectionTitle>

          <div className="duplicate-card">
            <div>
              <strong>
                {humanize(
                  result
                    .duplicate_result
                    .match_type,
                )}
              </strong>

              <p>
                {
                  result
                    .duplicate_result
                    .reason
                }
              </p>

              {result
                .duplicate_result
                .matched_run_id && (
                <small>
                  Matched claim:{" "}
                  {shortID(
                    result
                      .duplicate_result
                      .matched_run_id,
                  )}
                </small>
              )}
            </div>

            <div className="confidence-score">
              {Math.round(
                (result
                  .duplicate_result
                  .confidence || 0) *
                  100,
              )}
              %
            </div>
          </div>
        </>
      )}

      {clarification
        ?.required && (
        <ClarificationBox
          clarification={
            clarification
          }
          answer={answer}
          setAnswer={
            setAnswer
          }
          explanation={
            explanation
          }
          setExplanation={
            setExplanation
          }
          clarifying={
            clarifying
          }
          onClarify={
            onClarify
          }
        />
      )}

      {decision && (
        <>
          <SectionTitle>
            Reimbursement decision
          </SectionTitle>

          <div className="decision-card">
            <div>
              <span>
                Claimed
              </span>

              <strong>
                {formatMoney(
                  decision.claimed_amount,
                  decision.currency,
                )}
              </strong>
            </div>

            <div>
              <span>
                Policy limit
              </span>

              <strong>
                {formatMoney(
                  decision.policy_limit,
                  decision.currency,
                )}
              </strong>
            </div>

            <div>
              <span>
                Reimbursable
              </span>

              <strong>
                {formatMoney(
                  decision.reimbursable_amount,
                  decision.currency,
                )}
              </strong>
            </div>

            <div>
              <span>
                Excess
              </span>

              <strong>
                {formatMoney(
                  decision.excess_amount,
                  decision.currency,
                )}
              </strong>
            </div>
          </div>

          {decision.reason && (
            <div className="policy-reason">
              <strong>
                Policy reasoning
              </strong>

              <p>
                {decision.reason}
              </p>
            </div>
          )}
        </>
      )}

      {result
        .clarification_answers
        ?.length > 0 && (
        <>
          <SectionTitle>
            Clarification response
          </SectionTitle>

          {result.clarification_answers.map(
            (item, index) => (
              <div
                className="answered-card"
                key={`${item.question_code}-${index}`}
              >
                <div>
                  <strong>
                    {humanize(
                      item.question_code,
                    )}
                  </strong>

                  {item.explanation && (
                    <p>
                      {
                        item.explanation
                      }
                    </p>
                  )}
                </div>

                <StatusBadge
                  status={
                    item.answer ===
                    "yes"
                      ? "YES"
                      : "NO"
                  }
                />
              </div>
            ),
          )}
        </>
      )}

      {result.stored_document && (
        <>
          <SectionTitle>
            Evidence
          </SectionTitle>

          <div className="evidence-card">
            <EvidenceRow
              label="Original file"
              value={
                result
                  .stored_document
                  .original_name
              }
            />

            <EvidenceRow
              label="MIME type"
              value={
                result
                  .stored_document
                  .mime_type
              }
            />

            <EvidenceRow
              label="SHA-256"
              value={
                result
                  .stored_document
                  .sha256
              }
              mono
            />

            <EvidenceRow
              label="Storage key"
              value={
                result
                  .stored_document
                  .key
              }
              mono
            />
          </div>
        </>
      )}

      <SectionTitle>
        Claim timeline
      </SectionTitle>

      <Timeline
        events={events}
      />
    </div>
  );
}

function ProcessingTrace({
  result,
}) {
  return (
    <>
      <SectionTitle>
        Agent execution
      </SectionTitle>

      <div className="trace-grid">
        <TraceStep
          label="SHA-256"
          value="Checked"
          state="success"
        />

        <TraceStep
          label="AI inference"
          value={
            result.ai_processing_skipped
              ? "Skipped"
              : "Executed"
          }
          state={
            result.ai_processing_skipped
              ? "neutral"
              : "success"
          }
        />

        <TraceStep
          label="Fallback"
          value={
            result.fallback_used
              ? "Used"
              : "Not required"
          }
          state={
            result.fallback_used
              ? "warning"
              : "success"
          }
        />

        <TraceStep
          label="Policy"
          value={
            result.policy_decision
              ? "Evaluated"
              : result.final_status ===
                  "CLARIFICATION_REQUIRED"
                ? "Waiting"
                : "Not required"
          }
          state={
            result.policy_decision
              ? "success"
              : "neutral"
          }
        />
      </div>

      {result.fallback_used && (
        <div className="notice">
          <strong>
            Automatic model escalation
          </strong>

          <span>
            The primary extraction
            failed deterministic
            validation. The agent
            automatically retried the
            document using the larger
            vision model.
          </span>
        </div>
      )}

      {result.ai_processing_skipped && (
        <div className="notice warning">
          <strong>
            Expensive AI processing
            avoided
          </strong>

          <span>
            The document was identified
            as an exact duplicate using
            SHA-256 before vision-model
            inference.
          </span>
        </div>
      )}
    </>
  );
}

function LineItems({
  extraction,
}) {
  const items =
    extraction.line_items || [];

  if (!items.length) {
    return null;
  }

  return (
    <>
      <SectionTitle>
        Line items
      </SectionTitle>

      <div className="line-items-table">
        <div className="line-item header">
          <span>Item</span>
          <span>Qty</span>
          <span>Amount</span>
        </div>

        {items.map(
          (item, index) => (
            <div
              className="line-item"
              key={`${item.name}-${index}`}
            >
              <span>
                {item.name ||
                  "Unnamed item"}
              </span>

              <span>
                {item.quantity ||
                  "—"}
              </span>

              <strong>
                {formatMoney(
                  item.amount,
                  extraction.currency,
                )}
              </strong>
            </div>
          ),
        )}
      </div>
    </>
  );
}

function ValidationPanel({
  validation,
  currency,
}) {
  const issues =
    validation.issues || [];

  return (
    <>
      <SectionTitle>
        Deterministic validation
      </SectionTitle>

      <div
        className={
          validation.valid
            ? "validation-card valid"
            : "validation-card invalid"
        }
      >
        <div className="validation-heading">
          <div>
            <strong>
              {validation.valid
                ? "Validation passed"
                : "Validation failed"}
            </strong>

            <span>
              Go verification layer
            </span>
          </div>

          <div className="validation-state">
            {validation.valid
              ? "PASS"
              : "FAIL"}
          </div>
        </div>

        {validation.line_item_sum !==
          undefined && (
          <div className="validation-metric">
            <span>
              Calculated line-item sum
            </span>

            <strong>
              {formatMoney(
                validation.line_item_sum,
                currency,
              )}
            </strong>
          </div>
        )}

        {issues.length > 0 && (
          <div className="issue-list">
            {issues.map(
              (issue, index) => (
                <div
                  className="issue-row"
                  key={
                    issue.code ||
                    index
                  }
                >
                  <span
                    className={`issue-severity ${String(
                      issue.severity ||
                        "INFO",
                    ).toLowerCase()}`}
                  >
                    {issue.severity ||
                      "INFO"}
                  </span>

                  <div>
                    <strong>
                      {issue.code
                        ? humanize(
                            issue.code,
                          )
                        : "Validation issue"}
                    </strong>

                    <p>
                      {issue.message ||
                        issue.reason ||
                        JSON.stringify(
                          issue,
                        )}
                    </p>
                  </div>
                </div>
              ),
            )}
          </div>
        )}

        {validation.valid &&
          issues.length === 0 && (
            <p className="validation-copy">
              Receipt totals, required
              fields and line-item
              consistency passed the
              deterministic checks.
            </p>
          )}
      </div>
    </>
  );
}

function ClarificationBox({
  clarification,
  answer,
  setAnswer,
  explanation,
  setExplanation,
  clarifying,
  onClarify,
}) {
  const question =
    clarification.questions?.[0];

  if (!question) {
    return null;
  }

  const supported =
    question.code ===
    DUPLICATE_QUESTION;

  return (
    <>
      <SectionTitle>
        Agent needs clarification
      </SectionTitle>

      <div className="clarification-card">
        <div className="agent-avatar">
          AI
        </div>

        <div className="clarification-content">
          <strong>
            {question.question}
          </strong>

          {question.reason && (
            <p>
              {question.reason}
            </p>
          )}

          {supported ? (
            <>
              <div className="choice-row">
                <button
                  type="button"
                  className={
                    answer === "yes"
                      ? "choice active"
                      : "choice"
                  }
                  onClick={() =>
                    setAnswer("yes")
                  }
                >
                  Yes, separate
                  transaction
                </button>

                <button
                  type="button"
                  className={
                    answer === "no"
                      ? "choice active"
                      : "choice"
                  }
                  onClick={() =>
                    setAnswer("no")
                  }
                >
                  No, duplicate
                </button>
              </div>

              {answer ===
                "yes" && (
                <textarea
                  value={
                    explanation
                  }
                  onChange={(event) =>
                    setExplanation(
                      event.target
                        .value,
                    )
                  }
                  placeholder="Explain why this is a separate transaction..."
                  rows={4}
                />
              )}

              <button
                type="button"
                className="primary-button compact"
                disabled={
                  !answer ||
                  clarifying
                }
                onClick={() =>
                  onClarify(
                    question.code,
                  )
                }
              >
                {clarifying
                  ? "Submitting…"
                  : "Submit clarification"}
              </button>
            </>
          ) : (
            <div className="notice warning">
              This clarification
              currently requires
              manual review.
            </div>
          )}
        </div>
      </div>
    </>
  );
}

function Timeline({
  events,
}) {
  if (!events?.length) {
    return (
      <div className="empty-timeline">
        No lifecycle events
        available.
      </div>
    );
  }

  return (
    <div className="timeline">
      {events.map(
        (event, index) => (
          <div
            className="timeline-item"
            key={
              event.id ||
              `${event.event_type}-${index}`
            }
          >
            <div className="timeline-marker" />

            <div>
              <strong>
                {humanize(
                  event.event_type,
                )}
              </strong>

              <span>
                {formatDateTime(
                  event.created_at,
                )}
              </span>
            </div>
          </div>
        ),
      )}
    </div>
  );
}

function EvidenceRow({
  label,
  value,
  mono = false,
}) {
  return (
    <div className="evidence-row">
      <span>{label}</span>

      <strong
        className={
          mono ? "mono" : ""
        }
      >
        {value || "—"}
      </strong>
    </div>
  );
}

function TraceStep({
  label,
  value,
  state,
}) {
  return (
    <div
      className={`trace-step ${state}`}
    >
      <span>{label}</span>

      <strong>{value}</strong>
    </div>
  );
}

function EmptyResult() {
  return (
    <div className="empty-result">
      <div className="empty-icon">
        ◇
      </div>

      <h3>
        Waiting for an expense
      </h3>

      <p>
        Submit a receipt to see
        extraction, verification,
        duplicate analysis,
        clarification and policy
        results here.
      </p>
    </div>
  );
}

function PipelineItem({
  number,
  title,
  text,
}) {
  return (
    <div className="pipeline-item">
      <span>{number}</span>

      <div>
        <strong>
          {title}
        </strong>

        <small>{text}</small>
      </div>
    </div>
  );
}

function Field({
  label,
  ...props
}) {
  return (
    <label className="field">
      <span>{label}</span>

      <input {...props} />
    </label>
  );
}

function InfoBox({
  label,
  value,
}) {
  return (
    <div className="info-box">
      <span>{label}</span>

      <strong>{value}</strong>
    </div>
  );
}

function Detail({
  label,
  value,
}) {
  return (
    <div className="detail">
      <span>{label}</span>

      <strong>
        {value || "—"}
      </strong>
    </div>
  );
}

function SectionTitle({
  children,
}) {
  return (
    <h3 className="result-section-title">
      {children}
    </h3>
  );
}

function StatusBadge({
  status,
}) {
  return (
    <span
      className={`status-badge ${statusClass(
        status,
      )}`}
    >
      {humanize(
        status || "UNKNOWN",
      )}
    </span>
  );
}

function statusClass(
  status,
) {
  switch (status) {
    case "APPROVED":
    case "YES":
      return "success";

    case "LIMIT_EXCEEDED":
    case "CLARIFICATION_REQUIRED":
      return "warning";

    case "DUPLICATE_DETECTED":
    case "NOT_ALLOWED":
    case "NO":
      return "danger";

    default:
      return "neutral";
  }
}

function shortModelName(
  model,
) {
  if (!model) {
    return "—";
  }

  return model.replace(
    "qwen3-vl:",
    "Qwen3-VL ",
  );
}

function formatMoney(
  amount,
  currency,
) {
  if (
    amount === undefined ||
    amount === null
  ) {
    return "—";
  }

  try {
    return new Intl.NumberFormat(
      "en-US",
      {
        style: "currency",
        currency:
          currency || "USD",
      },
    ).format(amount);
  } catch {
    return `${currency || ""} ${Number(
      amount,
    ).toFixed(2)}`;
  }
}

function formatFileSize(
  bytes,
) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }

  if (bytes < 1024 * 1024) {
    return `${(
      bytes / 1024
    ).toFixed(1)} KB`;
  }

  return `${(
    bytes /
    (1024 * 1024)
  ).toFixed(1)} MB`;
}

function formatDateTime(
  value,
) {
  if (!value) {
    return "";
  }

  const date =
    new Date(value);

  if (
    Number.isNaN(
      date.getTime(),
    )
  ) {
    return value;
  }

  return date.toLocaleString();
}

function humanize(
  value,
) {
  if (!value) {
    return "";
  }

  return String(value)
    .replaceAll("_", " ")
    .toLowerCase()
    .replace(
      /\b\w/g,
      (character) =>
        character.toUpperCase(),
    );
}

function shortID(
  value,
) {
  if (!value) {
    return "—";
  }

  if (value.length <= 22) {
    return value;
  }

  return `${value.slice(
    0,
    18,
  )}…`;
}

export default App;
