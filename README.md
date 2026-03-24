# TEENet Wallet

A multi-chain crypto wallet where private keys never leave TEE hardware -- split across distributed TEE-DAO nodes via threshold cryptography.

> **Disclaimer:** This software manages real cryptocurrency assets. Use at your own risk. The authors are not responsible for any loss of funds. Always test thoroughly on testnets before using with real assets.

## Features

### Wallet and Blockchain

- **Multi-chain support** -- Ethereum, Solana, and all EVM-compatible chains (Sepolia, Holesky, BSC, Optimism, Base Sepolia, and any custom chain via `chains.json` or the runtime API)
- **ERC-20 token transfers** with contract whitelist security
- **SPL token transfers** with automatic ATA (Associated Token Account) creation for recipients
- **General smart contract interaction** via `contract-call` endpoint -- full ABI encoding on EVM, generic program calls on Solana
- **Wrap/Unwrap SOL** -- convert between native SOL and wSOL (Wrapped SOL) SPL token
- **Convenience endpoints** -- `approve-token`, `revoke-approval`, `call-read` for common DeFi operations
- **EIP-1559 transactions** with dynamic fee estimation
- **Solana transaction building** -- native transfers, SPL TransferChecked, arbitrary program instructions
- **Nonce manager** for concurrent EVM transaction safety
- **Idempotent transfers** via `Idempotency-Key` header to prevent duplicates
- **Native and token balance queries**
- **Custom chain management** -- add or remove EVM chains at runtime via API (persisted in DB across restarts)

### Smart Contract Security (3-Layer Model)

1. **Contract whitelist** -- only pre-approved contract addresses (EVM), token mints (SPL), or program IDs (Solana) can be called
2. **Per-contract method restrictions** -- optional `allowed_methods` list limits which functions can be invoked (EVM)
3. **High-risk method force-approval** -- `approve`, `transferFrom`, `increaseAllowance`, `setApprovalForAll`, and `safeTransferFrom` always require Passkey approval, even with auto-approve enabled (EVM)
4. **Auto-approve mode** -- whitelisted contracts/programs with `auto_approve: true` allow API keys to execute without Passkey confirmation

### ABI Encoder

- Full Solidity ABI encoding: `address`, `bool`, `uintN`, `intN`, `bytesN`, `bytes`, `string`, dynamic arrays, fixed-size arrays (`T[N]`), and tuples
- Supports complex DeFi calls including Uniswap V3 `exactInputSingle` with tuple parameters and fixed-size array arguments

### Authentication and Authorization

- **Dual auth model:**
  - **API Keys** (`ocw_` prefix) -- for AI agent and programmatic automation
  - **Passkey sessions** (`ps_` prefix) -- for sensitive operations requiring human presence
- **Passkey (WebAuthn)** hardware approval for high-value transactions and destructive operations
- **Auto-approve mode** -- trusted contracts can be flagged so API keys execute without Passkey, except for high-risk methods
- **USD-denominated approval thresholds** with configurable daily spend limits (ETH/SOL prices via CoinGecko, stablecoins pegged to $1)
- **CSRF protection** via `X-CSRF-Token` header for Passkey sessions
- **Rate limiting** -- per-API-key for general requests, per-IP for registration endpoints
- **Invite-based and open registration** flows

### Infrastructure

- **Structured logging** (slog, JSON format)
- **Audit logging** for all wallet operations
- **Graceful shutdown** with configurable timeout
- **SQLite with WAL mode** for concurrent read performance
- **Built-in web UI** for account management and transaction approval
- **OpenClaw skill integration** for AI agent interaction
- **Docker deployment** -- runs as non-root user
- **Content Security Policy** headers for the web UI

## How It Works

```
AI Agent / App                       User (Browser)
    |  (API Key: ocw_*)                  |  (Passkey Session: ps_*)
    v                                    v
+--------------------------------------------------+
|  TEENet Wallet  (:8080)                           |
|  - Builds transactions                            |
|  - Enforces contract whitelist + method gates     |
|  - Manages approval policies + daily limits       |
|  - Routes to approval queue or direct signing     |
+--------------------------------------------------+
    |  (TEENet SDK - HTTP)
    v
+--------------------------------------------------+
|  app-comm-consensus  (:8089)                      |
|  - M-of-N voting coordination                    |
+--------------------------------------------------+
    |  (gRPC + mTLS)
    v
+--------------------------------------------------+
|  TEE-DAO Key Management Cluster                   |
|  - 3-of-5 threshold signing (FROST / GG20)        |
|  - Private key shares never leave TEE hardware    |
+--------------------------------------------------+
```

1. An application or AI agent sends a request to TEENet Wallet, authenticated with an API key or Passkey session.
2. TEENet Wallet validates the request against the contract whitelist, method restrictions, and approval policies.
3. If the amount exceeds the threshold, a high-risk method is called, or auto-approve is not enabled, the transaction enters a pending state requiring Passkey approval through the web UI.
4. TEENet Wallet requests a threshold signature from the local app-comm-consensus node via the TEENet SDK.
5. The TEE-DAO cluster performs distributed signing -- the private key is never reconstructed on any single machine.
6. TEENet Wallet broadcasts the signed transaction to the blockchain.

## Quick Start

### Prerequisites

- Go 1.24+
- SQLite3 development headers (`apt-get install libsqlite3-dev` on Debian/Ubuntu)
- A running TEENet mesh node with app-comm-consensus on port 8089

### Build and Run

```bash
make build
./teenet-wallet
```

The server starts on `http://0.0.0.0:8080` by default.

### Docker

```bash
make docker
docker run -p 8080:8080 \
  -e CONSENSUS_URL=http://host.docker.internal:8089 \
  -v wallet-data:/data \
  teenet-wallet:latest
```

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `CONSENSUS_URL` | `http://localhost:8089` | URL of the local app-comm-consensus node |
| `HOST` | `0.0.0.0` | Bind address |
| `PORT` | `8080` | HTTP listen port |
| `DATA_DIR` | `/data` | Directory for SQLite database |
| `BASE_URL` | `http://localhost:<PORT>` | Public-facing URL (used in approval links) |
| `FRONTEND_URL` | _(empty)_ | Allowed CORS origin; empty disables CORS headers |
| `CHAINS_FILE` | `./chains.json` | Path to chain configuration file |
| `APP_INSTANCE_ID` | _(from TEENet)_ | TEENet application instance identifier |
| `API_KEY_RATE_LIMIT` | `200` | Max requests per minute per API key |
| `WALLET_CREATE_RATE_LIMIT` | `5` | Max wallet creations per minute per key |
| `REGISTRATION_RATE_LIMIT` | `10` | Max registration attempts per minute per IP |
| `APPROVAL_EXPIRY_MINUTES` | `30` | Minutes before a pending approval expires |
| `MAX_WALLETS_PER_USER` | `20` | Maximum wallets a single user can create |

RPC URLs for each blockchain are defined in `chains.json`, not as individual environment variables. Override the file path with `CHAINS_FILE` if needed. Additional EVM chains can also be added at runtime via the `POST /api/chains` endpoint (Passkey required); these are persisted in the database and survive restarts.

## API Reference

**Auth legend:** "Public" = no authentication. "Dual" = API key or Passkey session. "Passkey" = Passkey session only.

### Public

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/health` | Health check (returns DB status) |
| GET | `/api/chains` | List supported chains (built-in + custom) |
| GET | `/api/prices` | Current USD prices for ETH, SOL, stablecoins |

### Authentication

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/auth/check-name` | Public | Check if a display name is available |
| GET | `/api/auth/passkey/options` | Public | Get WebAuthn login challenge |
| POST | `/api/auth/passkey/verify` | Public | Verify WebAuthn assertion and create session |
| POST | `/api/auth/passkey/register/begin` | Public | Begin open registration (rate-limited) |
| GET | `/api/auth/passkey/register/options` | Public | Get registration options (invite-token flow) |
| POST | `/api/auth/passkey/register/verify` | Public | Complete passkey registration (rate-limited) |
| POST | `/api/auth/invite` | Passkey | Generate an invite link |
| DELETE | `/api/auth/session` | Passkey | Logout (revoke current session) |
| DELETE | `/api/auth/account` | Passkey | Delete account and all keys |
| POST | `/api/auth/apikey/generate` | Passkey | Generate a new API key |
| GET | `/api/auth/apikey/list` | Passkey | List API key metadata |
| DELETE | `/api/auth/apikey` | Passkey | Revoke an API key |

### Custom Chains

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/chains` | Passkey | Add a custom EVM chain (persisted across restarts) |
| DELETE | `/api/chains/:name` | Passkey | Remove a custom chain (fails if wallets exist on it) |

### Wallets

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/wallets` | Dual | Create a new wallet (rate-limited) |
| GET | `/api/wallets` | Dual | List all wallets |
| GET | `/api/wallets/:id` | Dual | Get wallet details |
| PATCH | `/api/wallets/:id` | Dual | Rename a wallet (update label) |
| DELETE | `/api/wallets/:id` | Passkey | Delete a wallet (irreversible) |
| POST | `/api/wallets/:id/sign` | Dual | Sign an arbitrary message |
| POST | `/api/wallets/:id/transfer` | Dual | Build, sign, and broadcast a transfer (native or token) |
| POST | `/api/wallets/:id/wrap-sol` | Dual | Wrap native SOL into wSOL (Solana only) |
| POST | `/api/wallets/:id/unwrap-sol` | Dual | Unwrap all wSOL back to native SOL (Solana only) |
| GET | `/api/wallets/:id/balance` | Dual | Get native token balance |
| GET | `/api/wallets/:id/pubkey` | Dual | Get wallet public key |

### Contract Whitelist

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/wallets/:id/contracts` | Dual | List whitelisted contracts |
| POST | `/api/wallets/:id/contracts` | Dual | Add contract (API key creates pending approval) |
| PUT | `/api/wallets/:id/contracts/:cid` | Dual | Update contract config (API key creates pending approval) |
| DELETE | `/api/wallets/:id/contracts/:cid` | Passkey | Remove a whitelisted contract |

### Contract Calls

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/wallets/:id/contract-call` | Dual | Execute a contract call (EVM: ABI-encoded; Solana: program instruction). Optional `amount_usd` field for threshold/daily-limit enforcement |
| POST | `/api/wallets/:id/approve-token` | Dual | Approve ERC-20 token spending (always high-risk) |
| POST | `/api/wallets/:id/revoke-approval` | Dual | Revoke ERC-20 token approval (always high-risk) |
| POST | `/api/wallets/:id/call-read` | Dual | Read-only contract call (no signing, EVM only) |

### Approval Policies

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/wallets/:id/policy` | Dual | Get USD approval policy (one per wallet) |
| PUT | `/api/wallets/:id/policy` | Dual | Set USD policy: `threshold_usd`, `daily_limit_usd` (API key creates pending approval) |
| DELETE | `/api/wallets/:id/policy` | Passkey | Remove approval policy |

### Approvals

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/approvals/pending` | Dual | List pending approvals |
| GET | `/api/approvals/:id` | Dual | Get approval details |
| POST | `/api/approvals/:id/approve` | Passkey | Approve a pending request |
| POST | `/api/approvals/:id/reject` | Passkey | Reject a pending request |

### Audit

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/audit/logs` | Dual | Query operation audit log |

## Security Model

- **Distributed Key Generation (DKG):** Private keys are generated across a cluster of TEE nodes using threshold cryptography (FROST for Ed25519/Schnorr, GG20 for ECDSA). No single node ever holds the full private key.
- **Threshold Signing:** Transaction signing requires cooperation of M-of-N TEE nodes.
- **Passkey Hardware Approval:** High-value transfers, high-risk contract methods, and destructive operations (key deletion, policy changes) require fresh WebAuthn hardware authentication.
- **Contract Whitelist with Method Gates:** Contract interactions are restricted to pre-approved addresses. Per-contract `allowed_methods` lists restrict callable functions. High-risk methods (`approve`, `transferFrom`, `increaseAllowance`, `setApprovalForAll`, `safeTransferFrom`) always require Passkey approval regardless of auto-approve settings.
- **Auto-Approve Mode:** Trusted contracts can be flagged with `auto_approve: true`, allowing API keys to execute non-high-risk methods without Passkey confirmation.
- **CSRF Protection:** All state-changing API requests from Passkey sessions require a `X-CSRF-Token` header.
- **Rate Limiting:** Per-API-key and per-IP rate limits protect against abuse and prevent TEE DKG resource exhaustion.
- **Daily Spend Limits:** Optional USD-denominated daily limits that hard-block transfers when exceeded. Uses pre-deduction with rollback on signing/broadcast failure (auth/capture pattern) to prevent phantom spend from failed transactions.
- **Content Security Policy:** The web UI is served with a restrictive CSP header. Additional security headers include `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, and `Referrer-Policy: strict-origin-when-cross-origin`.

## Supported Chains

| Chain | Currency | Protocol | Curve | Family |
|-------|----------|----------|-------|--------|
| Ethereum Mainnet | ETH | ECDSA | secp256k1 | EVM |
| Optimism Mainnet | ETH | ECDSA | secp256k1 | EVM |
| Sepolia Testnet | ETH | ECDSA | secp256k1 | EVM |
| Holesky Testnet | ETH | ECDSA | secp256k1 | EVM |
| Base Sepolia Testnet | ETH | ECDSA | secp256k1 | EVM |
| BSC Testnet | tBNB | ECDSA | secp256k1 | EVM |
| Solana Mainnet | SOL | Schnorr | ed25519 | Solana |
| Solana Devnet | SOL | Schnorr | ed25519 | Solana |

Add or modify chains by editing `chains.json`, or add custom EVM chains at runtime via `POST /api/chains` (persisted in the database). Any EVM-compatible chain can be added by providing a name, RPC URL, and currency.

## Development

```bash
make build         # compile binary
make test          # run tests with race detector
make lint          # go vet + staticcheck
make docker        # build Docker image
make clean         # remove binary
```

### Project Structure

```
main.go                  Entry point, router setup, middleware
handler/
  auth.go                Passkey registration, login, API key management
  wallet.go              Wallet CRUD, sign, transfer, policy management
  contract.go            Contract whitelist CRUD
  contract_call.go       General contract calls, approve-token, revoke-approval
  call_read.go           Read-only contract calls
  approval.go            Approval queue (list, approve, reject)
  balance.go             Native token balance queries
  audit.go               Audit log queries
  middleware.go          Auth, CSRF, Passkey-only middleware
  ratelimit.go           Per-key and per-IP rate limiters
  idempotency.go         Idempotent transfer deduplication
  price.go               USD price service (CoinGecko + cache)
  helpers.go             Shared handler utilities
  response.go            Standardized JSON response helpers
model/
  user.go                User and API key models
  wallet.go              Wallet, CustomChain models and chain config loader
  contract.go            AllowedContract model (whitelist + method restrictions)
  policy.go              ApprovalPolicy and ApprovalRequest models
  audit.go               AuditLog model
  idempotency.go         IdempotencyRecord model
chain/
  abi.go                 Full Solidity ABI encoder (address, uintN, bytes, tuples, fixed arrays)
  tx_eth.go              EVM transaction building (EIP-1559)
  tx_sol.go              Solana transaction building (native, SPL, program call, wrap/unwrap)
  nonce.go               Nonce manager for concurrent EVM transactions
  rpc.go                 JSON-RPC client with retry
  address.go             Address validation, derivation, PDA and ATA computation
frontend/                Pre-built web UI assets
docs/                    Design documents
skill/tee-wallet/        OpenClaw AI agent skill definition
chains.json              Supported chain configuration
Dockerfile               Multi-stage Docker build (non-root runtime)
```

## OpenClaw Skill

The `skill/tee-wallet/` directory contains an [OpenClaw](https://openclaw.io) skill definition that enables AI agents to manage wallets, check balances, and send transactions through natural language. See `skill/tee-wallet/SKILL.md` for the full specification.

## Related Projects

| Project | Description |
|---------|-------------|
| [TEENet SDK (Go/TypeScript)](https://github.com/TEENet-io/teenet-sdk) | Client SDK for TEE signing and key management — used by this wallet to interact with TEE-DAO nodes |
| [OpenClaw](https://openclaw.io) | AI assistant platform — this wallet integrates as a skill |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## License

MIT -- see [LICENSE](LICENSE)
