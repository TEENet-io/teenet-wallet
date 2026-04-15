# Data Model

## Database Schema

SQLite with WAL mode. Tables are auto-migrated on startup via GORM:

| Table | Model | Purpose |
|-------|-------|---------|
| `users` | `User` | Registered users |
| `passkey_credentials` | `PasskeyCredential` | WebAuthn credentials |
| `api_keys` | `APIKey` | API keys (hashed, prefixed `ocw_`) |
| `wallets` | `Wallet` | Wallets with chain, address, public key |
| `approval_policies` | `ApprovalPolicy` | USD thresholds and daily limits |
| `approval_requests` | `ApprovalRequest` | Pending/approved/rejected approvals |
| `allowed_contracts` | `AllowedContract` | Contract whitelist per wallet |
| `audit_logs` | `AuditLog` | Full operation audit trail |
| `idempotency_records` | `IdempotencyRecord` | Idempotency-Key cache (24h TTL) |
| `address_book_entries` | `AddressBookEntry` | Saved contacts per user/chain (unique nickname) |
| `custom_chains` | `CustomChain` | User-added EVM chains |

## GORM Patterns

- **AutoMigrate on startup** -- schema changes are applied automatically when the server starts. There are no migration files.
- **WAL mode** -- SQLite is configured with Write-Ahead Logging for concurrent read performance.
- **No repository layer** -- handlers use GORM directly; the codebase is simple enough that an abstraction layer adds no value.

## Chain Configuration

- **Built-in chains** are loaded from `chains.json` at startup.
- **Custom chains** added via `POST /api/chains` are stored in the `custom_chains` table and merged with built-in chains at load time.
- See [chains.json Schema](chains-schema.md) for the full field reference.
