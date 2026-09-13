# Admin API Key Issuance

## Goal

Add a low-coupling admin workflow that creates an API key for an existing active
user without requiring that user to log in. The admin should receive the
plaintext key once together with copyable usage instructions.

## Design

- Add `POST /admin/api-keys/issue` beside the existing admin API key endpoint.
- Keep key validation, persistence, cache invalidation, group binding, quota,
  expiry, and rate-limit behavior in `APIKeyService.Create`.
- Inject a narrow API-key creation interface into `AdminAPIKeyHandler` so the
  new handler remains easy to test and future upstream merges stay localized.
- Use the existing admin authentication, audit, compliance, rate limiting, and
  idempotency middleware/helpers.
- Add a standalone admin page, API client method, sidebar entry, route, and
  English/Chinese locale files.
- Do not add a new database table, user login path, or user-facing route.

## Verification

- Backend handler tests cover successful issuance, request forwarding,
  inactive-user rejection, and idempotent request wiring.
- Frontend typecheck/build and locale completeness checks.
- Review the final diff for unrelated changes and upstream merge surface.
