# Backend API Follow-Ups

This file records only the remaining design decisions after the final API
sweep.

Source of truth order for the current backend remains:

1. `backend/server/server.go`
2. routed handlers, DTOs, and focused tests under `backend/handlers/**`
3. `backend/docs/openapi.yaml`
4. this file

As of `2026-04-23`, the current code/spec pair has been validated for:

- OpenAPI parsing plus kin-openapi structural checks
- embedded `/openapi.yaml` and `/swagger/` serving
- registered route inventory versus `backend/docs/openapi.yaml`
- the shared public `reason` vocabulary used by the swept handlers
- runtime serving for the public docs/config/metrics/market-read surface

This file should not be used to track routine route-level drift. If a future
task finds a real implementation/spec mismatch, fix the code or
`backend/docs/openapi.yaml` first and then update this file only if the issue
is truly a deferred design decision.

## Remaining Deferred Decisions

Only the following API decisions remain deferred at this stage.

### 1. Limited-Scope Token Login Redesign

Current implementation:

- `POST /v0/login` returns a normal bearer token plus `mustChangePassword`.
- Protected handlers enforce the password-change gate through the current auth
  service and middleware path.
- `POST /v0/changepassword` still accepts an authenticated request using the
  current token-validation contract.

Why it remains deferred:

- The existing login/auth contract is now documented and validated as-is.
- The remaining question is product/security design: whether first-login users
  should receive a limited-scope or short-lived token instead of the current
  normal bearer token.

Decision for this wave:

- Keep the existing login contract and OpenAPI shape.
- Do not redesign token issuance in this sweep.

### 2. Public Route Reorganization

Current implementation:

- `backend/server/server.go` is the live route inventory source of truth.
- `backend/docs/openapi.yaml` reflects the current monolith route layout,
  including the supported legacy aliases that still exist in code.

Why it remains deferred:

- Any CRUD-style or resource-layout rewrite would be a coordinated product and
  client migration, not a documentation repair.

Decision for this wave:

- Keep documenting the live route structure exactly as implemented.
- Do not reopen route-taxonomy redesign work here.

### 3. Bets-To-Trades Rename

Current implementation:

- The API still uses bets terminology in routes, tags, and schemas, including
  `/v0/markets/bets/{marketId}`, `/v0/bet`, and `/v0/sell`.

Why it remains deferred:

- The rename is still a cross-cutting compatibility change with client and
  documentation impact.

Decision for this wave:

- Keep the existing bets terminology as the canonical current contract.
- Do not partially rename routes, tags, or schemas in this task.

## Non-Goals For This File

The following are not active issues for this document:

- re-auditing completed endpoint sweeps
- reintroducing speculative CRUD redesign notes
- bundling new backend behavior changes into a documentation-only follow-up

If future work changes the backend or API contract, update the code and
`backend/docs/openapi.yaml` first, then revise this file to match the new
implemented state.
