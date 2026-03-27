# Developer Guide

This guide is for developers who want to build, test, and contribute to the teenet-wallet codebase.

---

## Build & Run

**Prerequisites:** Go 1.24+, SQLite3 dev headers, a running `app-comm-consensus` node on port 8089.

```bash
# Build
make build

# Run (defaults: port 8080, data in /data, consensus at localhost:8089)
./teenet-wallet

# Or with Docker
make docker
docker run -p 8080:8080 \
  -e CONSENSUS_URL=http://host.docker.internal:8089 \
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
│   ├── wallet.go        # Wallet CRUD, transfer, sign, wrap/unwrap SOL, policy, daily-spent
│   ├── contract.go      # Contract whitelist add/update/delete
│   ├── contract_call.go # General contract calls (EVM + Solana), approve-token, revoke-approval
│   ├── call_read.go     # Read-only contract queries (eth_call)
│   ├── approval.go      # Approval list/detail/approve/reject, post-approval execution
│   ├── balance.go       # On-chain balance queries (native + ERC-20 + SPL)
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
│       └── SKILL.md     # OpenClaw skill definition
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
- **TEENet SDK** -- signing goes through `github.com/TEENet-io/teenet-sdk/go`, which talks to the local `app-comm-consensus` node.

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

Tests use in-memory SQLite (`file::memory:`) and don't require a running consensus node -- the SDK client is nil in tests, and signing calls are expected to fail (tests verify behavior up to the signing step).

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
