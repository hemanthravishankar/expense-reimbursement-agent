import { useEffect, useMemo, useState } from "react";

import {
  createExpenseReport,
  getClaim,
  getClaimEvents,
  getExpenseReport,
  getHealth,
  listAdminExpenseReports,
  listEmployeeExpenseReports,
  resolveClarification,
} from "./api";

import "./App.css";

const DUPLICATE_QUESTION = "CONFIRM_POSSIBLE_DUPLICATE";

const initialEmployee = {
  employee_id: "EMP60001",
  employee_name: "Kiran Kumar",
  employee_grade: "B2",
  department: "Engineering",
  location: "Bengaluru",
};

const initialReportMeta = {
  title: "October Business Expenses",
  purpose: "Customer meeting travel and meal expenses",
};

const reviewStatuses = new Set([
  "MANUAL_REVIEW",
  "REVIEW_REQUIRED",
  "HUMAN_REVIEW_REQUIRED",
  "CURRENCY_MISMATCH",
]);

const actionStatuses = new Set([
  "CLARIFICATION_REQUIRED",
  "ACTION_REQUIRED",
]);

const rejectedStatuses = new Set([
  "REJECTED",
  "DUPLICATE_DETECTED",
  "NOT_ALLOWED",
]);

function App() {
  const [portal, setPortal] = useState("employee");
  const [systemState, setSystemState] = useState("checking");

  const [employee, setEmployee] = useState(initialEmployee);
  const [reportMeta, setReportMeta] = useState(initialReportMeta);
  const [files, setFiles] = useState([]);
  const [dragging, setDragging] = useState(false);

  const [activeReport, setActiveReport] = useState(null);
  const [selectedClaim, setSelectedClaim] = useState(null);
  const [events, setEvents] = useState([]);

  const [employeeReports, setEmployeeReports] = useState([]);
  const [adminReports, setAdminReports] = useState([]);
  const [adminFilter, setAdminFilter] = useState("review");

  const [answer, setAnswer] = useState("");
  const [explanation, setExplanation] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [loadingReport, setLoadingReport] = useState(false);
  const [loadingClaim, setLoadingClaim] = useState(false);
  const [loadingEmployeeHistory, setLoadingEmployeeHistory] = useState(false);
  const [loadingAdmin, setLoadingAdmin] = useState(false);
  const [clarifying, setClarifying] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;

    getHealth()
      .then(() => {
        if (active) {
          setSystemState("online");
        }
      })
      .catch(() => {
        if (active) {
          setSystemState("offline");
        }
      });

    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (portal !== "admin") {
      return undefined;
    }

    let cancelled = false;

    async function loadAdminQueue() {
      try {
        setLoadingAdmin(true);
        setError("");
        const response = await listAdminExpenseReports();

        if (!cancelled) {
          setAdminReports(normalizeListPayload(response));
        }
      } catch (requestError) {
        if (!cancelled) {
          setError(requestError.message);
        }
      } finally {
        if (!cancelled) {
          setLoadingAdmin(false);
        }
      }
    }

    loadAdminQueue();

    return () => {
      cancelled = true;
    };
  }, [portal]);

  const filteredAdminReports = useMemo(() => {
    const reports = normalizeListPayload(adminReports);

    if (adminFilter === "all") {
      return reports;
    }

    return reports.filter((report) => {
      const status = String(report.status || "").toUpperCase();

      if (adminFilter === "review") {
        return reviewStatuses.has(status) || actionStatuses.has(status);
      }

      if (adminFilter === "completed") {
        return ["COMPLETED", "APPROVED", "LIMIT_EXCEEDED"].includes(status);
      }

      if (adminFilter === "rejected") {
        return rejectedStatuses.has(status) || status === "PARTIALLY_REJECTED";
      }

      return true;
    });
  }, [adminReports, adminFilter]);

  const adminCounts = useMemo(() => {
    const reports = normalizeListPayload(adminReports);

    return {
      all: reports.length,
      review: reports.filter((report) => {
        const status = String(report.status || "").toUpperCase();
        return reviewStatuses.has(status) || actionStatuses.has(status);
      }).length,
      completed: reports.filter((report) =>
        ["COMPLETED", "APPROVED", "LIMIT_EXCEEDED"].includes(
          String(report.status || "").toUpperCase(),
        ),
      ).length,
      rejected: reports.filter((report) => {
        const status = String(report.status || "").toUpperCase();
        return rejectedStatuses.has(status) || status === "PARTIALLY_REJECTED";
      }).length,
    };
  }, [adminReports]);

  function updateEmployee(event) {
    const { name, value } = event.target;

    setEmployee((current) => ({
      ...current,
      [name]: value,
    }));
  }

  function updateReportMeta(event) {
    const { name, value } = event.target;

    setReportMeta((current) => ({
      ...current,
      [name]: value,
    }));
  }

  function addFiles(fileList) {
    const incoming = Array.from(fileList || []);

    if (!incoming.length) {
      return;
    }

    const accepted = incoming.filter((file) => {
      const name = file.name.toLowerCase();
      return (
        file.type === "image/jpeg" ||
        file.type === "image/png" ||
        file.type === "application/pdf" ||
        name.endsWith(".jpg") ||
        name.endsWith(".jpeg") ||
        name.endsWith(".png") ||
        name.endsWith(".pdf")
      );
    });

    if (accepted.length !== incoming.length) {
      setError("Only JPG, PNG and PDF receipts are supported.");
    } else {
      setError("");
    }

    setFiles((current) => {
      const next = [...current];

      for (const file of accepted) {
        const duplicate = next.some(
          (item) =>
            item.name === file.name &&
            item.size === file.size &&
            item.lastModified === file.lastModified,
        );

        if (!duplicate) {
          next.push(file);
        }
      }

      return next;
    });
  }

  function removeFile(index) {
    setFiles((current) => current.filter((_, itemIndex) => itemIndex !== index));
  }

  async function submitReport(event) {
    event.preventDefault();
    setError("");
    setAnswer("");
    setExplanation("");

    if (!employee.employee_id.trim()) {
      setError("Employee ID is required.");
      return;
    }

    if (!employee.employee_name.trim()) {
      setError("Employee name is required.");
      return;
    }

    if (!employee.employee_grade.trim()) {
      setError("Employee grade is required.");
      return;
    }

    if (!reportMeta.title.trim()) {
      setError("Report title is required.");
      return;
    }

    if (!files.length) {
      setError("Add at least one receipt or invoice.");
      return;
    }

    const totalSize = files.reduce((sum, file) => sum + file.size, 0);

    if (totalSize > 20 * 1024 * 1024) {
      setError("Combined upload size exceeds the 20 MB request limit.");
      return;
    }

    const formData = new FormData();

    Object.entries(employee).forEach(([key, value]) => {
      formData.append(key, String(value || "").trim());
    });

    formData.append("title", reportMeta.title.trim());
    formData.append("purpose", reportMeta.purpose.trim());

    files.forEach((file) => {
      formData.append("receipts", file);
    });

    try {
      setSubmitting(true);
      setSelectedClaim(null);
      setEvents([]);

      const created = await createExpenseReport(formData);
      setActiveReport(created);

      const firstClaimID = created?.items?.[0]?.claim_id;

      if (firstClaimID) {
        await loadClaimDetail(firstClaimID);
      }

      await refreshEmployeeReports(employee.employee_id);
    } catch (requestError) {
      setError(requestError.message);
    } finally {
      setSubmitting(false);
    }
  }

  async function refreshEmployeeReports(employeeID = employee.employee_id) {
    const trimmed = String(employeeID || "").trim();

    if (!trimmed) {
      setError("Enter an Employee ID before loading report history.");
      return;
    }

    try {
      setLoadingEmployeeHistory(true);
      const response = await listEmployeeExpenseReports(trimmed);
      setEmployeeReports(normalizeListPayload(response));
    } catch (requestError) {
      setError(requestError.message);
    } finally {
      setLoadingEmployeeHistory(false);
    }
  }

  async function refreshAdminReports() {
    try {
      setLoadingAdmin(true);
      setError("");
      const response = await listAdminExpenseReports();
      setAdminReports(normalizeListPayload(response));
    } catch (requestError) {
      setError(requestError.message);
    } finally {
      setLoadingAdmin(false);
    }
  }

  async function openReport(reportID) {
    if (!reportID) {
      return;
    }

    try {
      setLoadingReport(true);
      setError("");
      setSelectedClaim(null);
      setEvents([]);
      setAnswer("");
      setExplanation("");

      const report = await getExpenseReport(reportID);
      setActiveReport(report);

      const firstClaimID = report?.items?.[0]?.claim_id;

      if (firstClaimID) {
        await loadClaimDetail(firstClaimID);
      }
    } catch (requestError) {
      setError(requestError.message);
    } finally {
      setLoadingReport(false);
    }
  }

  async function loadClaimDetail(claimID) {
    if (!claimID) {
      return;
    }

    try {
      setLoadingClaim(true);
      setAnswer("");
      setExplanation("");

      const [claim, timeline] = await Promise.all([
        getClaim(claimID),
        getClaimEvents(claimID).catch(() => []),
      ]);

      setSelectedClaim(claim);
      setEvents(Array.isArray(timeline) ? timeline : []);
    } catch (requestError) {
      setError(requestError.message);
    } finally {
      setLoadingClaim(false);
    }
  }

  async function submitClarification(questionCode) {
    if (!selectedClaim?.run_id) {
      return;
    }

    if (!answer) {
      setError("Select Yes or No before submitting clarification.");
      return;
    }

    if (answer === "yes" && !explanation.trim()) {
      setError("Explain why this is a genuinely separate transaction.");
      return;
    }

    try {
      setClarifying(true);
      setError("");

      const updated = await resolveClarification(selectedClaim.run_id, {
        question_code: questionCode,
        answer,
        explanation: explanation.trim(),
      });

      setSelectedClaim(updated);

      const timeline = await getClaimEvents(updated.run_id).catch(() => []);
      setEvents(Array.isArray(timeline) ? timeline : []);

      if (activeReport?.report_id) {
        const refreshed = await getExpenseReport(activeReport.report_id);
        setActiveReport(refreshed);
      }

      if (portal === "employee") {
        await refreshEmployeeReports(employee.employee_id);
      } else {
        await refreshAdminReports();
      }

      setAnswer("");
      setExplanation("");
    } catch (requestError) {
      setError(requestError.message);
    } finally {
      setClarifying(false);
    }
  }

  function switchPortal(nextPortal) {
    setPortal(nextPortal);
    setActiveReport(null);
    setSelectedClaim(null);
    setEvents([]);
    setError("");
    setAnswer("");
    setExplanation("");
  }

  function resetSubmission() {
    setFiles([]);
    setActiveReport(null);
    setSelectedClaim(null);
    setEvents([]);
    setAnswer("");
    setExplanation("");
    setError("");
  }

  return (
    <div className="app-shell">
      <Header
        portal={portal}
        onSwitchPortal={switchPortal}
        systemState={systemState}
      />

      <main className="main-content">
        {portal === "employee" ? (
          <EmployeePortal
            employee={employee}
            reportMeta={reportMeta}
            files={files}
            dragging={dragging}
            setDragging={setDragging}
            updateEmployee={updateEmployee}
            updateReportMeta={updateReportMeta}
            addFiles={addFiles}
            removeFile={removeFile}
            submitReport={submitReport}
            submitting={submitting}
            activeReport={activeReport}
            selectedClaim={selectedClaim}
            events={events}
            loadClaimDetail={loadClaimDetail}
            loadingClaim={loadingClaim}
            loadingReport={loadingReport}
            employeeReports={normalizeListPayload(employeeReports)}
            refreshEmployeeReports={refreshEmployeeReports}
            loadingEmployeeHistory={loadingEmployeeHistory}
            openReport={openReport}
            answer={answer}
            setAnswer={setAnswer}
            explanation={explanation}
            setExplanation={setExplanation}
            clarifying={clarifying}
            submitClarification={submitClarification}
            resetSubmission={resetSubmission}
            error={error}
          />
        ) : (
          <AdminPortal
            reports={filteredAdminReports}
            counts={adminCounts}
            adminFilter={adminFilter}
            setAdminFilter={setAdminFilter}
            refreshAdminReports={refreshAdminReports}
            loadingAdmin={loadingAdmin}
            activeReport={activeReport}
            selectedClaim={selectedClaim}
            events={events}
            openReport={openReport}
            loadClaimDetail={loadClaimDetail}
            loadingClaim={loadingClaim}
            loadingReport={loadingReport}
            error={error}
          />
        )}
      </main>

      <footer className="footer">
        <span>Expense Agent · AI Innovation Lab</span>
        <span>LLM understands · Go verifies · Policy decides</span>
      </footer>
    </div>
  );
}

function Header({ portal, onSwitchPortal, systemState }) {
  return (
    <header className="topbar">
      <div className="brand-lockup">
        <div className="brand-mark">EA</div>
        <div>
          <div className="brand">Expense Agent</div>
          <div className="brand-subtitle">Intelligent reimbursement operations</div>
        </div>
      </div>

      <nav className="portal-tabs" aria-label="Portal navigation">
        <button
          type="button"
          className={portal === "employee" ? "portal-tab active" : "portal-tab"}
          onClick={() => onSwitchPortal("employee")}
        >
          Employee
        </button>
        <button
          type="button"
          className={portal === "admin" ? "portal-tab active" : "portal-tab"}
          onClick={() => onSwitchPortal("admin")}
        >
          Finance review
        </button>
      </nav>

      <div className={`system-status ${systemState}`}>
        <span className="status-dot" />
        {systemState === "online"
          ? "Agent online"
          : systemState === "offline"
            ? "API unavailable"
            : "Checking API"}
      </div>
    </header>
  );
}

function EmployeePortal(props) {
  const {
    employee,
    reportMeta,
    files,
    dragging,
    setDragging,
    updateEmployee,
    updateReportMeta,
    addFiles,
    removeFile,
    submitReport,
    submitting,
    activeReport,
    selectedClaim,
    events,
    loadClaimDetail,
    loadingClaim,
    loadingReport,
    employeeReports,
    refreshEmployeeReports,
    loadingEmployeeHistory,
    openReport,
    answer,
    setAnswer,
    explanation,
    setExplanation,
    clarifying,
    submitClarification,
    resetSubmission,
    error,
  } = props;

  const combinedSize = files.reduce((sum, file) => sum + file.size, 0);

  return (
    <>
      <section className="hero employee-hero">
        <div className="hero-copy">
          <div className="eyebrow">EMPLOYEE EXPENSE WORKSPACE</div>
          <h1>One report. Multiple receipts. One accountable decision trail.</h1>
          <p>
            Upload receipts together, let the vision agent extract each document,
            then rely on deterministic validation, duplicate controls and policy
            enforcement before reimbursement is calculated.
          </p>
        </div>

        <div className="hero-metrics">
          <Metric label="Vision" value="Qwen3-VL" sub="4B primary · 8B fallback" />
          <Metric label="Controls" value="Deterministic" sub="Validation + policy" />
          <Metric label="Evidence" value="SHA-256" sub="Immutable document identity" />
        </div>
      </section>

      {error && <ErrorBanner message={error} />}

      <section className="employee-layout">
        <div className="left-stack">
          <section className="panel submit-panel">
            <PanelHeader
              eyebrow="NEW REPORT"
              title="Submit business expenses"
              aside={<span className="panel-hint">Up to 20 MB per request</span>}
            />

            <form onSubmit={submitReport}>
              <div className="form-section-title">Employee</div>
              <div className="form-grid three">
                <Field
                  label="Employee ID"
                  name="employee_id"
                  value={employee.employee_id}
                  onChange={updateEmployee}
                  placeholder="EMP60001"
                  required
                />
                <Field
                  label="Employee name"
                  name="employee_name"
                  value={employee.employee_name}
                  onChange={updateEmployee}
                  placeholder="Kiran Kumar"
                  required
                />
                <Field
                  label="Grade"
                  name="employee_grade"
                  value={employee.employee_grade}
                  onChange={updateEmployee}
                  placeholder="B2"
                  required
                />
                <Field
                  label="Department"
                  name="department"
                  value={employee.department}
                  onChange={updateEmployee}
                  placeholder="Engineering"
                />
                <Field
                  label="Location"
                  name="location"
                  value={employee.location}
                  onChange={updateEmployee}
                  placeholder="Bengaluru"
                />
              </div>

              <div className="form-section-title">Report</div>
              <div className="form-grid two">
                <Field
                  label="Report title"
                  name="title"
                  value={reportMeta.title}
                  onChange={updateReportMeta}
                  placeholder="October Business Expenses"
                  required
                />
                <TextAreaField
                  label="Business purpose"
                  name="purpose"
                  value={reportMeta.purpose}
                  onChange={updateReportMeta}
                  placeholder="Customer meeting travel and meal expenses"
                  rows={3}
                />
              </div>

              <div className="form-section-title receipt-title-row">
                <span>Receipts</span>
                {files.length > 0 && (
                  <span className="file-count">
                    {files.length} file{files.length === 1 ? "" : "s"} · {formatFileSize(combinedSize)}
                  </span>
                )}
              </div>

              <label
                className={dragging ? "upload-zone dragging" : "upload-zone"}
                onDragEnter={(event) => {
                  event.preventDefault();
                  setDragging(true);
                }}
                onDragOver={(event) => {
                  event.preventDefault();
                  setDragging(true);
                }}
                onDragLeave={(event) => {
                  event.preventDefault();
                  setDragging(false);
                }}
                onDrop={(event) => {
                  event.preventDefault();
                  setDragging(false);
                  addFiles(event.dataTransfer.files);
                }}
              >
                <input
                  type="file"
                  multiple
                  accept=".jpg,.jpeg,.png,.pdf,image/jpeg,image/png,application/pdf"
                  onChange={(event) => {
                    addFiles(event.target.files);
                    event.target.value = "";
                  }}
                />
                <div className="upload-symbol">＋</div>
                <div>
                  <strong>Drop receipts here or click to browse</strong>
                  <span>JPG, PNG or PDF · multiple files supported</span>
                </div>
              </label>

              {files.length > 0 && (
                <div className="file-list">
                  {files.map((file, index) => (
                    <div className="file-row" key={`${file.name}-${file.size}-${index}`}>
                      <div className="file-type">{fileExtension(file.name)}</div>
                      <div className="file-row-copy">
                        <strong>{file.name}</strong>
                        <span>{formatFileSize(file.size)}</span>
                      </div>
                      <button
                        type="button"
                        className="icon-button"
                        onClick={() => removeFile(index)}
                        aria-label={`Remove ${file.name}`}
                      >
                        ×
                      </button>
                    </div>
                  ))}
                </div>
              )}

              <button className="primary-button" type="submit" disabled={submitting}>
                {submitting ? (
                  <>
                    <span className="spinner" />
                    Processing {files.length} receipt{files.length === 1 ? "" : "s"}…
                  </>
                ) : (
                  <>
                    Submit expense report
                    <span className="button-arrow">→</span>
                  </>
                )}
              </button>
            </form>
          </section>

          <section className="panel history-panel">
            <PanelHeader
              eyebrow="HISTORY"
              title="My expense reports"
              aside={
                <button
                  type="button"
                  className="text-button"
                  onClick={() => refreshEmployeeReports()}
                  disabled={loadingEmployeeHistory}
                >
                  {loadingEmployeeHistory ? "Loading…" : "Refresh"}
                </button>
              }
            />

            {employeeReports.length ? (
              <ReportList
                reports={employeeReports}
                onOpen={openReport}
                activeReportID={activeReport?.report_id}
              />
            ) : (
              <EmptyState
                title="No history loaded"
                text="Enter your Employee ID and select Refresh to load submitted reports."
                compact
              />
            )}
          </section>
        </div>

        <section className="panel result-workspace">
          <PanelHeader
            eyebrow="AGENT OUTCOME"
            title={activeReport ? activeReport.title || "Expense report" : "Report decision"}
            aside={
              activeReport ? (
                <StatusBadge status={activeReport.status} />
              ) : (
                <span className="panel-hint">Waiting for submission</span>
              )
            }
          />

          {loadingReport ? (
            <LoadingState text="Loading report…" />
          ) : !activeReport ? (
            <EmptyResult />
          ) : (
            <ReportWorkspace
              report={activeReport}
              selectedClaim={selectedClaim}
              events={events}
              onSelectClaim={loadClaimDetail}
              loadingClaim={loadingClaim}
              answer={answer}
              setAnswer={setAnswer}
              explanation={explanation}
              setExplanation={setExplanation}
              clarifying={clarifying}
              onClarify={submitClarification}
              employeeView
            />
          )}

          {activeReport && (
            <div className="result-actions">
              <button type="button" className="secondary-button" onClick={resetSubmission}>
                Start another report
              </button>
            </div>
          )}
        </section>
      </section>
    </>
  );
}

function AdminPortal({
  reports,
  counts,
  adminFilter,
  setAdminFilter,
  refreshAdminReports,
  loadingAdmin,
  activeReport,
  selectedClaim,
  events,
  openReport,
  loadClaimDetail,
  loadingClaim,
  loadingReport,
  error,
}) {
  return (
    <>
      <section className="hero admin-hero">
        <div className="hero-copy">
          <div className="eyebrow">FINANCE & RISK CONTROL</div>
          <h1>Review exceptions without losing the evidence trail.</h1>
          <p>
            Cross-employee reuse, strong duplicate identity, human-review cases and
            employee clarifications are surfaced here before money is authorized.
          </p>
        </div>

        <div className="admin-kpis">
          <KpiCard label="Needs review" value={counts.review} tone="warning" />
          <KpiCard label="Completed" value={counts.completed} tone="success" />
          <KpiCard label="Rejected" value={counts.rejected} tone="danger" />
          <KpiCard label="All reports" value={counts.all} tone="neutral" />
        </div>
      </section>

      {error && <ErrorBanner message={error} />}

      <section className="admin-layout">
        <section className="panel review-queue">
          <PanelHeader
            eyebrow="REVIEW QUEUE"
            title="Expense reports"
            aside={
              <button
                type="button"
                className="text-button"
                onClick={refreshAdminReports}
                disabled={loadingAdmin}
              >
                {loadingAdmin ? "Refreshing…" : "Refresh"}
              </button>
            }
          />

          <div className="filter-tabs">
            <FilterButton
              label="Review"
              count={counts.review}
              active={adminFilter === "review"}
              onClick={() => setAdminFilter("review")}
            />
            <FilterButton
              label="Completed"
              count={counts.completed}
              active={adminFilter === "completed"}
              onClick={() => setAdminFilter("completed")}
            />
            <FilterButton
              label="Rejected"
              count={counts.rejected}
              active={adminFilter === "rejected"}
              onClick={() => setAdminFilter("rejected")}
            />
            <FilterButton
              label="All"
              count={counts.all}
              active={adminFilter === "all"}
              onClick={() => setAdminFilter("all")}
            />
          </div>

          {loadingAdmin && !reports.length ? (
            <LoadingState text="Loading Finance queue…" compact />
          ) : reports.length ? (
            <ReportList
              reports={reports}
              onOpen={openReport}
              activeReportID={activeReport?.report_id}
              admin
            />
          ) : (
            <EmptyState
              title="Queue is clear"
              text="No reports match the selected filter."
              compact
            />
          )}
        </section>

        <section className="panel finance-detail">
          <PanelHeader
            eyebrow="CASE DETAIL"
            title={activeReport ? activeReport.title || "Expense report" : "Select a report"}
            aside={activeReport && <StatusBadge status={activeReport.status} />}
          />

          {loadingReport ? (
            <LoadingState text="Loading case…" />
          ) : !activeReport ? (
            <EmptyState
              title="Select a report from the queue"
              text="Finance risk signals, receipt extraction, duplicate evidence and lifecycle events will appear here."
            />
          ) : (
            <ReportWorkspace
              report={activeReport}
              selectedClaim={selectedClaim}
              events={events}
              onSelectClaim={loadClaimDetail}
              loadingClaim={loadingClaim}
              employeeView={false}
            />
          )}
        </section>
      </section>
    </>
  );
}

function ReportWorkspace({
  report,
  selectedClaim,
  events,
  onSelectClaim,
  loadingClaim,
  answer,
  setAnswer,
  explanation,
  setExplanation,
  clarifying,
  onClarify,
  employeeView,
}) {
  return (
    <div className="report-workspace-content">
      <ReportSummary report={report} />

      <SectionTitle>Receipts in this report</SectionTitle>
      <div className="claim-card-grid">
        {(report.items || []).map((item) => (
          <button
            type="button"
            key={item.claim_id}
            className={
              selectedClaim?.run_id === item.claim_id
                ? "claim-card active"
                : "claim-card"
            }
            onClick={() => onSelectClaim(item.claim_id)}
          >
            <div className="claim-card-top">
              <span className="claim-position">#{item.position || 1}</span>
              <StatusBadge status={item.status} />
            </div>
            <strong>{item.merchant || item.document_name || "Receipt"}</strong>
            <span>{item.category || item.document_name || "Expense"}</span>
            <div className="claim-card-money">
              <span>{formatMoney(item.claimed_amount, item.currency)}</span>
              <strong>{formatMoney(item.reimbursable_amount, item.currency)}</strong>
            </div>
          </button>
        ))}
      </div>

      {loadingClaim ? (
        <LoadingState text="Loading claim evidence…" compact />
      ) : selectedClaim ? (
        <ClaimDetail
          claim={selectedClaim}
          events={events}
          employeeView={employeeView}
          answer={answer}
          setAnswer={setAnswer}
          explanation={explanation}
          setExplanation={setExplanation}
          clarifying={clarifying}
          onClarify={onClarify}
        />
      ) : (
        <EmptyState
          title="Select a receipt"
          text="Choose a receipt above to inspect its extraction, policy decision and audit evidence."
          compact
        />
      )}
    </div>
  );
}

function ReportSummary({ report }) {
  const currency = report.items?.find((item) => item.currency)?.currency || "USD";

  return (
    <>
      <div className="report-summary-strip">
        <SummaryMetric label="Report ID" value={shortID(report.report_id)} mono />
        <SummaryMetric label="Receipts" value={String(report.item_count ?? report.items?.length ?? 0)} />
        <SummaryMetric
          label="Claimed"
          value={formatMoney(report.total_claimed, currency)}
        />
        <SummaryMetric
          label="Reimbursable"
          value={formatMoney(report.total_reimbursable, currency)}
          accent
        />
      </div>

      <div className="report-meta-row">
        <div>
          <span>Employee</span>
          <strong>{report.employee?.name || "—"}</strong>
          <small>{report.employee?.id || ""}</small>
        </div>
        <div>
          <span>Grade</span>
          <strong>{report.employee?.grade || "—"}</strong>
        </div>
        <div>
          <span>Purpose</span>
          <strong>{report.purpose || "—"}</strong>
        </div>
        <div>
          <span>Created</span>
          <strong>{formatDateTime(report.created_at)}</strong>
        </div>
      </div>
    </>
  );
}

function ClaimDetail({
  claim,
  events,
  employeeView,
  answer,
  setAnswer,
  explanation,
  setExplanation,
  clarifying,
  onClarify,
}) {
  const extraction = claim.final_extraction;
  const validation = claim.final_validation;
  const duplicate = claim.duplicate_result;
  const decision = claim.policy_decision;
  const clarification = claim.clarification_result;
  const riskReview = Boolean(duplicate?.requires_admin_review);

  return (
    <div className="claim-detail">
      <div className="claim-detail-header">
        <div>
          <span className="detail-kicker">CLAIM DETAIL</span>
          <h3>{extraction?.merchant || claim.receipt_path || "Receipt"}</h3>
          <p>{shortID(claim.run_id)}</p>
        </div>
        <StatusBadge status={claim.final_status} />
      </div>

      {riskReview && (
        <RiskBanner duplicate={duplicate} />
      )}

      {claim.final_status === "MANUAL_REVIEW" && !riskReview && (
        <div className="notice warning strong-notice">
          <strong>Finance review required</strong>
          <span>
            Strong duplicate identity evidence was found. Automatic reimbursement is
            intentionally blocked pending human review.
          </span>
        </div>
      )}

      <SectionTitle>Agent execution</SectionTitle>
      <div className="trace-grid">
        <TraceStep label="Document hash" value="Verified" state="success" />
        <TraceStep
          label="Vision model"
          value={claim.ai_processing_skipped ? "Skipped" : shortModelName(claim.final_model)}
          state={claim.ai_processing_skipped ? "neutral" : "success"}
        />
        <TraceStep
          label="Validation"
          value={claim.ai_processing_skipped ? "N/A" : validation?.valid ? "Passed" : "Failed"}
          state={validation?.valid ? "success" : "warning"}
        />
        <TraceStep
          label="Duplicate control"
          value={duplicate?.match_type ? humanize(duplicate.match_type) : "No match"}
          state={duplicate?.match_type && duplicate.match_type !== "NONE" ? "warning" : "success"}
        />
        <TraceStep
          label="Policy"
          value={decision ? "Evaluated" : riskReview || claim.final_status === "MANUAL_REVIEW" ? "Held for review" : "Not required"}
          state={decision ? "success" : "neutral"}
        />
      </div>

      {extraction && (
        <>
          <SectionTitle>Extracted expense</SectionTitle>
          <div className="detail-grid">
            <Detail label="Merchant" value={extraction.merchant} />
            <Detail label="Expense date" value={extraction.expense_date} />
            <Detail label="Category" value={extraction.category_suggestion} />
            <Detail label="Receipt number" value={extraction.receipt_number || "—"} />
            <Detail label="Currency" value={extraction.currency} />
            <Detail
              label="AI confidence"
              value={
                extraction.confidence !== undefined && extraction.confidence !== null
                  ? `${Math.round(extraction.confidence * 100)}%`
                  : "—"
              }
            />
            <Detail
              label="Subtotal"
              value={formatMoney(extraction.subtotal, extraction.currency)}
            />
            <Detail
              label="Tax"
              value={formatMoney(extraction.tax_amount, extraction.currency)}
            />
            <Detail
              label="Service charge"
              value={formatMoney(extraction.service_charge, extraction.currency)}
            />
            <Detail
              label="Other fees"
              value={formatMoney(extraction.other_fees, extraction.currency)}
            />
            <Detail
              label="Discount"
              value={formatMoney(extraction.discount_amount, extraction.currency)}
            />
            <Detail
              label="Total"
              value={formatMoney(extraction.total_amount, extraction.currency)}
              strong
            />
          </div>

          <LineItems extraction={extraction} />
        </>
      )}

      {validation && !claim.ai_processing_skipped && (
        <ValidationPanel validation={validation} currency={extraction?.currency} />
      )}

      {duplicate && duplicate.match_type && duplicate.match_type !== "NONE" && (
        <DuplicatePanel duplicate={duplicate} employeeView={employeeView} />
      )}

      {employeeView && clarification?.required && (
        <ClarificationBox
          clarification={clarification}
          answer={answer}
          setAnswer={setAnswer}
          explanation={explanation}
          setExplanation={setExplanation}
          clarifying={clarifying}
          onClarify={onClarify}
        />
      )}

      {decision && (
        <DecisionPanel decision={decision} />
      )}

      {!decision && (riskReview || claim.final_status === "MANUAL_REVIEW") && (
        <>
          <SectionTitle>Reimbursement state</SectionTitle>
          <div className="held-decision-card">
            <div>
              <span>Automatic reimbursement</span>
              <strong>Blocked pending review</strong>
            </div>
            <div className="held-amount">
              <span>Authorized now</span>
              <strong>{formatMoney(0, extraction?.currency || "USD")}</strong>
            </div>
          </div>
        </>
      )}

      {claim.clarification_answers?.length > 0 && (
        <ClarificationAnswers answers={claim.clarification_answers} />
      )}

      <EvidencePanel claim={claim} />

      <SectionTitle>Business lifecycle</SectionTitle>
      <Timeline events={events} />
    </div>
  );
}

function RiskBanner({ duplicate }) {
  return (
    <div className="risk-banner">
      <div className="risk-icon">!</div>
      <div className="risk-copy">
        <span className="risk-label">FINANCE RISK SIGNAL</span>
        <strong>{humanize(duplicate.match_type)}</strong>
        <p>{duplicate.reason}</p>
        <div className="risk-meta">
          <span>
            Confidence <strong>{Math.round((duplicate.confidence || 0) * 100)}%</strong>
          </span>
          <span>
            Strong identity <strong>{duplicate.strong_identity ? "Yes" : "No"}</strong>
          </span>
          <span>
            Matched employee <strong>{duplicate.matched_employee_id || "—"}</strong>
          </span>
          <span>
            Matched run <strong className="mono">{shortID(duplicate.matched_run_id)}</strong>
          </span>
        </div>
      </div>
    </div>
  );
}

function DuplicatePanel({ duplicate, employeeView }) {
  const crossEmployee = String(duplicate.match_type || "").startsWith("CROSS_EMPLOYEE");

  return (
    <>
      <SectionTitle>Duplicate & fraud controls</SectionTitle>
      <div className={duplicate.requires_admin_review ? "duplicate-card risk" : "duplicate-card"}>
        <div>
          <div className="duplicate-title-row">
            <strong>{humanize(duplicate.match_type)}</strong>
            {duplicate.strong_identity && <span className="mini-pill">Strong identity</span>}
          </div>
          <p>{duplicate.reason}</p>

          {!employeeView && duplicate.matched_employee_id && (
            <div className="duplicate-metadata">
              <span>Matched employee</span>
              <strong>{duplicate.matched_employee_id}</strong>
            </div>
          )}

          {duplicate.matched_run_id && !crossEmployee && (
            <small>Matched claim: {shortID(duplicate.matched_run_id)}</small>
          )}
        </div>
        <div className="confidence-score">
          {Math.round((duplicate.confidence || 0) * 100)}%
          <span>match</span>
        </div>
      </div>
    </>
  );
}

function DecisionPanel({ decision }) {
  return (
    <>
      <SectionTitle>Reimbursement decision</SectionTitle>
      <div className="decision-card">
        <div>
          <span>Claimed</span>
          <strong>{formatMoney(decision.claimed_amount, decision.currency)}</strong>
        </div>
        <div>
          <span>Policy limit</span>
          <strong>{formatMoney(decision.policy_limit, decision.currency)}</strong>
        </div>
        <div className="decision-highlight">
          <span>Reimbursable</span>
          <strong>{formatMoney(decision.reimbursable_amount, decision.currency)}</strong>
        </div>
        <div>
          <span>Excess</span>
          <strong>{formatMoney(decision.excess_amount, decision.currency)}</strong>
        </div>
      </div>

      {decision.reason && (
        <div className="policy-reason">
          <strong>Deterministic policy reasoning</strong>
          <p>{decision.reason}</p>
        </div>
      )}
    </>
  );
}

function ClarificationAnswers({ answers }) {
  return (
    <>
      <SectionTitle>Employee clarification</SectionTitle>
      <div className="answer-list">
        {answers.map((item, index) => (
          <div className="answered-card" key={`${item.question_code}-${index}`}>
            <div>
              <strong>{humanize(item.question_code)}</strong>
              {item.explanation && <p>{item.explanation}</p>}
              {item.answered_at && <small>{formatDateTime(item.answered_at)}</small>}
            </div>
            <StatusBadge status={String(item.answer || "").toUpperCase()} />
          </div>
        ))}
      </div>
    </>
  );
}

function EvidencePanel({ claim }) {
  if (!claim.stored_document && !claim.receipt_sha256) {
    return null;
  }

  return (
    <>
      <SectionTitle>Evidence & audit</SectionTitle>
      <div className="evidence-card">
        <EvidenceRow
          label="Original file"
          value={claim.stored_document?.original_name || claim.receipt_path}
        />
        <EvidenceRow label="MIME type" value={claim.stored_document?.mime_type || claim.document_mime_type} />
        <EvidenceRow label="SHA-256" value={claim.receipt_sha256 || claim.stored_document?.sha256} mono />
        <EvidenceRow label="Storage key" value={claim.stored_document?.key} mono />
        <EvidenceRow label="Audit record" value={claim.audit_path} mono />
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
  const question = clarification.questions?.[0];

  if (!question) {
    return null;
  }

  const supported = question.code === DUPLICATE_QUESTION;

  return (
    <>
      <SectionTitle>Agent needs your clarification</SectionTitle>
      <div className="clarification-card">
        <div className="agent-avatar">AI</div>
        <div className="clarification-content">
          <strong>{question.question}</strong>
          {question.reason && <p>{question.reason}</p>}

          {supported ? (
            <>
              <div className="choice-row">
                <button
                  type="button"
                  className={answer === "yes" ? "choice active" : "choice"}
                  onClick={() => setAnswer("yes")}
                >
                  Yes, genuinely separate
                </button>
                <button
                  type="button"
                  className={answer === "no" ? "choice active danger-choice" : "choice"}
                  onClick={() => setAnswer("no")}
                >
                  No, same expense
                </button>
              </div>

              {answer === "yes" && (
                <textarea
                  value={explanation}
                  onChange={(event) => setExplanation(event.target.value)}
                  placeholder="Explain what makes this a separate transaction…"
                  rows={4}
                />
              )}

              <button
                type="button"
                className="primary-button compact"
                disabled={!answer || clarifying}
                onClick={() => onClarify(question.code)}
              >
                {clarifying ? "Submitting…" : "Submit clarification"}
              </button>
            </>
          ) : (
            <div className="notice warning">
              <strong>Manual review required</strong>
              <span>This clarification type must be handled by Finance.</span>
            </div>
          )}
        </div>
      </div>
    </>
  );
}

function ValidationPanel({ validation, currency }) {
  const issues = validation.issues || [];

  return (
    <>
      <SectionTitle>Deterministic validation</SectionTitle>
      <div className={validation.valid ? "validation-card valid" : "validation-card invalid"}>
        <div className="validation-heading">
          <div>
            <strong>{validation.valid ? "Validation passed" : "Validation failed"}</strong>
            <span>Go verification layer</span>
          </div>
          <div className="validation-state">{validation.valid ? "PASS" : "FAIL"}</div>
        </div>

        {validation.line_item_sum !== undefined && (
          <div className="validation-metric">
            <span>Calculated line-item sum</span>
            <strong>{formatMoney(validation.line_item_sum, currency)}</strong>
          </div>
        )}

        {issues.length > 0 ? (
          <div className="issue-list">
            {issues.map((issue, index) => (
              <div className="issue-row" key={issue.code || index}>
                <span className={`issue-severity ${String(issue.severity || "INFO").toLowerCase()}`}>
                  {issue.severity || "INFO"}
                </span>
                <div>
                  <strong>{issue.code ? humanize(issue.code) : "Validation issue"}</strong>
                  <p>{issue.message || issue.reason || JSON.stringify(issue)}</p>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p className="validation-copy">
            Receipt totals, required fields and line-item consistency passed deterministic checks.
          </p>
        )}
      </div>
    </>
  );
}

function LineItems({ extraction }) {
  const items = extraction.line_items || [];

  if (!items.length) {
    return null;
  }

  return (
    <>
      <SectionTitle>Line items</SectionTitle>
      <div className="line-items-table">
        <div className="line-item header">
          <span>Item</span>
          <span>Qty</span>
          <span>Amount</span>
        </div>
        {items.map((item, index) => (
          <div className="line-item" key={`${item.name}-${index}`}>
            <span>{item.name || "Unnamed item"}</span>
            <span>{item.quantity || "—"}</span>
            <strong>{formatMoney(item.amount, extraction.currency)}</strong>
          </div>
        ))}
      </div>
    </>
  );
}

function Timeline({ events }) {
  if (!events?.length) {
    return <div className="empty-timeline">No lifecycle events available.</div>;
  }

  return (
    <div className="timeline">
      {events.map((event, index) => (
        <div className="timeline-item" key={event.id || `${event.event_type}-${index}`}>
          <div className="timeline-marker" />
          <div className="timeline-copy">
            <strong>{humanize(event.event_type)}</strong>
            <span>{formatDateTime(event.created_at)}</span>
          </div>
        </div>
      ))}
    </div>
  );
}

function ReportList({ reports, onOpen, activeReportID, admin = false }) {
  return (
    <div className="report-list">
      {reports.map((report) => {
        const id = report.report_id || report.id;
        const currency = report.items?.find((item) => item.currency)?.currency || "USD";

        return (
          <button
            type="button"
            className={id === activeReportID ? "report-list-item active" : "report-list-item"}
            key={id}
            onClick={() => onOpen(id)}
          >
            <div className="report-list-top">
              <div>
                <strong>{report.title || "Expense report"}</strong>
                <span>{admin ? report.employee?.name || report.employee_name || "Employee" : formatDateTime(report.created_at)}</span>
              </div>
              <StatusBadge status={report.status} />
            </div>
            <div className="report-list-bottom">
              {admin && (
                <span className="employee-chip">
                  {report.employee?.id || report.employee_id || "—"}
                </span>
              )}
              <span>{report.item_count ?? report.items?.length ?? 0} receipt{(report.item_count ?? report.items?.length ?? 0) === 1 ? "" : "s"}</span>
              <strong>{formatMoney(report.total_reimbursable, currency)}</strong>
            </div>
          </button>
        );
      })}
    </div>
  );
}

function PanelHeader({ eyebrow, title, aside }) {
  return (
    <div className="panel-heading">
      <div>
        <span className="panel-eyebrow">{eyebrow}</span>
        <h2>{title}</h2>
      </div>
      {aside && <div className="panel-aside">{aside}</div>}
    </div>
  );
}

function Metric({ label, value, sub }) {
  return (
    <div className="hero-metric">
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{sub}</small>
    </div>
  );
}

function KpiCard({ label, value, tone }) {
  return (
    <div className={`kpi-card ${tone}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function FilterButton({ label, count, active, onClick }) {
  return (
    <button
      type="button"
      className={active ? "filter-button active" : "filter-button"}
      onClick={onClick}
    >
      {label}
      <span>{count}</span>
    </button>
  );
}

function Field({ label, ...props }) {
  return (
    <label className="field">
      <span>{label}</span>
      <input {...props} />
    </label>
  );
}

function TextAreaField({ label, ...props }) {
  return (
    <label className="field">
      <span>{label}</span>
      <textarea {...props} />
    </label>
  );
}

function Detail({ label, value, strong = false }) {
  return (
    <div className={strong ? "detail strong" : "detail"}>
      <span>{label}</span>
      <strong>{value || "—"}</strong>
    </div>
  );
}

function SummaryMetric({ label, value, mono = false, accent = false }) {
  return (
    <div className={accent ? "summary-metric accent" : "summary-metric"}>
      <span>{label}</span>
      <strong className={mono ? "mono" : ""}>{value || "—"}</strong>
    </div>
  );
}

function EvidenceRow({ label, value, mono = false }) {
  return (
    <div className="evidence-row">
      <span>{label}</span>
      <strong className={mono ? "mono" : ""}>{value || "—"}</strong>
    </div>
  );
}

function TraceStep({ label, value, state }) {
  return (
    <div className={`trace-step ${state}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function SectionTitle({ children }) {
  return <h4 className="result-section-title">{children}</h4>;
}

function StatusBadge({ status }) {
  return (
    <span className={`status-badge ${statusClass(status)}`}>
      {humanize(status || "UNKNOWN")}
    </span>
  );
}

function ErrorBanner({ message }) {
  return (
    <div className="error-banner">
      <div className="error-icon">!</div>
      <div>
        <strong>Something needs attention</strong>
        <span>{message}</span>
      </div>
    </div>
  );
}

function EmptyResult() {
  return (
    <div className="empty-result">
      <div className="empty-illustration">
        <span>01</span>
        <span>02</span>
        <span>03</span>
      </div>
      <h3>Your report decision will appear here</h3>
      <p>
        Submit one or more receipts to see extraction, validation, duplicate controls,
        reimbursement policy and the business event trail.
      </p>
    </div>
  );
}

function EmptyState({ title, text, compact = false }) {
  return (
    <div className={compact ? "empty-state compact" : "empty-state"}>
      <div className="empty-diamond">◇</div>
      <strong>{title}</strong>
      <p>{text}</p>
    </div>
  );
}

function LoadingState({ text, compact = false }) {
  return (
    <div className={compact ? "loading-state compact" : "loading-state"}>
      <span className="spinner dark" />
      <span>{text}</span>
    </div>
  );
}

function statusClass(status) {
  const value = String(status || "").toUpperCase();

  if (["APPROVED", "COMPLETED", "YES"].includes(value)) {
    return "success";
  }

  if (["LIMIT_EXCEEDED", "CLARIFICATION_REQUIRED", "ACTION_REQUIRED", "PROCESSING"].includes(value)) {
    return "warning";
  }

  if (["MANUAL_REVIEW", "REVIEW_REQUIRED", "HUMAN_REVIEW_REQUIRED", "CURRENCY_MISMATCH"].includes(value)) {
    return "review";
  }

  if (["DUPLICATE_DETECTED", "NOT_ALLOWED", "REJECTED", "PARTIALLY_REJECTED", "NO"].includes(value)) {
    return "danger";
  }

  return "neutral";
}

function normalizeListPayload(value) {
  if (Array.isArray(value)) {
    return value;
  }

  if (Array.isArray(value?.reports)) {
    return value.reports;
  }

  if (Array.isArray(value?.items)) {
    return value.items;
  }

  return [];
}

function shortModelName(model) {
  if (!model) {
    return "—";
  }

  return model.replace("qwen3-vl:", "Qwen3-VL ");
}

function formatMoney(amount, currency) {
  if (amount === undefined || amount === null || Number.isNaN(Number(amount))) {
    return "—";
  }

  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: currency || "USD",
      maximumFractionDigits: 2,
    }).format(Number(amount));
  } catch {
    return `${currency || ""} ${Number(amount).toFixed(2)}`.trim();
  }
}

function formatFileSize(bytes) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }

  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }

  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDateTime(value) {
  if (!value) {
    return "—";
  }

  const date = new Date(value);

  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleString([], {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

function humanize(value) {
  if (!value) {
    return "";
  }

  return String(value)
    .replaceAll("_", " ")
    .toLowerCase()
    .replace(/\b\w/g, (character) => character.toUpperCase());
}

function shortID(value) {
  if (!value) {
    return "—";
  }

  const stringValue = String(value);

  if (stringValue.length <= 26) {
    return stringValue;
  }

  return `${stringValue.slice(0, 21)}…`;
}

function fileExtension(name) {
  const extension = String(name || "")
    .split(".")
    .pop()
    ?.toUpperCase();

  return extension || "FILE";
}

export default App;
