const API_BASE_URL =
  import.meta.env.VITE_API_BASE_URL || "http://localhost:8080";

async function parseResponse(response) {
  const text = await response.text();

  let body = null;

  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = { error: text };
    }
  }

  if (!response.ok) {
    throw new Error(
      body?.error || `Request failed with HTTP ${response.status}`,
    );
  }

  return body;
}

export async function getHealth() {
  const response = await fetch(`${API_BASE_URL}/healthz`);
  return parseResponse(response);
}

export async function analyzeExpense(formData) {
  const response = await fetch(
    `${API_BASE_URL}/api/v1/expenses/analyze`,
    {
      method: "POST",
      body: formData,
    },
  );

  return parseResponse(response);
}

export async function createExpenseReport(formData) {
  const response = await fetch(
    `${API_BASE_URL}/api/v1/expense-reports`,
    {
      method: "POST",
      body: formData,
    },
  );

  return parseResponse(response);
}

export async function getExpenseReport(reportID) {
  const response = await fetch(
    `${API_BASE_URL}/api/v1/expense-reports/${encodeURIComponent(
      reportID,
    )}`,
  );

  return parseResponse(response);
}

export async function listEmployeeExpenseReports(employeeID) {
  const response = await fetch(
    `${API_BASE_URL}/api/v1/employees/${encodeURIComponent(
      employeeID,
    )}/expense-reports`,
  );

  return parseResponse(response);
}

export async function listAdminExpenseReports() {
  const response = await fetch(
    `${API_BASE_URL}/api/v1/admin/expense-reports`,
  );

  return parseResponse(response);
}

export async function getClaim(claimID) {
  const response = await fetch(
    `${API_BASE_URL}/api/v1/claims/${encodeURIComponent(claimID)}`,
  );

  return parseResponse(response);
}

export async function getClaimEvents(claimID) {
  const response = await fetch(
    `${API_BASE_URL}/api/v1/claims/${encodeURIComponent(
      claimID,
    )}/events`,
  );

  return parseResponse(response);
}

export async function resolveClarification(claimID, payload) {
  const response = await fetch(
    `${API_BASE_URL}/api/v1/claims/${encodeURIComponent(
      claimID,
    )}/clarifications`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    },
  );

  return parseResponse(response);
}
