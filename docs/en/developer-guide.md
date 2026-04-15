# Developer Guide

This guide is for developers who want to build, test, and contribute to the teenet-wallet codebase.

---

## Build & Run

**Prerequisites:** Go 1.24+, SQLite3 dev headers, a running TEENet service node on port 8089.

```bash
# Build
make build

# Run (defaults: port 8080, data in /data, consensus at localhost:8089)
./teenet-wallet

# Or with Docker
make docker
docker run -p 8080:8080 \
  -e SERVICE_URL=http://host.docker.internal:8089 \
  -v wallet-data:/data \
  teenet-wallet:latest
```

All configuration is via environment variables -- see [Configuration](configuration.md) for the full list.

---

## Project Structure

```
teenet-wallet/
├── main.go              # Entry point: routes, middleware, DI wiring
├── handler/             # HTTP handlers (one file per domain)
│   ├── auth.go          # Passkey registration/login, API key CRUD, account deletion
│   ├── wallet.go        # Wallet CRUD, transfer, wrap/unwrap SOL, policy, daily-spent
│   ├── contract.go      # Contract whitelist add/update/delete
│   ├── contract_call.go # General contract calls (EVM + Solana), approve-token, revoke-approval
│   ├── sse.go           # SSE event stream endpoint
│   ├── sse_hub.go       # Per-user SSE pub/sub hub
│   ├── approval.go      # Approval list/detail/approve/reject, post-approval execution
│   ├── balance.go       # On-chain balance queries (native + ERC-20 + SPL)
│   ├── addressbook.go   # Address book CRUD, nickname resolution
│   ├── audit.go         # Audit log queries + writeAuditCtx helper
│   ├── price.go         # PriceService: CoinGecko + Jupiter price feeds, TTL cache
│   ├── middleware.go     # Auth middleware (API key + Passkey session), CORS, CSP
│   ├── ratelimit.go     # Per-key and per-IP rate limiting
│   ├── idempotency.go   # Idempotency-Key store (24h TTL, per-user)
│   ├── helpers.go       # Shared utils: isPasskeyAuth, authInfo, createPendingApproval
│   └── response.go      # JSON error/success response helpers
├── model/               # GORM models (SQLite)
│   ├── wallet.go        # Wallet, ChainConfig, CustomChain, LoadChains()
│   ├── user.go          # User, PasskeyCredential
│   ├── apikey.go        # APIKey
│   ├── policy.go        # ApprovalPolicy, ApprovalRequest
│   ├── contract.go      # AllowedContract
│   ├── addressbook.go   # AddressBookEntry
│   ├── audit.go         # AuditLog
│   └── idempotency.go   # IdempotencyRecord
├── chain/               # Blockchain interaction (no DB, no HTTP -- pure chain logic)
│   ├── rpc.go           # EVM JSON-RPC + Solana RPC clients
│   ├── tx_eth.go        # Build EIP-1559 transactions, encode transfer calldata
│   ├── tx_sol.go        # Build Solana transactions (native, SPL, wrap/unwrap, program call)
│   ├── abi.go           # Solidity ABI encoder (supports all types including tuples)
│   ├── address.go       # Address derivation from public keys (EVM + Solana)
│   └── nonce.go         # EVM nonce manager for concurrent tx safety
├── frontend/
│   └── index.html       # Single-file SPA (vanilla JS, no build step)
├── skill/
│   └── tee-wallet/
│       └── SKILL.md     # OpenClaw skill definition (REST-based)
├── plugin/              # OpenClaw plugin (TypeScript, native tool integration)
│   ├── index.ts         # Plugin entry: registers tools + SSE approval watcher
│   ├── openclaw.plugin.json  # Plugin manifest (id, config schema, skills)
│   ├── src/
│   │   ├── api-client.ts       # Wallet backend HTTP client
│   │   ├── approval-watcher.ts # SSE subscription + subagent.run() notifications
│   │   ├── tools/              # Tool definitions (wallet, transfer, contract, policy, ...)
│   │   └── __tests__/          # Unit + E2E tests (node --test)
│   └── skill/tee-wallet/       # Agent instructions bundled with the plugin
├── docs/                # Docsify documentation site
├── Makefile             # build, test, lint, docker, clean
├── Dockerfile
└── chains.json          # Default chain configuration
```

### Key Design Decisions

- **Single binary** -- no microservices, everything in one Go process. SQLite for storage.
- **No ORM queries in handlers** -- handlers use GORM directly (simple enough that a repository layer adds no value).
- **GORM AutoMigrate** -- schema changes are applied automatically on startup. No migration files.
- **Single-file frontend** -- `frontend/index.html` is a complete SPA with no build tooling. Embedded via `gin.Static`.
- **TEENet SDK** -- signing goes through `github.com/TEENet-io/teenet-sdk/go`, which talks to the local TEENet service node.

---

## Testing

```bash
# Run all tests
make test

# Run handler tests only
go test ./handler/ -v

# Run a specific test
go test ./handler/ -run TestTransfer_ERC20 -v

# Lint
make lint
```

Tests use in-memory SQLite (`file::memory:`) and don't require a running TEENet service node -- the SDK client is nil in tests, and signing calls are expected to fail (tests verify behavior up to the signing step).

### Mock TEENet Service

For end-to-end local development without a real TEENet service, the [teenet-sdk](https://github.com/TEENet-io/teenet-sdk) ships a mock server (under [`mock-server/`](https://github.com/TEENet-io/teenet-sdk/tree/main/mock-server)) that implements the full TEENet HTTP API with real cryptographic signing. Point `SERVICE_URL` at it and the wallet behaves as if talking to production.

```bash
git clone https://github.com/TEENet-io/teenet-sdk.git
cd teenet-sdk/mock-server
go build && ./mock-server                                  # 127.0.0.1:8089
# For a custom port/bind: MOCK_SERVER_PORT=xxxx MOCK_SERVER_BIND=0.0.0.0 ./mock-server
# Note: if you change the port, update SERVICE_URL below to match.
```

Then run the wallet with:

```bash
SERVICE_URL=http://127.0.0.1:8089 ./teenet-wallet
```

**What it provides (34 endpoints):**

- **Core signing & keys** -- `/api/health`, `/api/publickeys/:app_instance_id`, `/api/submit-request`, `/api/generate-key`, `/api/apikey/:name`, `/api/apikey/:name/sign`
- **Voting cache** -- `/api/cache/:hash`, `/api/cache/status`, `/api/config/:app_instance_id`
- **Approval bridge** -- `/api/auth/passkey/options|verify|verify-as`, `/api/approvals/request/init`, `/api/approvals/request/:id/challenge|confirm`, `/api/approvals/:taskId/challenge|action`, `/api/approvals/pending`, `/api/requests/mine`, `/api/signature/by-tx/:txId`
- **Admin bridge** -- passkey user invite/list/delete, audit records, permission policy CRUD, public key / API key admin, passkey registration

**Signing modes** (selected by `app_instance_id`):

| Mode | Example app | Behavior |
|------|-------------|----------|
| Direct | `test-ecdsa-secp256k1`, `ethereum-wallet-app` | Signs immediately, returns `{status: "signed", signature}` |
| Voting | `test-voting-2of3` | First call returns `pending`; sign completes after 2 votes from distinct instances |
| Approval | `test-approval-required` | Returns `pending_approval` + `tx_id`; requires passkey approve via `/api/approvals/:taskId/action` |

**Pre-configured test apps:**

| App Instance ID | Protocol | Curve | Mode |
|-----------------|----------|-------|------|
| test-schnorr-ed25519 | schnorr | ed25519 | Direct |
| test-schnorr-secp256k1 | schnorr | secp256k1 | Direct |
| test-ecdsa-secp256k1 | ecdsa | secp256k1 | Direct |
| test-ecdsa-secp256r1 | ecdsa | secp256r1 | Direct |
| ethereum-wallet-app | ecdsa | secp256k1 | Direct |
| secure-messaging-app | schnorr | ed25519 | Direct |
| test-voting-2of3 | ecdsa | secp256k1 | Voting (2-of-N) |
| test-approval-required | ecdsa | secp256k1 | Approval |

Pre-seeded passkey users: **Alice** (ID=1) and **Bob** (ID=2), bound to `test-approval-required`.

> **About `app_instance_id`.** The protocol/curve column above only describes the **initial key** bound to each test app. An `app_instance_id` is not locked to one chain or one key — call `POST /api/generate-key` against any id to mint additional keys with whatever protocol/curve you need, then sign with them via `POST /api/submit-request`. Pick whichever id matches the mode you want to test (Direct / Voting / Approval); the curve of the *initial* key only matters if you sign with the default key.

**Hashing responsibility** (matches TEE-DAO backend):

| Protocol | Curve | Who hashes? |
|----------|-------|-------------|
| ECDSA | secp256k1 / secp256r1 | **Caller** -- pass a 32-byte hash (Keccak-256 for Ethereum, SHA-256 otherwise) |
| Schnorr | secp256k1 | Mock server (SHA-256 internally) |
| Schnorr | ed25519 | EdDSA protocol (SHA-512 internally) |
| HMAC | -- | HMAC (SHA-256 internally) |

**Limitations:** in-memory only (state resets on restart), deterministic private keys for reproducible signatures, approval tokens use a random HMAC secret with a 30-minute TTL. **Do not use in production.**

### Writing Tests

Pattern used throughout:

```go
func TestSomething(t *testing.T) {
    db := setupTestDB(t)  // in-memory SQLite with AutoMigrate
    wh := handler.NewWalletHandler(db, nil, "http://localhost:8080")
    r := gin.New()
    r.Use(handler.FakeAuth(userID))  // inject auth context
    r.POST("/wallets/:id/transfer", wh.Transfer)

    body := `{"to":"0x...", "amount":"0.1"}`
    req := httptest.NewRequest("POST", "/wallets/"+walletID+"/transfer", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    // Assert response
    assert(t, w.Code, http.StatusOK)
}
```

---

## Adding a New Chain

1. Add an entry to `chains.json` (or use `POST /api/chains` at runtime):

```json
{
  "name": "polygon",
  "label": "Polygon Mainnet",
  "protocol": "ecdsa",
  "curve": "secp256k1",
  "currency": "POL",
  "family": "evm",
  "rpc_url": "https://polygon-rpc.com",
  "chain_id": 137
}
```

2. If the chain needs a CoinGecko price feed, add the currency to `coinGeckoIDs` in `handler/price.go`.

3. If it's a CoinGecko-supported EVM platform for token pricing, add it to `coinGeckoPlatformIDs` in `handler/price.go`.

That's it. No code changes needed for standard EVM chains. Solana-family chains require changes to `chain/tx_sol.go`.

---

## Adding a New API Endpoint

1. **Handler** -- create a method on the appropriate handler struct in `handler/`. Follow the pattern: validate input → check auth → business logic → write audit → respond.

2. **Route** -- register it in `main.go` under the appropriate group (`pub` for public, `passkeyOnly` for Passkey-only, `auth` for dual-auth).

3. **Model** (if needed) -- add a GORM model in `model/`. AutoMigrate will create the table on next startup.

4. **Test** -- add a test in `handler/*_test.go` using in-memory SQLite.

5. **Docs** -- update the relevant page in `docs/en/` and `docs/zh/`, plus `docs/api/openapi.yaml`.

### Auth Groups in main.go

```go
pub := r.Group("/api")           // No auth required
passkeyOnly := auth.Group("")    // Passkey session required
auth := r.Group("/api")          // API key OR Passkey session
auth.Use(handler.AuthMiddleware) // Dual-auth middleware
```

---

## Database

SQLite with WAL mode. Tables are auto-migrated on startup:

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

---

## Signing Flow (Internal)

```
handler (wallet.go / contract_call.go)
  → build transaction (chain/tx_eth.go or chain/tx_sol.go)
  → check approval policy (threshold, daily limit)
  → if needs approval: create ApprovalRequest, return 202
  → if direct: sdk.Sign() → broadcast → return tx_hash

handler (approval.go) on approve:
  → verify fresh Passkey assertion
  → rebuild transaction (refresh nonce/gas)
  → sdk.Sign() → broadcast → update ApprovalRequest with tx_hash
```

The approval path rebuilds the transaction at approval time (not at request time) to ensure fresh nonce and gas estimates.

---
[Previous: API Overview](/en/api-overview.md) | [Next: Architecture & Security](/en/whitepaper.md)
