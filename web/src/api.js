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

async function jsonRequest(path, options = {}) {
  const response = await fetch(`${API_BASE_URL}${path}`, options);
  return parseResponse(response);
}

export async function getHealth() {
  return jsonRequest("/healthz");
}

export async function analyzeExpense(formData) {
  return jsonRequest("/api/v1/expenses/analyze", {
    method: "POST",
    body: formData,
  });
}

export async function createExpenseReport(formData) {
  return jsonRequest("/api/v1/expense-reports", {
    method: "POST",
    body: formData,
  });
}

export async function getExpenseReport(reportID) {
  return jsonRequest(
    `/api/v1/expense-reports/${encodeURIComponent(reportID)}`,
  );
}

export async function listEmployeeExpenseReports(employeeID) {
  return jsonRequest(
    `/api/v1/employees/${encodeURIComponent(
      employeeID,
    )}/expense-reports`,
  );
}

export async function listAdminExpenseReports() {
  return jsonRequest("/api/v1/admin/expense-reports");
}

export async function getClaim(claimID) {
  return jsonRequest(
    `/api/v1/claims/${encodeURIComponent(claimID)}`,
  );
}

export async function getClaimEvents(claimID) {
  return jsonRequest(
    `/api/v1/claims/${encodeURIComponent(claimID)}/events`,
  );
}

export function getClaimDocumentURL(claimID) {
  if (!claimID) {
    return "";
  }

  return `${API_BASE_URL}/api/v1/claims/${encodeURIComponent(
    claimID,
  )}/document`;
}

export async function resolveClarification(claimID, payload) {
  return jsonRequest(
    `/api/v1/claims/${encodeURIComponent(
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
}

export async function reviewClaim(claimID, payload) {
  return jsonRequest(
    `/api/v1/admin/claims/${encodeURIComponent(
      claimID,
    )}/review`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    },
  );
}

export async function listPolicies() {
  return jsonRequest("/api/v1/admin/policies");
}

export async function getActivePolicy() {
  return jsonRequest("/api/v1/admin/policies/active");
}

export async function getPolicy(policyID) {
  return jsonRequest(
    `/api/v1/admin/policies/${encodeURIComponent(policyID)}`,
  );
}

export async function createPolicy(payload) {
  return jsonRequest("/api/v1/admin/policies", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
}

export async function updatePolicy(policyID, payload) {
  return jsonRequest(
    `/api/v1/admin/policies/${encodeURIComponent(policyID)}`,
    {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    },
  );
}

export async function publishPolicy(policyID, payload) {
  return jsonRequest(
    `/api/v1/admin/policies/${encodeURIComponent(
      policyID,
    )}/publish`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    },
  );
}
