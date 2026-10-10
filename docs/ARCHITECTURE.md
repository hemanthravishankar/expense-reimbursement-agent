# Architecture

## Core design principle

> **LLM understands/proposes; deterministic Go code verifies/enforces.**

The model is an extraction component. It is not the policy engine, fraud authority, or reimbursement approver.

## Component architecture

```mermaid
flowchart LR
    subgraph Users
      EMP["Employee"]
      FIN["Finance Reviewer"]
    end

    subgraph Web
      UI["React / Vite"]
      NX["Nginx"]
    end

    subgraph Backend["Go Backend"]
      API["REST API"]
      REPORT["Report Service"]
      WF["Workflow Processor"]
      VALID["Validation Engine"]
      DUP["Duplicate / Risk Engine"]
      CLARIFY["Clarification Service"]
      FINREV["Finance Review Service"]
      POLICY["Policy Engine / Manager"]
      AUDIT["Audit Writer"]
    end

    subgraph AI
      OLLAMA["Ollama"]
      Q4["Qwen3-VL 4B"]
      Q8["Qwen3-VL 8B"]
    end

    subgraph Data
      PG["PostgreSQL"]
      DOC["DocumentStore"]
      JSON["Audit JSON"]
    end

    EMP --> UI
    FIN --> UI
    UI --> NX
    NX --> API

    API --> REPORT
    API --> CLARIFY
    API --> FINREV
    API --> POLICY

    REPORT --> WF
    WF --> VALID
    WF --> DUP
    WF --> OLLAMA
    OLLAMA --> Q4
    OLLAMA --> Q8
    WF --> DOC
    WF --> AUDIT

    REPORT --> PG
    CLARIFY --> PG
    FINREV --> PG
    POLICY --> PG
    AUDIT --> JSON
```

## Receipt decision flow

```mermaid
flowchart TD
    A["Upload receipt"] --> B["SHA-256 + MIME detection"]
    B --> C{"Same employee exact SHA?"}

    C -->|Yes| D["DUPLICATE_DETECTED\nAI skipped\n$0 automatic reimbursement"]
    C -->|No| E["Persist original evidence"]

    E --> F["Normalize JPG/PNG/PDF"]
    F --> G["Qwen3-VL 4B extraction"]
    G --> H["Normalize + deterministic validation"]

    H -->|Invalid| I["Fresh Qwen3-VL 8B extraction"]
    I --> J["Normalize + deterministic re-validation"]

    J -->|Still invalid| K["HUMAN_REVIEW_REQUIRED"]
    H -->|Valid| L["Employee-aware metadata duplicate detection"]
    J -->|Valid| L

    L --> M{"Cross-employee / admin-review gate?"}
    M -->|Yes| N["MANUAL_REVIEW\npolicy_decision=nil\n$0 pending review"]
    M -->|No| O{"Same-employee metadata match?"}

    O -->|Yes| P["CLARIFICATION_REQUIRED"]
    P --> Q{"Employee answer"}

    Q -->|NO| R["DUPLICATE_DETECTED\n$0"]
    Q -->|"YES + strong identity"| N
    Q -->|"YES + weaker similarity"| S["Load active policy"]

    O -->|No| S
    S --> T["Deterministic policy evaluation"]
    T --> U["APPROVED / LIMIT_EXCEEDED /\nNOT_ALLOWED / CURRENCY_MISMATCH /\nMANUAL_REVIEW"]

    N --> V["Finance review"]
    V -->|Reject| W["REJECTED\n$0"]
    V -->|Approve| S

    D --> X["Persist/audit"]
    K --> X
    R --> X
    U --> X
    W --> X
```

## Trust boundaries

### AI trust boundary

AI is allowed to propose document facts:

- merchant;
- date;
- money fields;
- receipt number;
- payment method;
- category suggestion;
- line items;
- confidence/warnings.

AI is not allowed to make the final reimbursement authorization.

### Employee trust boundary

Employee clarification is input to the workflow, not an override.

A strong same-employee transaction identity cannot self-approve merely because the employee answers `YES`.

### Finance trust boundary

Finance determines whether a reviewed case is legitimate enough to continue or should be rejected.

Finance approval still goes through the active deterministic policy.

### Policy trust boundary

The reimbursement amount comes from a policy version and deterministic Go evaluation.

A published policy change affects future decisions without rewriting historical decisions.

## Persistence separation

```mermaid
flowchart LR
    APP["Application"] --> PG["PostgreSQL\nbusiness state"]
    APP --> DOC["DocumentStore\noriginal evidence"]
    APP --> AUD["Audit JSON\ntechnical execution"]

    PG --> C["Claims / reports / events / policies"]
    DOC --> E["Original uploaded files"]
    AUD --> T["Hash / model / validation / duplicate / policy / status"]
```

This separation prevents technical execution records from being confused with business lifecycle state.

## Report aggregation

A report contains child claims.

Simplified parent status rules:

```text
Any CLARIFICATION_REQUIRED
-> ACTION_REQUIRED

Any MANUAL_REVIEW / HUMAN_REVIEW_REQUIRED / CURRENCY_MISMATCH
-> REVIEW_REQUIRED

Accepted + rejected mixture
-> PARTIALLY_REJECTED

All rejected
-> REJECTED

Otherwise all final
-> COMPLETED
```

`APPROVED` and `LIMIT_EXCEEDED` are accepted/final receipt outcomes.

## Deployment architecture

```mermaid
flowchart LR
    B["Browser"] --> F["Frontend + Nginx\nDocker"]
    F --> A["Go API\nDocker"]
    A --> P["PostgreSQL 16\nDocker"]
    A --> O["Ollama\nHost machine"]
    A --> V["Named volume\nreceipt evidence + audit"]
```

The recommended macOS/Apple Silicon deployment deliberately keeps Ollama on the host for native hardware acceleration.
