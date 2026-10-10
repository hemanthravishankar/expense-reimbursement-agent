import { useEffect, useMemo, useState } from "react";

import {
  createExpenseReport,
  createPolicy,
  getActivePolicy,
  getClaim,
  getClaimDocumentURL,
  getClaimEvents,
  getExpenseReport,
  getHealth,
  getPolicy,
  listAdminExpenseReports,
  listEmployeeExpenseReports,
  listPolicies,
  publishPolicy,
  resolveClarification,
  reviewClaim,
  updatePolicy,
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

const reviewableClaimStatuses = new Set([
  "MANUAL_REVIEW",
  "HUMAN_REVIEW_REQUIRED",
]);

function App() {
  const [portal, setPortal] = useState("employee");
  const [financeSection, setFinanceSection] = useState("review");
  const [systemState, setSystemState] = useState("checking");

  const [employee, setEmployee] = useState(initialEmployee);
  const [reportMeta, setReportMeta] = useState(initialReportMeta);
  const [files, setFiles] = useState([]);
  const [dragging, setDragging] = useState(false);

  const [activeReport, setActiveReport] = useState(null);
  const [selectedClaim, setSelectedClaim] = useState(null);
  const [referenceClaim, setReferenceClaim] = useState(null);
  const [events, setEvents] = useState([]);

  const [employeeReports, setEmployeeReports] = useState([]);
  const [adminReports, setAdminReports] = useState([]);
  const [adminFilter, setAdminFilter] = useState("review");

  const [answer, setAnswer] = useState("");
  const [explanation, setExplanation] = useState("");

  const [reviewer, setReviewer] = useState("Finance User");
  const [reviewNote, setReviewNote] = useState("");

  const [policies, setPolicies] = useState([]);
  const [activePolicy, setActivePolicy] = useState(null);
  const [selectedPolicy, setSelectedPolicy] = useState(null);

  const [submitting, setSubmitting] = useState(false);
  const [loadingReport, setLoadingReport] = useState(false);
  const [loadingClaim, setLoadingClaim] = useState(false);
  const [loadingEmployeeHistory, setLoadingEmployeeHistory] = useState(false);
  const [loadingAdmin, setLoadingAdmin] = useState(false);
  const [loadingPolicies, setLoadingPolicies] = useState(false);
  const [clarifying, setClarifying] = useState(false);
  const [reviewing, setReviewing] = useState(false);
  const [savingPolicy, setSavingPolicy] = useState(false);
  const [error, setError] = useState("");
  const [policyError, setPolicyError] = useState("");

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

    async function loadFinanceData() {
      try {
        setLoadingAdmin(true);
        setLoadingPolicies(true);
        setError("");
        setPolicyError("");

        const [reportsResponse, policiesResponse, activeResponse] =
          await Promise.all([
            listAdminExpenseReports(),
            listPolicies(),
            getActivePolicy(),
          ]);

        if (!cancelled) {
          setAdminReports(normalizeListPayload(reportsResponse));
          setPolicies(normalizeListPayload(policiesResponse));
          setActivePolicy(activeResponse || null);

          const normalizedPolicies =
            normalizeListPayload(policiesResponse);

          setSelectedPolicy((current) => {
            if (current) {
              return current;
            }

            const preferred =
              normalizedPolicies.find(
                (item) => item.status === "DRAFT",
              ) ||
              normalizedPolicies.find(
                (item) => item.status === "PUBLISHED",
              ) ||
              normalizedPolicies[0];

            return preferred || null;
          });
        }
      } catch (requestError) {
        if (!cancelled) {
          setError(requestError.message);
        }
      } finally {
        if (!cancelled) {
          setLoadingAdmin(false);
          setLoadingPolicies(false);
        }
      }
    }

    loadFinanceData();

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

    const accepted = [];
    const rejected = [];

    for (const file of incoming) {
      const name = file.name.toLowerCase();
      const supported =
        file.type === "image/jpeg" ||
        file.type === "image/png" ||
        file.type === "application/pdf" ||
        name.endsWith(".jpg") ||
        name.endsWith(".jpeg") ||
        name.endsWith(".png") ||
        name.endsWith(".pdf");

      if (!supported || file.size > 20 * 1024 * 1024) {
        rejected.push(file);
      } else {
        accepted.push(file);
      }
    }

    if (rejected.length) {
      setError(
        "Only JPG, PNG and PDF files up to 20 MB each are supported.",
      );
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

        if (!duplicate && next.length < 10) {
          next.push(file);
        }
      }

      return next;
    });
  }

  function removeFile(index) {
    setFiles((current) =>
      current.filter((_, itemIndex) => itemIndex !== index),
    );
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

    if (files.length > 10) {
      setError("A maximum of 10 receipts can be submitted in one report.");
      return;
    }

    const totalSize = files.reduce((sum, file) => sum + file.size, 0);

    if (totalSize > 95 * 1024 * 1024) {
      setError("Combined upload size is too large for one report.");
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
      setReferenceClaim(null);
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

  async function refreshPolicies(preferredPolicyID = "") {
    try {
      setLoadingPolicies(true);
      setPolicyError("");

      const [versions, active] = await Promise.all([
        listPolicies(),
        getActivePolicy(),
      ]);

      const normalized = normalizeListPayload(versions);
      setPolicies(normalized);
      setActivePolicy(active || null);

      const preferred =
        normalized.find((item) => item.policy_id === preferredPolicyID) ||
        normalized.find(
          (item) => item.policy_id === selectedPolicy?.policy_id,
        ) ||
        normalized.find((item) => item.status === "DRAFT") ||
        normalized.find((item) => item.status === "PUBLISHED") ||
        normalized[0];

      setSelectedPolicy(preferred || null);
      return preferred || null;
    } catch (requestError) {
      setPolicyError(requestError.message);
      return null;
    } finally {
      setLoadingPolicies(false);
    }
  }

  async function openPolicy(policyID) {
    if (!policyID) {
      return;
    }

    try {
      setLoadingPolicies(true);
      setPolicyError("");
      const version = await getPolicy(policyID);
      setSelectedPolicy(version);
    } catch (requestError) {
      setPolicyError(requestError.message);
    } finally {
      setLoadingPolicies(false);
    }
  }

  async function createPolicyDraft(payload) {
    try {
      setSavingPolicy(true);
      setPolicyError("");
      const created = await createPolicy(payload);
      await refreshPolicies(created.policy_id);
      return created;
    } catch (requestError) {
      setPolicyError(requestError.message);
      throw requestError;
    } finally {
      setSavingPolicy(false);
    }
  }

  async function savePolicyDraft(policyID, payload) {
    try {
      setSavingPolicy(true);
      setPolicyError("");
      const updated = await updatePolicy(policyID, payload);
      await refreshPolicies(updated.policy_id);
      return updated;
    } catch (requestError) {
      setPolicyError(requestError.message);
      throw requestError;
    } finally {
      setSavingPolicy(false);
    }
  }

  async function publishPolicyDraft(policyID, publishedBy) {
    try {
      setSavingPolicy(true);
      setPolicyError("");
      const published = await publishPolicy(policyID, {
        published_by: publishedBy,
      });
      await refreshPolicies(published.policy_id);
      return published;
    } catch (requestError) {
      setPolicyError(requestError.message);
      throw requestError;
    } finally {
      setSavingPolicy(false);
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
      setReferenceClaim(null);
      setEvents([]);
      setAnswer("");
      setExplanation("");
      setReviewNote("");

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
      setReviewNote("");
      setReferenceClaim(null);

      const [claim, timeline] = await Promise.all([
        getClaim(claimID),
        getClaimEvents(claimID).catch(() => []),
      ]);

      setSelectedClaim(claim);
      setEvents(Array.isArray(timeline) ? timeline : []);

      const matchedRunID = claim?.duplicate_result?.matched_run_id;

      if (portal === "admin" && matchedRunID) {
        const matched = await getClaim(matchedRunID).catch(() => null);
        setReferenceClaim(matched);
      }
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

  async function submitFinanceReview(action) {
    if (!selectedClaim?.run_id) {
      return;
    }

    if (!reviewer.trim()) {
      setError("Finance reviewer name is required.");
      return;
    }

    if (!reviewNote.trim()) {
      setError("Finance review note is required.");
      return;
    }

    try {
      setReviewing(true);
      setError("");

      const updated = await reviewClaim(selectedClaim.run_id, {
        action,
        reviewer: reviewer.trim(),
        note: reviewNote.trim(),
      });

      setSelectedClaim(updated);

      const timeline = await getClaimEvents(updated.run_id).catch(() => []);
      setEvents(Array.isArray(timeline) ? timeline : []);

      if (activeReport?.report_id) {
        const refreshed = await getExpenseReport(activeReport.report_id);
        setActiveReport(refreshed);
      }

      await refreshAdminReports();
      setReviewNote("");
    } catch (requestError) {
      setError(requestError.message);
    } finally {
      setReviewing(false);
    }
  }

  function switchPortal(nextPortal) {
    setPortal(nextPortal);
    setActiveReport(null);
    setSelectedClaim(null);
    setReferenceClaim(null);
    setEvents([]);
    setError("");
    setAnswer("");
    setExplanation("");
    setReviewNote("");
  }

  function resetSubmission() {
    setFiles([]);
    setActiveReport(null);
    setSelectedClaim(null);
    setReferenceClaim(null);
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
            financeSection={financeSection}
            setFinanceSection={setFinanceSection}
            reports={filteredAdminReports}
            counts={adminCounts}
            adminFilter={adminFilter}
            setAdminFilter={setAdminFilter}
            refreshAdminReports={refreshAdminReports}
            loadingAdmin={loadingAdmin}
            activeReport={activeReport}
            selectedClaim={selectedClaim}
            referenceClaim={referenceClaim}
            events={events}
            openReport={openReport}
            loadClaimDetail={loadClaimDetail}
            loadingClaim={loadingClaim}
            loadingReport={loadingReport}
            reviewer={reviewer}
            setReviewer={setReviewer}
            reviewNote={reviewNote}
            setReviewNote={setReviewNote}
            reviewing={reviewing}
            submitFinanceReview={submitFinanceReview}
            policies={policies}
            activePolicy={activePolicy}
            selectedPolicy={selectedPolicy}
            openPolicy={openPolicy}
            refreshPolicies={refreshPolicies}
            createPolicyDraft={createPolicyDraft}
            savePolicyDraft={savePolicyDraft}
            publishPolicyDraft={publishPolicyDraft}
            loadingPolicies={loadingPolicies}
            savingPolicy={savingPolicy}
            policyError={policyError}
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
          <div className="brand-subtitle">
            Intelligent reimbursement operations
          </div>
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
          Finance
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
      <section className="hero employee-hero employee-hero-refined">
        <div className="hero-copy">
          <div className="eyebrow">EMPLOYEE EXPENSES</div>
          <h1>Submit receipts. Track what is reimbursable.</h1>
          <p>
            Create one report for multiple expenses. The agent reads each receipt,
            verifies the numbers, checks policy and asks only when it needs your
            clarification.
          </p>
        </div>

        <div className="employee-promise-card">
          <div>
            <span>01</span>
            <strong>Upload</strong>
            <small>Up to 10 receipts</small>
          </div>
          <div>
            <span>02</span>
            <strong>Verify</strong>
            <small>AI + deterministic checks</small>
          </div>
          <div>
            <span>03</span>
            <strong>Track</strong>
            <small>Claim and reimbursement status</small>
          </div>
        </div>
      </section>

      {error && <ErrorBanner message={error} />}

      <section className="employee-layout">
        <div className="left-stack">
          <section className="panel submit-panel">
            <PanelHeader
              eyebrow="NEW REPORT"
              title="Submit business expenses"
              aside={
                <span className="panel-hint">
                  10 receipts max · 20 MB each
                </span>
              }
            />

            <form onSubmit={submitReport}>
              <div className="form-section-title">Your details</div>
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

              <div className="form-section-title">Expense report</div>
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
                    {files.length} file{files.length === 1 ? "" : "s"} ·{" "}
                    {formatFileSize(combinedSize)}
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
                    <div
                      className="file-row"
                      key={`${file.name}-${file.size}-${index}`}
                    >
                      <div className="file-type">
                        {fileExtension(file.name)}
                      </div>
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

              <button
                className="primary-button"
                type="submit"
                disabled={submitting}
              >
                {submitting ? (
                  <>
                    <span className="spinner" />
                    Processing {files.length} receipt
                    {files.length === 1 ? "" : "s"}…
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
            eyebrow="REPORT STATUS"
            title={
              activeReport
                ? activeReport.title || "Expense report"
                : "Your reimbursement result"
            }
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
              <button
                type="button"
                className="secondary-button"
                onClick={resetSubmission}
              >
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
  financeSection,
  setFinanceSection,
  reports,
  counts,
  adminFilter,
  setAdminFilter,
  refreshAdminReports,
  loadingAdmin,
  activeReport,
  selectedClaim,
  referenceClaim,
  events,
  openReport,
  loadClaimDetail,
  loadingClaim,
  loadingReport,
  reviewer,
  setReviewer,
  reviewNote,
  setReviewNote,
  reviewing,
  submitFinanceReview,
  policies,
  activePolicy,
  selectedPolicy,
  openPolicy,
  refreshPolicies,
  createPolicyDraft,
  savePolicyDraft,
  publishPolicyDraft,
  loadingPolicies,
  savingPolicy,
  policyError,
  error,
}) {
  return (
    <>
      <section className="hero admin-hero">
        <div className="hero-copy">
          <div className="eyebrow">FINANCE CONTROL CENTER</div>
          <h1>Investigate risk, decide claims and control policy.</h1>
          <p>
            Review evidence before reimbursement, compare suspicious receipts
            side by side and publish versioned policy without rebuilding the
            application.
          </p>
        </div>

        <div className="admin-kpis">
          <KpiCard label="Needs review" value={counts.review} tone="warning" />
          <KpiCard label="Completed" value={counts.completed} tone="success" />
          <KpiCard label="Rejected" value={counts.rejected} tone="danger" />
          <KpiCard
            label="Active policy"
            value={
              activePolicy?.version
                ? `v${activePolicy.version}`
                : "—"
            }
            tone="neutral"
          />
        </div>
      </section>

      <div className="finance-section-tabs">
        <button
          type="button"
          className={
            financeSection === "review"
              ? "finance-section-tab active"
              : "finance-section-tab"
          }
          onClick={() => setFinanceSection("review")}
        >
          Review queue
        </button>
        <button
          type="button"
          className={
            financeSection === "policies"
              ? "finance-section-tab active"
              : "finance-section-tab"
          }
          onClick={() => setFinanceSection("policies")}
        >
          Policy management
        </button>
      </div>

      {error && financeSection === "review" && (
        <ErrorBanner message={error} />
      )}

      {financeSection === "review" ? (
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
              title={
                activeReport
                  ? activeReport.title || "Expense report"
                  : "Select a report"
              }
              aside={
                activeReport && <StatusBadge status={activeReport.status} />
              }
            />

            {loadingReport ? (
              <LoadingState text="Loading case…" />
            ) : !activeReport ? (
              <EmptyState
                title="Select a report from the queue"
                text="Finance risk signals, receipt evidence, policy decisions and review actions will appear here."
              />
            ) : (
              <ReportWorkspace
                report={activeReport}
                selectedClaim={selectedClaim}
                referenceClaim={referenceClaim}
                events={events}
                onSelectClaim={loadClaimDetail}
                loadingClaim={loadingClaim}
                employeeView={false}
                reviewer={reviewer}
                setReviewer={setReviewer}
                reviewNote={reviewNote}
                setReviewNote={setReviewNote}
                reviewing={reviewing}
                onFinanceReview={submitFinanceReview}
              />
            )}
          </section>
        </section>
      ) : (
        <PolicyManagement
          key={`${selectedPolicy?.policy_id || "none"}-${
            selectedPolicy?.status || "unknown"
          }`}
          policies={policies}
          activePolicy={activePolicy}
          selectedPolicy={selectedPolicy}
          onOpenPolicy={openPolicy}
          onRefresh={refreshPolicies}
          onCreate={createPolicyDraft}
          onSave={savePolicyDraft}
          onPublish={publishPolicyDraft}
          loading={loadingPolicies}
          saving={savingPolicy}
          error={policyError}
        />
      )}
    </>
  );
}

function ReportWorkspace({
  report,
  selectedClaim,
  referenceClaim,
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
  reviewer,
  setReviewer,
  reviewNote,
  setReviewNote,
  reviewing,
  onFinanceReview,
}) {
  return (
    <div className="report-workspace-content">
      <ReportSummary report={report} employeeView={employeeView} />

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
              <span className="claim-position">
                #{item.position || 1}
              </span>
              <StatusBadge status={item.status} />
            </div>
            <strong>
              {item.merchant || item.document_name || "Receipt"}
            </strong>
            <span>
              {item.category || item.document_name || "Expense"}
            </span>
            <div className="claim-card-money">
              <span>
                {formatMoney(item.claimed_amount, item.currency)}
              </span>
              <strong>
                {formatMoney(item.reimbursable_amount, item.currency)}
              </strong>
            </div>
          </button>
        ))}
      </div>

      {loadingClaim ? (
        <LoadingState text="Loading claim evidence…" compact />
      ) : selectedClaim ? (
        <ClaimDetail
          claim={selectedClaim}
          referenceClaim={referenceClaim}
          events={events}
          employeeView={employeeView}
          answer={answer}
          setAnswer={setAnswer}
          explanation={explanation}
          setExplanation={setExplanation}
          clarifying={clarifying}
          onClarify={onClarify}
          reviewer={reviewer}
          setReviewer={setReviewer}
          reviewNote={reviewNote}
          setReviewNote={setReviewNote}
          reviewing={reviewing}
          onFinanceReview={onFinanceReview}
        />
      ) : (
        <EmptyState
          title="Select a receipt"
          text={
            employeeView
              ? "Choose a receipt above to see its reimbursement result."
              : "Choose a receipt above to inspect evidence, risk and policy."
          }
          compact
        />
      )}
    </div>
  );
}

function ReportSummary({ report, employeeView }) {
  const currency =
    report.items?.find((item) => item.currency)?.currency || "USD";

  return (
    <>
      <div className="report-summary-strip">
        {!employeeView && (
          <SummaryMetric
            label="Report ID"
            value={shortID(report.report_id)}
            mono
          />
        )}
        <SummaryMetric
          label="Receipts"
          value={String(
            report.item_count ?? report.items?.length ?? 0,
          )}
        />
        <SummaryMetric
          label="Claimed"
          value={formatMoney(report.total_claimed, currency)}
        />
        <SummaryMetric
          label="Reimbursable"
          value={formatMoney(report.total_reimbursable, currency)}
          accent
        />
        {employeeView && (
          <SummaryMetric
            label="Status"
            value={humanize(report.status)}
          />
        )}
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
  referenceClaim,
  events,
  employeeView,
  answer,
  setAnswer,
  explanation,
  setExplanation,
  clarifying,
  onClarify,
  reviewer,
  setReviewer,
  reviewNote,
  setReviewNote,
  reviewing,
  onFinanceReview,
}) {
  const extraction = claim.final_extraction;
  const validation = claim.final_validation;
  const duplicate = claim.duplicate_result;
  const decision = claim.policy_decision;
  const clarification = claim.clarification_result;
  const riskReview = Boolean(duplicate?.requires_admin_review);
  const reviewable = reviewableClaimStatuses.has(
    String(claim.final_status || "").toUpperCase(),
  );

  if (employeeView) {
    return (
      <div className="claim-detail employee-claim-detail">
        <EmployeeOutcomeSummary
          claim={claim}
          extraction={extraction}
          decision={decision}
        />

        {clarification?.required && (
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

        {extraction && (
          <>
            <SectionTitle>Receipt summary</SectionTitle>
            <div className="employee-receipt-summary">
              <Detail label="Merchant" value={extraction.merchant} />
              <Detail label="Date" value={extraction.expense_date} />
              <Detail
                label="Category"
                value={extraction.category_suggestion}
              />
              <Detail
                label="Receipt number"
                value={extraction.receipt_number || "—"}
              />
              <Detail
                label="Total"
                value={formatMoney(
                  extraction.total_amount,
                  extraction.currency,
                )}
                strong
              />
            </div>

            <LineItems extraction={extraction} />
          </>
        )}

        {decision && <DecisionPanel decision={decision} employeeView />}

        {!decision &&
          (riskReview || claim.final_status === "MANUAL_REVIEW") && (
            <>
              <SectionTitle>Reimbursement</SectionTitle>
              <div className="held-decision-card">
                <div>
                  <span>Current state</span>
                  <strong>Finance review required</strong>
                  <small>
                    No reimbursement is authorized until the review is
                    completed.
                  </small>
                </div>
                <div className="held-amount">
                  <span>Authorized now</span>
                  <strong>
                    {formatMoney(
                      0,
                      extraction?.currency || "USD",
                    )}
                  </strong>
                </div>
              </div>
            </>
          )}

        {claim.clarification_answers?.length > 0 && (
          <ClarificationAnswers answers={claim.clarification_answers} />
        )}

        <details className="technical-disclosure">
          <summary>
            <span>
              <strong>Technical details</strong>
              <small>
                Validation, duplicate checks, evidence and lifecycle
              </small>
            </span>
            <span className="disclosure-arrow">⌄</span>
          </summary>

          <div className="technical-body">
            <ExecutionTrace
              claim={claim}
              validation={validation}
              duplicate={duplicate}
              decision={decision}
            />

            {validation && !claim.ai_processing_skipped && (
              <ValidationPanel
                validation={validation}
                currency={extraction?.currency}
              />
            )}

            {duplicate &&
              duplicate.match_type &&
              duplicate.match_type !== "NONE" && (
                <DuplicatePanel
                  duplicate={duplicate}
                  employeeView
                />
              )}

            <EvidencePanel claim={claim} />

            <SectionTitle>Business lifecycle</SectionTitle>
            <Timeline events={events} />
          </div>
        </details>
      </div>
    );
  }

  return (
    <div className="claim-detail finance-claim-detail">
      <div className="claim-detail-header">
        <div>
          <span className="detail-kicker">CLAIM INVESTIGATION</span>
          <h3>
            {extraction?.merchant || claim.receipt_path || "Receipt"}
          </h3>
          <p>{shortID(claim.run_id)}</p>
        </div>
        <StatusBadge status={claim.final_status} />
      </div>

      {riskReview && <RiskBanner duplicate={duplicate} />}

      {claim.final_status === "MANUAL_REVIEW" && !riskReview && (
        <div className="notice warning strong-notice">
          <strong>Finance review required</strong>
          <span>
            Strong transaction identity or another review condition
            blocked automatic reimbursement.
          </span>
        </div>
      )}

      {(duplicate?.matched_run_id || referenceClaim) && (
        <FinanceEvidenceComparison
          claim={claim}
          referenceClaim={referenceClaim}
        />
      )}

      <ExecutionTrace
        claim={claim}
        validation={validation}
        duplicate={duplicate}
        decision={decision}
      />

      {extraction && (
        <>
          <SectionTitle>Extracted expense</SectionTitle>
          <div className="detail-grid">
            <Detail label="Merchant" value={extraction.merchant} />
            <Detail label="Expense date" value={extraction.expense_date} />
            <Detail
              label="Category"
              value={extraction.category_suggestion}
            />
            <Detail
              label="Receipt number"
              value={extraction.receipt_number || "—"}
            />
            <Detail label="Currency" value={extraction.currency} />
            <Detail
              label="AI confidence"
              value={
                extraction.confidence !== undefined &&
                extraction.confidence !== null
                  ? `${Math.round(extraction.confidence * 100)}%`
                  : "—"
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
              label="Service charge"
              value={formatMoney(
                extraction.service_charge,
                extraction.currency,
              )}
            />
            <Detail
              label="Other fees"
              value={formatMoney(
                extraction.other_fees,
                extraction.currency,
              )}
            />
            <Detail
              label="Discount"
              value={formatMoney(
                extraction.discount_amount,
                extraction.currency,
              )}
            />
            <Detail
              label="Total"
              value={formatMoney(
                extraction.total_amount,
                extraction.currency,
              )}
              strong
            />
          </div>

          <LineItems extraction={extraction} />
        </>
      )}

      {validation && !claim.ai_processing_skipped && (
        <ValidationPanel
          validation={validation}
          currency={extraction?.currency}
        />
      )}

      {duplicate &&
        duplicate.match_type &&
        duplicate.match_type !== "NONE" && (
          <DuplicatePanel duplicate={duplicate} employeeView={false} />
        )}

      {decision && <DecisionPanel decision={decision} />}

      {!decision && reviewable && (
        <>
          <SectionTitle>Reimbursement state</SectionTitle>
          <div className="held-decision-card">
            <div>
              <span>Automatic reimbursement</span>
              <strong>Blocked pending Finance decision</strong>
            </div>
            <div className="held-amount">
              <span>Authorized now</span>
              <strong>
                {formatMoney(0, extraction?.currency || "USD")}
              </strong>
            </div>
          </div>
        </>
      )}

      {claim.clarification_answers?.length > 0 && (
        <ClarificationAnswers answers={claim.clarification_answers} />
      )}

      {reviewable && onFinanceReview && (
        <FinanceReviewPanel
          reviewer={reviewer}
          setReviewer={setReviewer}
          note={reviewNote}
          setNote={setReviewNote}
          reviewing={reviewing}
          onReview={onFinanceReview}
        />
      )}

      <EvidencePanel claim={claim} />

      <SectionTitle>Business lifecycle</SectionTitle>
      <Timeline events={events} />
    </div>
  );
}

function EmployeeOutcomeSummary({ claim, extraction, decision }) {
  const status = String(claim.final_status || "");
  const currency =
    decision?.currency || extraction?.currency || "USD";
  const claimed =
    decision?.claimed_amount ?? extraction?.total_amount ?? 0;
  const reimbursable =
    decision?.reimbursable_amount ??
    (status === "APPROVED" ? claimed : 0);

  return (
    <div className={`employee-outcome ${statusClass(status)}`}>
      <div className="employee-outcome-main">
        <div>
          <span className="detail-kicker">YOUR CLAIM</span>
          <h3>{extraction?.merchant || claim.receipt_path || "Receipt"}</h3>
          <p>{employeeStatusMessage(status)}</p>
        </div>
        <StatusBadge status={status} />
      </div>

      <div className="employee-outcome-money">
        <div>
          <span>Claimed</span>
          <strong>{formatMoney(claimed, currency)}</strong>
        </div>
        <div className="outcome-reimbursable">
          <span>Reimbursable now</span>
          <strong>{formatMoney(reimbursable, currency)}</strong>
        </div>
      </div>
    </div>
  );
}

function ExecutionTrace({ claim, validation, duplicate, decision }) {
  const riskHeld =
    duplicate?.requires_admin_review ||
    claim.final_status === "MANUAL_REVIEW";

  return (
    <>
      <SectionTitle>Agent execution</SectionTitle>
      <div className="trace-grid">
        <TraceStep
          label="Document hash"
          value="Verified"
          state="success"
        />
        <TraceStep
          label="Vision model"
          value={
            claim.ai_processing_skipped
              ? "Skipped"
              : shortModelName(claim.final_model)
          }
          state={
            claim.ai_processing_skipped ? "neutral" : "success"
          }
        />
        <TraceStep
          label="Validation"
          value={
            claim.ai_processing_skipped
              ? "N/A"
              : validation?.valid
                ? "Passed"
                : "Failed"
          }
          state={validation?.valid ? "success" : "warning"}
        />
        <TraceStep
          label="Duplicate control"
          value={
            duplicate?.match_type
              ? humanize(duplicate.match_type)
              : "No match"
          }
          state={
            duplicate?.match_type &&
            duplicate.match_type !== "NONE"
              ? "warning"
              : "success"
          }
        />
        <TraceStep
          label="Policy"
          value={
            decision
              ? decision.policy_version
                ? `v${decision.policy_version}`
                : "Evaluated"
              : riskHeld
                ? "Held for review"
                : "Not required"
          }
          state={decision ? "success" : "neutral"}
        />
      </div>
    </>
  );
}

function FinanceEvidenceComparison({ claim, referenceClaim }) {
  const current = claim.final_extraction;
  const reference = referenceClaim?.final_extraction;
  const duplicate = claim.duplicate_result;

  return (
    <>
      <SectionTitle>Receipt comparison</SectionTitle>

      <div className="comparison-risk-summary">
        <div>
          <span>Match type</span>
          <strong>{humanize(duplicate?.match_type)}</strong>
        </div>
        <div>
          <span>Confidence</span>
          <strong>
            {Math.round((duplicate?.confidence || 0) * 100)}%
          </strong>
        </div>
        <div>
          <span>Strong identity</span>
          <strong>
            {duplicate?.strong_identity ? "Yes" : "No"}
          </strong>
        </div>
        <div>
          <span>Matched employee</span>
          <strong>
            {duplicate?.matched_employee_id || "—"}
          </strong>
        </div>
      </div>

      <div className="receipt-comparison-grid">
        <ReceiptEvidenceCard
          title="Current claim"
          claim={claim}
          extraction={current}
          employeeLabel={`${claim.employee?.name || "Employee"} · ${
            claim.employee?.id || "—"
          }`}
        />

        <ReceiptEvidenceCard
          title="Reference claim"
          claim={referenceClaim}
          extraction={reference}
          employeeLabel={
            referenceClaim
              ? `${
                  referenceClaim.employee?.name || "Employee"
                } · ${referenceClaim.employee?.id || "—"}`
              : duplicate?.matched_employee_id || "Matched employee"
          }
          emptyText={
            referenceClaim
              ? ""
              : "Reference claim could not be loaded."
          }
        />
      </div>
    </>
  );
}

function ReceiptEvidenceCard({
  title,
  claim,
  extraction,
  employeeLabel,
  emptyText,
}) {
  const mimeType =
    claim?.stored_document?.mime_type ||
    claim?.document_mime_type ||
    "";
  const documentURL = claim?.run_id
    ? getClaimDocumentURL(claim.run_id)
    : "";

  return (
    <div className="receipt-evidence-card">
      <div className="receipt-evidence-heading">
        <div>
          <span>{title}</span>
          <strong>{employeeLabel}</strong>
        </div>
        {claim && <StatusBadge status={claim.final_status} />}
      </div>

      {claim && documentURL ? (
        <DocumentPreview
          url={documentURL}
          mimeType={mimeType}
          name={
            claim.stored_document?.original_name ||
            claim.receipt_path ||
            "receipt"
          }
        />
      ) : (
        <div className="document-preview empty">
          {emptyText || "No document available."}
        </div>
      )}

      <div className="comparison-fields">
        <EvidenceFact
          label="Merchant"
          value={extraction?.merchant || "—"}
        />
        <EvidenceFact
          label="Date"
          value={extraction?.expense_date || "—"}
        />
        <EvidenceFact
          label="Receipt #"
          value={extraction?.receipt_number || "—"}
        />
        <EvidenceFact
          label="Total"
          value={
            extraction
              ? formatMoney(
                  extraction.total_amount,
                  extraction.currency,
                )
              : "—"
          }
        />
      </div>
    </div>
  );
}

function DocumentPreview({ url, mimeType, name }) {
  const isPDF =
    String(mimeType || "").toLowerCase() === "application/pdf" ||
    String(name || "").toLowerCase().endsWith(".pdf");

  return (
    <div className="document-preview">
      {isPDF ? (
        <iframe src={url} title={name || "Receipt PDF"} />
      ) : (
        <img src={url} alt={name || "Receipt evidence"} />
      )}
      <a href={url} target="_blank" rel="noreferrer">
        Open original ↗
      </a>
    </div>
  );
}

function EvidenceFact({ label, value }) {
  return (
    <div>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function FinanceReviewPanel({
  reviewer,
  setReviewer,
  note,
  setNote,
  reviewing,
  onReview,
}) {
  return (
    <>
      <SectionTitle>Finance decision</SectionTitle>
      <div className="finance-review-panel">
        <div className="finance-review-copy">
          <strong>Resolve this review</strong>
          <p>
            Approving confirms the transaction is legitimate and then
            applies the currently published deterministic policy. Rejecting
            authorizes no reimbursement.
          </p>
        </div>

        <div className="finance-review-form">
          <Field
            label="Reviewed by"
            value={reviewer}
            onChange={(event) => setReviewer(event.target.value)}
            placeholder="Finance reviewer"
          />
          <TextAreaField
            label="Review note"
            value={note}
            onChange={(event) => setNote(event.target.value)}
            placeholder="Record why this claim is legitimate or why it should be rejected…"
            rows={4}
          />
        </div>

        <div className="finance-review-actions">
          <button
            type="button"
            className="danger-button"
            disabled={reviewing || !reviewer.trim() || !note.trim()}
            onClick={() => onReview("REJECT")}
          >
            {reviewing ? "Saving…" : "Reject claim"}
          </button>
          <button
            type="button"
            className="approve-button"
            disabled={reviewing || !reviewer.trim() || !note.trim()}
            onClick={() => onReview("APPROVE")}
          >
            {reviewing
              ? "Saving…"
              : "Approve as legitimate & apply policy"}
          </button>
        </div>
      </div>
    </>
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
            Confidence{" "}
            <strong>
              {Math.round((duplicate.confidence || 0) * 100)}%
            </strong>
          </span>
          <span>
            Strong identity{" "}
            <strong>
              {duplicate.strong_identity ? "Yes" : "No"}
            </strong>
          </span>
          <span>
            Matched employee{" "}
            <strong>{duplicate.matched_employee_id || "—"}</strong>
          </span>
          <span>
            Matched run{" "}
            <strong className="mono">
              {shortID(duplicate.matched_run_id)}
            </strong>
          </span>
        </div>
      </div>
    </div>
  );
}

function DuplicatePanel({ duplicate, employeeView }) {
  const crossEmployee = String(
    duplicate.match_type || "",
  ).startsWith("CROSS_EMPLOYEE");

  return (
    <>
      <SectionTitle>Duplicate & fraud controls</SectionTitle>
      <div
        className={
          duplicate.requires_admin_review
            ? "duplicate-card risk"
            : "duplicate-card"
        }
      >
        <div>
          <div className="duplicate-title-row">
            <strong>{humanize(duplicate.match_type)}</strong>
            {duplicate.strong_identity && (
              <span className="mini-pill">Strong identity</span>
            )}
          </div>
          <p>
            {employeeView && crossEmployee
              ? "This claim requires Finance review before reimbursement can be authorized."
              : duplicate.reason}
          </p>

          {!employeeView && duplicate.matched_employee_id && (
            <div className="duplicate-metadata">
              <span>Matched employee</span>
              <strong>{duplicate.matched_employee_id}</strong>
            </div>
          )}

          {duplicate.matched_run_id &&
            (!crossEmployee || !employeeView) && (
              <small>
                Matched claim: {shortID(duplicate.matched_run_id)}
              </small>
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

function DecisionPanel({ decision, employeeView = false }) {
  return (
    <>
      <SectionTitle>
        {employeeView
          ? "Reimbursement"
          : "Reimbursement decision"}
      </SectionTitle>
      <div className="decision-card">
        <div>
          <span>Claimed</span>
          <strong>
            {formatMoney(
              decision.claimed_amount,
              decision.currency,
            )}
          </strong>
        </div>
        <div>
          <span>Policy limit</span>
          <strong>
            {formatMoney(
              decision.policy_limit,
              decision.currency,
            )}
          </strong>
        </div>
        <div className="decision-highlight">
          <span>Reimbursable</span>
          <strong>
            {formatMoney(
              decision.reimbursable_amount,
              decision.currency,
            )}
          </strong>
        </div>
        <div>
          <span>Excess</span>
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
            {employeeView ? "Why" : "Deterministic policy reasoning"}
          </strong>
          <p>{decision.reason}</p>
          {(decision.policy_name || decision.policy_version) && (
            <small>
              {decision.policy_name || "Policy"}
              {decision.policy_version
                ? ` · Version ${decision.policy_version}`
                : ""}
            </small>
          )}
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
          <div
            className="answered-card"
            key={`${item.question_code}-${index}`}
          >
            <div>
              <strong>{humanize(item.question_code)}</strong>
              {item.explanation && <p>{item.explanation}</p>}
              {item.answered_at && (
                <small>{formatDateTime(item.answered_at)}</small>
              )}
            </div>
            <StatusBadge
              status={String(item.answer || "").toUpperCase()}
            />
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
          value={
            claim.stored_document?.original_name ||
            claim.receipt_path
          }
        />
        <EvidenceRow
          label="MIME type"
          value={
            claim.stored_document?.mime_type ||
            claim.document_mime_type
          }
        />
        <EvidenceRow
          label="SHA-256"
          value={
            claim.receipt_sha256 ||
            claim.stored_document?.sha256
          }
          mono
        />
        <EvidenceRow
          label="Storage key"
          value={claim.stored_document?.key}
          mono
        />
        <EvidenceRow
          label="Audit record"
          value={claim.audit_path}
          mono
        />
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
      <SectionTitle>Action required</SectionTitle>
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
                  className={
                    answer === "yes"
                      ? "choice active"
                      : "choice"
                  }
                  onClick={() => setAnswer("yes")}
                >
                  Yes, genuinely separate
                </button>
                <button
                  type="button"
                  className={
                    answer === "no"
                      ? "choice active danger-choice"
                      : "choice"
                  }
                  onClick={() => setAnswer("no")}
                >
                  No, same expense
                </button>
              </div>

              {answer === "yes" && (
                <textarea
                  value={explanation}
                  onChange={(event) =>
                    setExplanation(event.target.value)
                  }
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
                {clarifying
                  ? "Submitting…"
                  : "Submit clarification"}
              </button>
            </>
          ) : (
            <div className="notice warning">
              <strong>Manual review required</strong>
              <span>
                This clarification type must be handled by Finance.
              </span>
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
            <span>Go verification layer</span>
          </div>
          <div className="validation-state">
            {validation.valid ? "PASS" : "FAIL"}
          </div>
        </div>

        {validation.line_item_sum !== undefined && (
          <div className="validation-metric">
            <span>Calculated line-item sum</span>
            <strong>
              {formatMoney(validation.line_item_sum, currency)}
            </strong>
          </div>
        )}

        {issues.length > 0 && (
          <div className="issue-list">
            {issues.map((issue, index) => (
              <div
                className="issue-row"
                key={issue.code || index}
              >
                <span
                  className={`issue-severity ${String(
                    issue.severity || "INFO",
                  ).toLowerCase()}`}
                >
                  {issue.severity || "INFO"}
                </span>

                <div>
                  <strong>
                    {issue.code
                      ? humanize(issue.code)
                      : "Validation issue"}
                  </strong>
                  <p>
                    {issue.message ||
                      issue.reason ||
                      JSON.stringify(issue)}
                  </p>
                </div>
              </div>
            ))}
          </div>
        )}

        {validation.valid && issues.length === 0 && (
          <p className="validation-copy">
            Receipt totals, required fields and line-item
            consistency passed deterministic checks.
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
          <div
            className="line-item"
            key={`${item.name}-${index}`}
          >
            <span>{item.name || "Unnamed item"}</span>
            <span>{item.quantity || "—"}</span>
            <strong>
              {formatMoney(item.amount, extraction.currency)}
            </strong>
          </div>
        ))}
      </div>
    </>
  );
}

function Timeline({ events }) {
  if (!events?.length) {
    return (
      <div className="empty-timeline">
        No lifecycle events are available.
      </div>
    );
  }

  return (
    <div className="timeline">
      {events.map((event, index) => (
        <div
          className="timeline-item"
          key={`${event.event_id || event.id || index}`}
        >
          <div className="timeline-marker" />
          <div>
            <strong>
              {humanize(
                event.event_type || event.type || "EVENT",
              )}
            </strong>
            <span>
              {formatDateTime(
                event.created_at ||
                  event.occurred_at ||
                  event.timestamp,
              )}
            </span>
            {event.payload && (
              <EventPayload payload={event.payload} />
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

function EventPayload({ payload }) {
  const value =
    typeof payload === "string"
      ? safeJSONParse(payload)
      : payload;

  if (!value || typeof value !== "object") {
    return null;
  }

  const note = value.note;
  const reviewer = value.reviewer;
  const action = value.action;

  if (!note && !reviewer && !action) {
    return null;
  }

  return (
    <div className="timeline-payload">
      {action && <strong>{humanize(action)}</strong>}
      {reviewer && <span>Reviewer: {reviewer}</span>}
      {note && <p>{note}</p>}
    </div>
  );
}

function PolicyManagement({
  policies,
  activePolicy,
  selectedPolicy,
  onOpenPolicy,
  onRefresh,
  onCreate,
  onSave,
  onPublish,
  loading,
  saving,
  error,
}) {
  const [creating, setCreating] = useState(false);
  const [actor, setActor] = useState("Finance User");
  const [name, setName] = useState(
    activePolicy?.name || "Company Expense Policy",
  );
  const [effectiveFrom, setEffectiveFrom] = useState(
    selectedPolicy?.effective_from || todayDateString(),
  );
  const [currency, setCurrency] = useState(
    selectedPolicy?.currency || activePolicy?.currency || "USD",
  );
  const [rules, setRules] = useState(() =>
    Array.isArray(selectedPolicy?.rules)
      ? selectedPolicy.rules.map((rule) => ({ ...rule }))
      : activePolicyToRules(activePolicy),
  );

  const isDraft = selectedPolicy?.status === "DRAFT";

  function startCreating() {
    setName(activePolicy?.name || "Company Expense Policy");
    setCurrency(activePolicy?.currency || "USD");
    setEffectiveFrom(todayDateString());
    setRules(activePolicyToRules(activePolicy));
    setCreating(true);
  }

  function updateRule(index, field, value) {
    setRules((current) =>
      current.map((rule, itemIndex) => {
        if (itemIndex !== index) {
          return rule;
        }

        if (field === "allowed") {
          return {
            ...rule,
            allowed: !!value,
            max_amount: value
              ? Number(rule.max_amount || 0)
              : 0,
          };
        }

        if (field === "max_amount") {
          return {
            ...rule,
            max_amount: Number(value),
          };
        }

        return {
          ...rule,
          [field]: value,
        };
      }),
    );
  }

  function addRule() {
    setRules((current) => [
      ...current,
      {
        employee_grade: "B2",
        category: "",
        allowed: true,
        max_amount: 1,
      },
    ]);
  }

  function removeRule(index) {
    setRules((current) =>
      current.filter((_, itemIndex) => itemIndex !== index),
    );
  }

  async function handleCreate(event) {
    event.preventDefault();

    await onCreate({
      name: name.trim(),
      currency: currency.trim().toUpperCase(),
      effective_from: effectiveFrom,
      created_by: actor.trim(),
      rules: sanitizePolicyRules(rules),
    });

    setCreating(false);
  }

  async function handleSave() {
    if (!selectedPolicy?.policy_id) {
      return;
    }

    await onSave(selectedPolicy.policy_id, {
      currency: currency.trim().toUpperCase(),
      effective_from: effectiveFrom,
      rules: sanitizePolicyRules(rules),
    });
  }

  async function handlePublish() {
    if (!selectedPolicy?.policy_id || !actor.trim()) {
      return;
    }

    await onPublish(selectedPolicy.policy_id, actor.trim());
  }

  return (
    <section className="policy-layout">
      {error && <ErrorBanner message={error} />}

      <div className="policy-overview-grid">
        <section className="panel active-policy-card">
          <PanelHeader
            eyebrow="ACTIVE POLICY"
            title={activePolicy?.name || "No active policy"}
            aside={
              activePolicy?.version ? (
                <StatusBadge status="PUBLISHED" />
              ) : null
            }
          />

          {activePolicy ? (
            <>
              <div className="active-policy-hero">
                <div>
                  <span>Published version</span>
                  <strong>v{activePolicy.version}</strong>
                </div>
                <div>
                  <span>Currency</span>
                  <strong>{activePolicy.currency}</strong>
                </div>
                <div>
                  <span>Configured grades</span>
                  <strong>
                    {Object.keys(activePolicy.limits || {}).length}
                  </strong>
                </div>
                <div>
                  <span>Rules</span>
                  <strong>
                    {activePolicyToRules(activePolicy).length}
                  </strong>
                </div>
              </div>

              <p className="policy-help">
                This is the policy currently used for new claims,
                clarification approvals and Finance approvals.
              </p>
            </>
          ) : (
            <EmptyState
              title="No published policy"
              text="Publish a draft before automatic reimbursement decisions can use it."
              compact
            />
          )}
        </section>

        <section className="panel policy-actions-card">
          <PanelHeader
            eyebrow="POLICY OPERATIONS"
            title="Version safely"
          />

          <p className="policy-help">
            Published versions are immutable. Create a draft, edit the
            rules and publish it when Finance is ready. Existing claim
            decisions remain historically intact.
          </p>

          <div className="policy-top-actions">
            <button
              type="button"
              className="primary-button"
              onClick={startCreating}
            >
              Create draft from active
            </button>
            <button
              type="button"
              className="secondary-button"
              onClick={() => onRefresh()}
              disabled={loading}
            >
              {loading ? "Refreshing…" : "Refresh policies"}
            </button>
          </div>
        </section>
      </div>

      {creating && (
        <section className="panel create-policy-panel">
          <PanelHeader
            eyebrow="NEW POLICY VERSION"
            title="Create draft"
            aside={
              <button
                type="button"
                className="text-button"
                onClick={() => setCreating(false)}
              >
                Cancel
              </button>
            }
          />

          <form onSubmit={handleCreate}>
            <div className="form-grid three">
              <Field
                label="Policy name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                required
              />
              <Field
                label="Currency"
                value={currency}
                onChange={(event) =>
                  setCurrency(event.target.value.toUpperCase())
                }
                required
              />
              <Field
                label="Effective from"
                type="date"
                value={effectiveFrom}
                onChange={(event) =>
                  setEffectiveFrom(event.target.value)
                }
                required
              />
              <Field
                label="Created by"
                value={actor}
                onChange={(event) => setActor(event.target.value)}
                required
              />
            </div>

            <PolicyRuleEditor
              rules={rules}
              editable
              updateRule={updateRule}
              addRule={addRule}
              removeRule={removeRule}
              currency={currency}
            />

            <button
              className="primary-button compact"
              type="submit"
              disabled={saving}
            >
              {saving ? "Creating…" : "Create draft"}
            </button>
          </form>
        </section>
      )}

      <section className="policy-main-grid">
        <section className="panel policy-version-list-panel">
          <PanelHeader
            eyebrow="VERSION HISTORY"
            title="Policy versions"
          />

          {loading && !policies.length ? (
            <LoadingState text="Loading policies…" compact />
          ) : policies.length ? (
            <div className="policy-version-list">
              {policies.map((version) => (
                <button
                  type="button"
                  key={version.policy_id}
                  className={
                    selectedPolicy?.policy_id === version.policy_id
                      ? "policy-version-row active"
                      : "policy-version-row"
                  }
                  onClick={() => onOpenPolicy(version.policy_id)}
                >
                  <div>
                    <strong>
                      {version.name} · v{version.version}
                    </strong>
                    <span>
                      Effective {version.effective_from} ·{" "}
                      {version.currency}
                    </span>
                  </div>
                  <StatusBadge status={version.status} />
                </button>
              ))}
            </div>
          ) : (
            <EmptyState
              title="No policy versions"
              text="Create a draft to begin."
              compact
            />
          )}
        </section>

        <section className="panel policy-editor-panel">
          <PanelHeader
            eyebrow={isDraft ? "DRAFT EDITOR" : "POLICY DETAIL"}
            title={
              selectedPolicy
                ? `${selectedPolicy.name} · v${selectedPolicy.version}`
                : "Select a version"
            }
            aside={
              selectedPolicy && (
                <StatusBadge status={selectedPolicy.status} />
              )
            }
          />

          {!selectedPolicy ? (
            <EmptyState
              title="Select a policy version"
              text="Choose a version from the left to inspect or edit its rules."
            />
          ) : (
            <>
              <div className="policy-metadata-grid">
                <Detail
                  label="Policy ID"
                  value={shortID(selectedPolicy.policy_id)}
                />
                <Detail
                  label="Created by"
                  value={selectedPolicy.created_by || "—"}
                />
                <Detail
                  label="Created"
                  value={formatDateTime(selectedPolicy.created_at)}
                />
                <Detail
                  label="Published by"
                  value={selectedPolicy.published_by || "—"}
                />
              </div>

              <div className="policy-editor-controls">
                <Field
                  label="Currency"
                  value={currency}
                  disabled={!isDraft}
                  onChange={(event) =>
                    setCurrency(event.target.value.toUpperCase())
                  }
                />
                <Field
                  label="Effective from"
                  type="date"
                  value={effectiveFrom}
                  disabled={!isDraft}
                  onChange={(event) =>
                    setEffectiveFrom(event.target.value)
                  }
                />
                {isDraft && (
                  <Field
                    label="Finance actor"
                    value={actor}
                    onChange={(event) => setActor(event.target.value)}
                    placeholder="Finance User"
                  />
                )}
              </div>

              <PolicyRuleEditor
                rules={rules}
                editable={isDraft}
                updateRule={updateRule}
                addRule={addRule}
                removeRule={removeRule}
                currency={currency}
              />

              {isDraft && (
                <div className="policy-editor-actions">
                  <button
                    type="button"
                    className="secondary-button"
                    onClick={handleSave}
                    disabled={saving}
                  >
                    {saving ? "Saving…" : "Save draft"}
                  </button>
                  <button
                    type="button"
                    className="approve-button"
                    onClick={handlePublish}
                    disabled={saving || !actor.trim()}
                  >
                    {saving ? "Publishing…" : "Publish & activate"}
                  </button>
                </div>
              )}

              {!isDraft && (
                <div className="notice">
                  <strong>Immutable historical version</strong>
                  <span>
                    Published and retired policies remain read-only so
                    claim decisions can be audited against the version
                    that produced them.
                  </span>
                </div>
              )}
            </>
          )}
        </section>
      </section>
    </section>
  );
}

function PolicyRuleEditor({
  rules,
  editable,
  updateRule,
  addRule,
  removeRule,
  currency,
}) {
  return (
    <div className="policy-rules-section">
      <div className="policy-rules-heading">
        <div>
          <span className="detail-kicker">REIMBURSEMENT RULES</span>
          <strong>
            {rules.length} rule{rules.length === 1 ? "" : "s"}
          </strong>
        </div>
        {editable && (
          <button
            type="button"
            className="secondary-button compact-secondary"
            onClick={addRule}
          >
            + Add rule
          </button>
        )}
      </div>

      <div className="policy-rule-table">
        <div className="policy-rule-row policy-rule-header">
          <span>Grade</span>
          <span>Category</span>
          <span>Allowed</span>
          <span>Maximum ({currency || "USD"})</span>
          <span />
        </div>

        {rules.map((rule, index) => (
          <div
            className="policy-rule-row"
            key={`${rule.employee_grade}-${rule.category}-${index}`}
          >
            {editable ? (
              <input
                value={rule.employee_grade}
                onChange={(event) =>
                  updateRule(
                    index,
                    "employee_grade",
                    event.target.value,
                  )
                }
              />
            ) : (
              <strong>{rule.employee_grade}</strong>
            )}

            {editable ? (
              <input
                value={rule.category}
                onChange={(event) =>
                  updateRule(index, "category", event.target.value)
                }
              />
            ) : (
              <span>{rule.category}</span>
            )}

            {editable ? (
              <label className="policy-toggle">
                <input
                  type="checkbox"
                  checked={Boolean(rule.allowed)}
                  onChange={(event) =>
                    updateRule(
                      index,
                      "allowed",
                      event.target.checked,
                    )
                  }
                />
                <span>{rule.allowed ? "Allowed" : "Blocked"}</span>
              </label>
            ) : (
              <StatusBadge
                status={rule.allowed ? "ALLOWED" : "NOT_ALLOWED"}
              />
            )}

            {editable ? (
              <input
                type="number"
                min="0"
                step="0.01"
                disabled={!rule.allowed}
                value={rule.max_amount}
                onChange={(event) =>
                  updateRule(
                    index,
                    "max_amount",
                    event.target.value,
                  )
                }
              />
            ) : (
              <strong>
                {rule.allowed
                  ? formatMoney(rule.max_amount, currency)
                  : "—"}
              </strong>
            )}

            {editable ? (
              <button
                type="button"
                className="icon-button"
                onClick={() => removeRule(index)}
                aria-label="Remove policy rule"
              >
                ×
              </button>
            ) : (
              <span />
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function ReportList({
  reports,
  onOpen,
  activeReportID,
  admin = false,
}) {
  return (
    <div
      className={
        admin
          ? "report-list finance-report-list"
          : "report-list employee-report-list"
      }
    >
      {reports.map((report) => {
        const itemCount =
          report.item_count ?? report.items?.length ?? 0;
        const currency =
          report.items?.find((item) => item.currency)?.currency ||
          "USD";
        const reviewItemCount = (report.items || []).filter(
          (item) => {
            const status = String(
              item.status || "",
            ).toUpperCase();

            return (
              reviewStatuses.has(status) ||
              actionStatuses.has(status)
            );
          },
        ).length;
        const active =
          report.report_id === activeReportID;

        return (
          <button
            type="button"
            className={[
              "report-summary-card",
              admin
                ? "finance-report-card"
                : "employee-report-card",
              active ? "active" : "",
            ]
              .filter(Boolean)
              .join(" ")}
            key={report.report_id}
            onClick={() => onOpen(report.report_id)}
          >
            <div className="report-card-top">
              <div className="report-card-identity">
                <div
                  className={`report-card-mark ${statusClass(
                    report.status,
                  )}`}
                >
                  {admin ? "F" : "R"}
                </div>

                <div className="report-card-title">
                  <strong>
                    {report.title || "Expense report"}
                  </strong>

                  {admin ? (
                    <span>
                      {report.employee?.name || "Employee"}
                      {report.employee?.id
                        ? ` · ${report.employee.id}`
                        : ""}
                    </span>
                  ) : (
                    <span>
                      {formatDateTime(report.created_at)}
                    </span>
                  )}
                </div>
              </div>

              <StatusBadge status={report.status} />
            </div>

            {!admin && report.purpose && (
              <p className="report-card-purpose">
                {report.purpose}
              </p>
            )}

            {admin && (
              <div
                className={
                  reviewItemCount > 0
                    ? "finance-review-signal active"
                    : "finance-review-signal"
                }
              >
                <span className="finance-review-signal-dot" />
                <span>
                  {reviewItemCount > 0
                    ? `${reviewItemCount} receipt${
                        reviewItemCount === 1 ? "" : "s"
                      } need attention`
                    : "No receipt currently needs manual action"}
                </span>
              </div>
            )}

            <div className="report-card-metrics">
              <ReportCardMetric
                label="Receipts"
                value={String(itemCount)}
              />
              <ReportCardMetric
                label="Claimed"
                value={formatMoney(
                  report.total_claimed,
                  currency,
                )}
              />
              <ReportCardMetric
                label="Reimbursable"
                value={formatMoney(
                  report.total_reimbursable,
                  currency,
                )}
                positive
              />
            </div>

            <div className="report-card-footer">
              <span>
                {admin
                  ? formatDateTime(report.created_at)
                  : shortID(report.report_id)}
              </span>

              <strong>
                {admin ? "Open review" : "View report"}
                <span aria-hidden="true"> →</span>
              </strong>
            </div>
          </button>
        );
      })}
    </div>
  );
}

function ReportCardMetric({
  label,
  value,
  positive = false,
}) {
  return (
    <div className="report-card-metric">
      <span>{label}</span>
      <strong className={positive ? "positive" : ""}>
        {value}
      </strong>
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
      {aside && <div>{aside}</div>}
    </div>
  );
}

function KpiCard({ label, value, tone }) {
  return (
    <div className={`kpi-card ${tone || ""}`}>
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
    <div className="detail">
      <span>{label}</span>
      <strong className={strong ? "detail-strong" : ""}>
        {value ?? "—"}
      </strong>
    </div>
  );
}

function SummaryMetric({
  label,
  value,
  mono = false,
  accent = false,
}) {
  return (
    <div className={accent ? "summary-metric accent" : "summary-metric"}>
      <span>{label}</span>
      <strong className={mono ? "mono" : ""}>{value}</strong>
    </div>
  );
}

function EvidenceRow({ label, value, mono = false }) {
  return (
    <div className="evidence-row">
      <span>{label}</span>
      <strong className={mono ? "mono" : ""}>
        {value || "—"}
      </strong>
    </div>
  );
}

function TraceStep({ label, value, state }) {
  return (
    <div className={`trace-step ${state || ""}`}>
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
    <div className="global-error">
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
      <div className="empty-icon">◎</div>
      <h3>Your report result will appear here</h3>
      <p>
        Submit one or more receipts to see reimbursement status,
        policy outcome and any action required from you.
      </p>
    </div>
  );
}

function EmptyState({ title, text, compact = false }) {
  return (
    <div className={compact ? "empty-state compact" : "empty-state"}>
      <div className="empty-state-icon">○</div>
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

  if (
    [
      "APPROVED",
      "COMPLETED",
      "YES",
      "PUBLISHED",
      "ALLOWED",
    ].includes(value)
  ) {
    return "success";
  }

  if (
    [
      "LIMIT_EXCEEDED",
      "CLARIFICATION_REQUIRED",
      "ACTION_REQUIRED",
      "MANUAL_REVIEW",
      "REVIEW_REQUIRED",
      "HUMAN_REVIEW_REQUIRED",
      "CURRENCY_MISMATCH",
      "DRAFT",
    ].includes(value)
  ) {
    return "warning";
  }

  if (
    [
      "REJECTED",
      "DUPLICATE_DETECTED",
      "NOT_ALLOWED",
      "NO",
    ].includes(value)
  ) {
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

function activePolicyToRules(policy) {
  const rules = [];

  Object.entries(policy?.limits || {}).forEach(
    ([employeeGrade, categories]) => {
      Object.entries(categories || {}).forEach(
        ([category, rule]) => {
          rules.push({
            employee_grade: employeeGrade,
            category,
            allowed: Boolean(rule?.allowed),
            max_amount: Number(rule?.max_amount || 0),
          });
        },
      );
    },
  );

  return rules.sort((a, b) => {
    const gradeCompare = a.employee_grade.localeCompare(
      b.employee_grade,
    );

    if (gradeCompare !== 0) {
      return gradeCompare;
    }

    return a.category.localeCompare(b.category);
  });
}

function sanitizePolicyRules(rules) {
  return (rules || []).map((rule) => ({
    employee_grade: String(rule.employee_grade || "").trim(),
    category: String(rule.category || "").trim(),
    allowed: !!rule.allowed,
    max_amount: rule.allowed
      ? Number(rule.max_amount || 0)
      : 0,
  }));
}

function employeeStatusMessage(status) {
  const value = String(status || "").toUpperCase();

  const messages = {
    APPROVED: "This expense is within policy and is fully reimbursable.",
    LIMIT_EXCEEDED:
      "This expense is valid, but reimbursement is capped by policy.",
    CLARIFICATION_REQUIRED:
      "We need one answer from you before the claim can continue.",
    MANUAL_REVIEW:
      "Finance needs to verify this claim before any reimbursement is authorized.",
    HUMAN_REVIEW_REQUIRED:
      "The document needs human verification before reimbursement.",
    DUPLICATE_DETECTED:
      "This expense was identified as a duplicate and will not be reimbursed.",
    REJECTED:
      "This claim was rejected and has no reimbursement.",
    NOT_ALLOWED:
      "This category is not reimbursable under the applicable policy.",
    CURRENCY_MISMATCH:
      "Finance needs to review the currency before reimbursement.",
  };

  return messages[value] || "Your claim has been processed.";
}

function shortModelName(model) {
  if (!model) {
    return "—";
  }

  return String(model).replace(":instruct", "");
}

function formatMoney(amount, currency) {
  const numeric = Number(amount);

  if (!Number.isFinite(numeric)) {
    return "—";
  }

  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: currency || "USD",
      maximumFractionDigits: 2,
    }).format(numeric);
  } catch {
    return `${currency || ""} ${numeric.toFixed(2)}`.trim();
  }
}

function formatFileSize(bytes) {
  const value = Number(bytes) || 0;

  if (value < 1024) {
    return `${value} B`;
  }

  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }

  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDateTime(value) {
  if (!value) {
    return "—";
  }

  const date = new Date(value);

  if (Number.isNaN(date.getTime())) {
    return String(value);
  }

  return new Intl.DateTimeFormat("en-IN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

function humanize(value) {
  if (!value) {
    return "—";
  }

  return String(value)
    .replace(/_/g, " ")
    .toLowerCase()
    .replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function shortID(value) {
  const text = String(value || "");

  if (!text) {
    return "—";
  }

  if (text.length <= 18) {
    return text;
  }

  return `${text.slice(0, 8)}…${text.slice(-7)}`;
}

function fileExtension(name) {
  const parts = String(name || "").split(".");

  if (parts.length < 2) {
    return "FILE";
  }

  return parts.pop().toUpperCase();
}

function todayDateString() {
  const date = new Date();
  const local = new Date(
    date.getTime() - date.getTimezoneOffset() * 60000,
  );
  return local.toISOString().slice(0, 10);
}

function safeJSONParse(value) {
  try {
    return JSON.parse(value);
  } catch {
    return null;
  }
}

export default App;
