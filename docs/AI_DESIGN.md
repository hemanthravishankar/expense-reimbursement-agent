# AI Design

## Role of AI

AI is used for **document understanding**, not money authorization.

The system asks the vision model to extract a structured representation of the receipt/invoice. Everything that can directly affect reimbursement is then checked by deterministic code.

> **AI output is evidence to validate, not an approval decision.**

## Models

### Primary

```text
qwen3-vl:4b-instruct
```

Used for the first extraction because it is smaller and faster.

### Fallback

```text
qwen3-vl:8b-instruct
```

Used only if deterministic validation rejects the primary extraction.

## Inference settings

Receipt extraction uses:

- JSON output mode;
- non-streaming response;
- `think=false`;
- temperature `0`;
- bounded generation length.

These settings aim to make the output more constrained and reproducible.

## Extraction schema

The model returns fields such as:

```json
{
  "document_type": "receipt",
  "merchant": "",
  "expense_date": "",
  "subtotal": 0,
  "tax_amount": 0,
  "service_charge": 0,
  "tip_amount": 0,
  "discount_amount": 0,
  "other_fees": 0,
  "total_amount": 0,
  "currency": "",
  "receipt_number": "",
  "payment_method": "",
  "category_suggestion": "",
  "line_items": [
    {
      "name": "",
      "quantity": 0,
      "amount": 0
    }
  ],
  "confidence": 0,
  "warnings": []
}
```

## Prompt constraints

The extraction prompt instructs the model to:

- return JSON only;
- not invent invisible information;
- normalize dates to `YYYY-MM-DD`;
- use ISO currency codes;
- keep financial components separate;
- treat multiple pages as one expense document;
- avoid counting repeated totals twice;
- extract genuine line items only.

## Transportation-specific handling

Ride receipts often contain ambiguous lines.

The prompt and deterministic normalizer distinguish:

- fare components -> subtotal;
- tax-only rows -> tax;
- airport/booking/convenience/platform/parking/toll/surcharge -> other fees;
- combined `Taxes & Other Fees` -> `other_fees` if the split is not reliable.

The system does not invent a tax split just to balance arithmetic.

## Deterministic validation

The main money identity is:

```text
subtotal
+ tax
+ service charge
+ tip
+ other fees
- discount
= total
```

Line-item totals must also agree with subtotal when line items are present.

The validator uses a small money tolerance to handle normal decimal representation.

## Why the fallback is fresh

A common failure mode in multi-model pipelines is to let the second model inherit a flawed first answer.

This project instead performs:

```text
original document
    -> 4B extraction
    -> deterministic validation
    -> if invalid:
       original document
       -> fresh 8B extraction
       -> deterministic validation
```

The fallback therefore has a chance to independently recover from the first extraction.

## What AI does not decide

AI does not decide:

- whether an exact binary is a duplicate;
- whether a Finance gate is required;
- whether an employee's clarification overrides strong identity;
- the reimbursement limit;
- the reimbursable amount;
- whether a policy version is active;
- Finance approval/rejection.

Those are deterministic application or human-review decisions.

## Model endpoint configuration

Default local endpoint:

```text
http://localhost:11434/api/chat
```

For containerized deployment the client can resolve a host or remote endpoint through:

```text
OLLAMA_BASE_URL
OLLAMA_URL
```

This keeps the model-serving boundary replaceable without changing the workflow.
