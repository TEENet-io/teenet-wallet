# TEENet Wallet -- Product Guide

## What is TEENet Wallet?

TEENet Wallet is a multi-chain cryptocurrency wallet where private keys never exist as a whole on any single machine. Instead of storing a private key in a file, a hardware security module, or a browser extension, TEENet Wallet splits every key across a cluster of Trusted Execution Environment (TEE) nodes using threshold cryptography. When a transaction needs to be signed, a quorum of TEE nodes (for example, 3 out of 5) cooperate to produce a valid signature without ever reconstructing the full private key. The result is a wallet that is as easy to use as a hosted API, but with security guarantees that exceed traditional custody solutions.

TEENet Wallet exposes a clean REST API for wallet creation, balance queries, token transfers, smart contract interaction, and approval management. It supports Ethereum, Solana, and any EVM-compatible chain. Two authentication modes serve different audiences: API keys (`ocw_` prefix) for AI agents and automated pipelines, and Passkey/WebAuthn sessions (`ps_` prefix) for human operators who need hardware-bound approval for high-value or sensitive operations.

The target audience includes AI agent platforms (such as OpenClaw) that need programmatic custody, DeFi automation systems that execute trades and rebalances without human intervention below configurable thresholds, and institutional teams that require multi-layer approval policies with USD-denominated spend limits -- all without ever exposing raw private key material.

---

## Key Features

**Multi-chain support.** Ethereum mainnet, Solana mainnet, and all EVM-compatible chains (Sepolia, Holesky, BSC, Optimism, Base Sepolia) are supported out of the box. Custom EVM chains can be added at runtime via API or by editing `chains.json`.

**Dual authentication model.** API keys enable headless automation for bots and agents. Passkey sessions provide hardware-bound human approval for sensitive operations. Both modes share the same REST API surface, and the wallet enforces which operations require which auth level.

**USD-denominated approval thresholds.** Each wallet can have a policy that triggers Passkey approval when a single transaction exceeds a USD threshold. Real-time ETH and SOL prices are fetched from CoinGecko; stablecoins are pegged at $1. A single policy covers all currencies on the wallet.

**Daily spend limits.** An optional daily USD cap hard-blocks transfers when exceeded -- no approval path, just a hard stop until UTC midnight. The wallet uses an auth/capture pattern that pre-deducts spend and rolls back on signing or broadcast failure, preventing phantom spend from failed transactions.

**Contract whitelist with 3-layer security.** (1) Only pre-approved contract addresses, token mints, or program IDs can be called. (2) Optional per-contract `allowed_methods` lists restrict which functions can be invoked. (3) High-risk methods (`approve`, `transferFrom`, `increaseAllowance`, `setApprovalForAll`, `safeTransferFrom`) always require Passkey approval, even when auto-approve is enabled.

**Smart contract interaction.** Full Solidity ABI encoding covers `address`, `uintN`, `intN`, `bytesN`, `bytes`, `string`, dynamic arrays, fixed-size arrays, and tuples. Solana program calls accept raw account metas and hex-encoded instruction data. Convenience endpoints handle `approve-token`, `revoke-approval`, and read-only calls.

**ERC-20 and SPL token transfers.** Token transfers are built, signed, and broadcast by the backend. For Solana, the recipient's Associated Token Account (ATA) is created automatically if it does not exist. Wrap/unwrap SOL endpoints convert between native SOL and wSOL.

**Idempotent transfers.** The `Idempotency-Key` header prevents duplicate transactions when retrying after network errors.

---

## Architecture

```
 AI Agent / App                        Human (Browser)
     |  API Key (ocw_*)                    |  Passkey Session (ps_*)
     v                                     v
+--------------------------------------------------------------+
|  TEENet Wallet  (:8080)                                      |
|  - Builds transactions (EIP-1559, Solana)                    |
|  - Enforces contract whitelist + method gates                |
|  - Manages USD approval policies + daily limits              |
|  - Routes to approval queue or direct signing                |
|  - Nonce manager for concurrent EVM safety                   |
+--------------------------------------------------------------+
     |  TEENet SDK (HTTP)
     v
+--------------------------------------------------------------+
|  app-comm-consensus  (:8089)                                 |
|  - M-of-N voting coordination                               |
|  - Caches application/instance/key config from UMS           |
+--------------------------------------------------------------+
     |  gRPC + mutual TLS
     v
+--------------------------------------------------------------+
|  TEE-DAO Key Management Cluster  (3-5 nodes)                 |
|  - Distributed Key Generation (FROST / GG20)                 |
|  - Threshold signing (e.g. 3-of-5)                           |
|  - Private key shares never leave TEE hardware               |
+--------------------------------------------------------------+
```

### Signing Flow

1. A client sends a request to TEENet Wallet, authenticated with an API key or Passkey session.
2. TEENet Wallet validates the request against the contract whitelist, method restrictions, and approval policies (USD threshold, daily limit).
3. If the request requires human approval (amount exceeds threshold, high-risk method, or auto-approve is not enabled), it enters a pending state. The human approves via Passkey in the web UI.
4. TEENet Wallet sends the transaction payload to the local `app-comm-consensus` node using the TEENet SDK.
5. `app-comm-consensus` coordinates M-of-N voting across the TEE-DAO cluster via gRPC with mutual TLS.
6. The TEE-DAO nodes perform threshold signing -- the private key is never reconstructed on any single machine.
7. TEENet Wallet receives the signature, broadcasts the signed transaction to the blockchain, and returns the transaction hash.

---

## Quick Start

### Prerequisites

- **Go 1.24+** (for building from source)
- **SQLite3 development headers** (`apt-get install libsqlite3-dev` on Debian/Ubuntu)
- A running **TEENet mesh node** with `app-comm-consensus` on port 8089

### Installation

Build from source:

```bash
git clone https://github.com/TEENet-io/teenet-wallet.git
cd teenet-wallet
make build
```

Or use Docker:

```bash
make docker
docker run -p 8080:8080 \
  -e CONSENSUS_URL=http://host.docker.internal:8089 \
  -v wallet-data:/data \
  teenet-wallet:latest
```

Start the server:

```bash
./teenet-wallet
```

The server listens on `http://0.0.0.0:8080` by default.

### Create Your First Wallet

**Step 1: Register with a Passkey.**

Open the web UI at `http://localhost:8080` in a browser that supports WebAuthn (Chrome, Safari, Firefox). Complete the Passkey registration flow. This creates your user account and a Passkey session.

**Step 2: Generate an API key.**

From the web UI, go to Settings and generate an API key. The key starts with `ocw_`. Save it securely -- it is shown only once.

Alternatively, if you already have a Passkey session token (`ps_`), you can use the API:

```bash
curl -s -X POST http://localhost:8080/api/auth/apikey/generate \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck" \
  -H "Content-Type: application/json" \
  -d '{"label": "my-agent-key"}'
```

**Step 3: Create a wallet.**

```bash
curl -s -X POST http://localhost:8080/api/wallets \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"chain": "sepolia", "label": "Test Wallet"}'
```

Response:

```json
{
  "success": true,
  "wallet": {
    "id": "8a2fbc16-faf4-451a-be34-9fc5c49cde00",
    "chain": "sepolia",
    "address": "0x1234abcd5678ef90...",
    "label": "Test Wallet",
    "status": "ready"
  }
}
```

Note: Ethereum (ECDSA) wallets may take 1-2 minutes to create due to distributed key generation. Solana wallets are created instantly.

### Send Your First Transaction

Fund the wallet address with testnet ETH from a Sepolia faucet, then send a transfer:

```bash
curl -s -X POST http://localhost:8080/api/wallets/8a2fbc16-faf4-451a-be34-9fc5c49cde00/transfer \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "0.01",
    "memo": "first test transfer"
  }'
```

Response (direct completion):

```json
{
  "success": true,
  "status": "completed",
  "tx_hash": "0xabc123...",
  "chain": "sepolia",
  "explorer_url": "https://sepolia.etherscan.io/tx/0xabc123..."
}
```

### Set an Approval Policy

Protect the wallet by requiring Passkey approval for transfers above $50 USD, with a $500 daily limit:

```bash
curl -s -X PUT http://localhost:8080/api/wallets/8a2fbc16-faf4-451a-be34-9fc5c49cde00/policy \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "threshold_usd": "50",
    "daily_limit_usd": "500",
    "enabled": true
  }'
```

When called with an API key, policy changes require Passkey approval. The response includes an `approval_id`:

```json
{
  "success": true,
  "pending": true,
  "approval_id": 1,
  "message": "Policy change submitted for approval"
}
```

Open the approval link in the web UI and confirm with your Passkey to activate the policy.

---

## Configuration Reference

All configuration is via environment variables. No configuration files are required for the wallet service itself (chain definitions live in `chains.json`).

| Variable | Default | Description |
|----------|---------|-------------|
| `CONSENSUS_URL` | `http://localhost:8089` | URL of the local `app-comm-consensus` node |
| `HOST` | `0.0.0.0` | Bind address for the HTTP server |
| `PORT` | `8080` | HTTP listen port |
| `DATA_DIR` | `/data` | Directory for the SQLite database file (`wallet.db`) |
| `BASE_URL` | `http://localhost:<PORT>` | Public-facing URL used in approval links and callbacks |
| `FRONTEND_URL` | _(empty)_ | Allowed CORS origin; empty disables CORS headers entirely |
| `CHAINS_FILE` | `./chains.json` | Path to the chain configuration file |
| `APP_INSTANCE_ID` | _(from TEENet)_ | TEENet application instance identifier; usually set automatically |
| `API_KEY_RATE_LIMIT` | `200` | Maximum requests per minute per API key |
| `WALLET_CREATE_RATE_LIMIT` | `5` | Maximum wallet creations per minute per key (TEE DKG is expensive) |
| `REGISTRATION_RATE_LIMIT` | `10` | Maximum registration attempts per minute per IP |
| `APPROVAL_EXPIRY_MINUTES` | `30` | Minutes before a pending approval request expires |
| `MAX_WALLETS_PER_USER` | `20` | Maximum wallets a single user can create |

RPC URLs for each blockchain are defined in `chains.json`, not as individual environment variables. Additional EVM chains can be added at runtime via `POST /api/chains` (Passkey required); these are persisted in the database and survive restarts.

---

## Authentication

TEENet Wallet uses a dual authentication model. Every protected endpoint accepts either type of credential in the `Authorization` header.

### API Keys

API keys are intended for AI agents, bots, and automated pipelines. They are prefixed with `ocw_`.

**Generating a key** (requires Passkey session):

```bash
curl -s -X POST http://localhost:8080/api/auth/apikey/generate \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck" \
  -H "Content-Type: application/json" \
  -d '{"label": "production-agent"}'
```

**Using a key:**

```bash
curl -s http://localhost:8080/api/wallets \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

**Listing keys:**

```bash
curl -s http://localhost:8080/api/auth/apikey/list \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck"
```

**Revoking a key:**

```bash
curl -s -X DELETE http://localhost:8080/api/auth/apikey \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck" \
  -H "Content-Type: application/json" \
  -d '{"key_id": "KEY_ID_HERE"}'
```

API keys can perform most operations directly. However, certain sensitive operations (wallet deletion, policy deletion, contract removal, approval/reject actions) require a Passkey session. When an API key attempts to set a policy or add a contract to the whitelist, the request creates a pending approval that the Passkey owner must confirm.

**Rate limiting:** Each API key is limited to a configurable number of requests per minute (default: 200). Wallet creation has a separate, lower limit (default: 5 per minute) because TEE distributed key generation is computationally expensive.

### Passkey Sessions

Passkey sessions use the WebAuthn standard for hardware-bound authentication. They are prefixed with `ps_`.

**Registration flow:**

1. `POST /api/auth/passkey/register/begin` -- begin open registration (returns a challenge)
2. Complete the WebAuthn ceremony in the browser
3. `POST /api/auth/passkey/register/verify` -- submit the attestation

**Login flow:**

1. `GET /api/auth/passkey/options` -- get a login challenge
2. Complete the WebAuthn assertion in the browser
3. `POST /api/auth/passkey/verify` -- submit the assertion and receive a session token

Passkey sessions are required for:
- Generating, listing, and revoking API keys
- Deleting wallets
- Deleting approval policies
- Removing contracts from the whitelist
- Approving or rejecting pending approval requests
- Deleting the user account
- Adding or removing custom chains

### CSRF Protection

All state-changing requests made with a Passkey session must include the `X-CSRF-Token` header. Any non-empty value is accepted (e.g., `nocheck`). This prevents cross-site request forgery attacks against browser-based sessions.

```bash
curl -s -X POST http://localhost:8080/api/wallets \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck" \
  -H "Content-Type: application/json" \
  -d '{"chain": "ethereum", "label": "My Wallet"}'
```

API key requests do not require the CSRF header.

---

## Wallet Management

### Create a Wallet

```bash
curl -s -X POST http://localhost:8080/api/wallets \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"chain": "ethereum", "label": "Main Wallet"}'
```

The `chain` field must match a name from `GET /api/chains` (e.g., `ethereum`, `solana`, `sepolia`, `solana-devnet`, or a custom chain name). Ethereum/EVM wallets use ECDSA on secp256k1 and may take 1-2 minutes for distributed key generation. Solana wallets use Schnorr on ed25519 and are created instantly.

Each user can create up to `MAX_WALLETS_PER_USER` wallets (default: 20).

### List Wallets

```bash
curl -s http://localhost:8080/api/wallets \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

Returns all wallets for the authenticated user, including `id`, `chain`, `address`, `label`, and `status` (`creating`, `ready`, or `error`).

### Get Wallet Details

```bash
curl -s http://localhost:8080/api/wallets/WALLET_ID \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

### Rename a Wallet

```bash
curl -s -X PATCH http://localhost:8080/api/wallets/WALLET_ID \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"label": "Updated Label"}'
```

### Delete a Wallet

Wallet deletion is irreversible and requires a Passkey session:

```bash
curl -s -X DELETE http://localhost:8080/api/wallets/WALLET_ID \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck"
```

### Chain Selection

Query the available chains before creating a wallet:

```bash
curl -s http://localhost:8080/api/chains
```

Response:

```json
{
  "success": true,
  "chains": [
    {"name": "ethereum", "label": "Ethereum Mainnet", "currency": "ETH", "family": "evm", "custom": false},
    {"name": "solana", "label": "Solana Mainnet", "currency": "SOL", "family": "solana", "custom": false},
    {"name": "sepolia", "label": "Sepolia Testnet", "currency": "ETH", "family": "evm", "custom": false}
  ]
}
```

Custom EVM chains can be added at runtime (Passkey required):

```bash
curl -s -X POST http://localhost:8080/api/chains \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "polygon",
    "label": "Polygon Mainnet",
    "currency": "MATIC",
    "rpc_url": "https://polygon-rpc.com",
    "chain_id": 137
  }'
```

Custom chains are persisted in the database and survive restarts. They can be removed with `DELETE /api/chains/:name` (fails if any wallet exists on that chain).

---

## Transfers

### Native Transfers (ETH / SOL)

Send native currency by calling the `/transfer` endpoint without a `token` field:

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/transfer \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "0.1",
    "memo": "payment for services"
  }'
```

For Solana wallets, use a base58 recipient address:

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/transfer \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "RecipientBase58Address...",
    "amount": "1.5"
  }'
```

The backend builds the transaction (EIP-1559 for EVM, native transfer instruction for Solana), signs it via the TEE cluster, and broadcasts it. The response includes the `tx_hash` on success.

### ERC-20 Token Transfers

Include the `token` field to send ERC-20 tokens. The token contract must be whitelisted first (see [Contract Whitelist](#contract-whitelist)).

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/transfer \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "100",
    "token": {
      "contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
      "symbol": "USDC",
      "decimals": 6
    }
  }'
```

The `amount` is in human-readable token units (e.g., `100` for 100 USDC). The backend converts to raw units using the `decimals` value.

**Important:** Omitting the `token` field sends native ETH instead. Always double-check that your request payload includes the `token` object when sending tokens.

### SPL Token Transfers (Solana)

SPL token transfers use the same `/transfer` endpoint with the `token` field. The token mint must be whitelisted.

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/transfer \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "RecipientBase58Address...",
    "amount": "50",
    "token": {
      "contract": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
      "symbol": "USDC",
      "decimals": 6
    }
  }'
```

If the recipient does not have an Associated Token Account (ATA) for the token mint, the backend creates it automatically in the same transaction.

### Wrap and Unwrap SOL

Convert native SOL to wSOL (Wrapped SOL SPL token):

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/wrap-sol \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"amount": "0.5"}'
```

Unwrap all wSOL back to native SOL (closes the wSOL ATA):

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/unwrap-sol \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{}'
```

### Idempotency

To prevent duplicate transactions when retrying after network errors, include the `Idempotency-Key` header:

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/transfer \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Idempotency-Key: unique-request-id-12345" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "0.5"
  }'
```

If a request with the same idempotency key has already been processed, the wallet returns the original response without executing the transfer again. Keys are scoped to the authenticated user.

---

## Smart Contract Interaction

### Contract Whitelist

Before calling any smart contract, the contract address (EVM), token mint (SPL), or program ID (Solana) must be added to the wallet's whitelist.

**List whitelisted contracts:**

```bash
curl -s http://localhost:8080/api/wallets/WALLET_ID/contracts \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

**Add a contract via API key** (creates a pending approval):

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/contracts \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contract_address": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
    "symbol": "USDC",
    "decimals": 6,
    "label": "USD Coin",
    "allowed_methods": "transfer,balanceOf",
    "auto_approve": false
  }'
```

A `202` response means the request needs Passkey approval:

```json
{
  "success": true,
  "pending": true,
  "approval_id": 7,
  "message": "Contract whitelist request submitted for approval"
}
```

**Add a contract via Passkey session** (applied immediately):

The same endpoint returns `201` when called with a Passkey session, and the contract is whitelisted immediately.

**Update a contract configuration:**

```bash
curl -s -X PUT http://localhost:8080/api/wallets/WALLET_ID/contracts/CONTRACT_ID \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"auto_approve": true, "allowed_methods": "transfer,approve,balanceOf"}'
```

**Remove a contract** (Passkey only):

```bash
curl -s -X DELETE http://localhost:8080/api/wallets/WALLET_ID/contracts/CONTRACT_ID \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck"
```

**Whitelist fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `contract_address` | Yes | EVM address (`0x...`) or Solana mint/program address (base58) |
| `symbol` | No | Token symbol (e.g., USDC) |
| `decimals` | No | Token decimals (6 for USDC, 18 for WETH, 9 for most SPL tokens) |
| `label` | No | Human-readable label |
| `allowed_methods` | No | Comma-separated method names (e.g., `transfer,approve`). Empty = all methods allowed |
| `auto_approve` | No | If `true`, API keys can execute non-high-risk calls without Passkey. Default: `false` |

### Contract Calls (EVM)

Call any whitelisted smart contract function using the `/contract-call` endpoint with `func_sig` and `args`:

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/contract-call \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xContractAddress...",
    "func_sig": "swap(address,uint256,uint256,address)",
    "args": [
      "0xTokenAddress...",
      "1000000",
      "990000",
      "0xRecipientAddress..."
    ],
    "value": "0.1",
    "amount_usd": "250.00",
    "memo": "DEX swap"
  }'
```

**Function signature format:** Use Solidity-style signatures such as `transfer(address,uint256)`, `approve(address,uint256)`, or `exactInputSingle((address,address,uint24,address,uint256,uint256,uint160))` for tuple parameters.

**Supported argument types:** `address`, `bool`, `uintN`, `intN`, `bytesN`, `bytes`, `string`, dynamic arrays, fixed-size arrays (`T[N]`), and tuples.

### Program Calls (Solana)

For Solana programs, use `accounts` and `data` instead of `func_sig`/`args`:

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/contract-call \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "ProgramIdBase58...",
    "accounts": [
      {"pubkey": "Account1Base58...", "is_signer": false, "is_writable": true},
      {"pubkey": "Account2Base58...", "is_signer": false, "is_writable": false}
    ],
    "data": "a1b2c3d4e5f6...",
    "amount_usd": "100.00",
    "memo": "program interaction"
  }'
```

The program ID must be whitelisted. The wallet's own address is added as a signer automatically if required. The `data` field contains hex-encoded instruction data (discriminator + encoded arguments).

### Read-Only Calls

Query contract state without signing or sending a transaction. No gas fees, no approval needed:

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/call-read \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
    "func_sig": "balanceOf(address)",
    "args": ["0xYourWalletAddress..."]
  }'
```

Response:

```json
{
  "success": true,
  "result": "0x0000000000000000000000000000000000000000000000000000000005f5e100",
  "contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
  "method": "balanceOf"
}
```

Useful for checking token balances (`balanceOf`), allowances (`allowance`), and reading contract state (`totalSupply`, `name`, `symbol`, `decimals`). This endpoint is EVM-only.

### The `amount_usd` Field for Threshold Enforcement

When a contract call involves transferring value (e.g., a DeFi swap, a deposit, a token transfer via contract), include the `amount_usd` field with the approximate USD value so the wallet can enforce threshold and daily-limit policies.

```json
{
  "contract": "0x...",
  "func_sig": "deposit(uint256)",
  "args": ["1000000000"],
  "amount_usd": "1000.00"
}
```

If both `value` (native ETH sent with the call) and `amount_usd` are present, the wallet uses whichever is larger. If neither is provided, threshold and daily-limit checks are skipped for that call.

You can check current prices via `GET /api/prices` to help compute the USD value.

### Convenience Endpoints

**Approve ERC-20 token spending** (always requires Passkey -- high-risk method):

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/approve-token \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xTokenAddress...",
    "spender": "0xSpenderAddress...",
    "amount": "1000",
    "decimals": 6
  }'
```

**Revoke ERC-20 token approval** (always requires Passkey -- high-risk method):

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/revoke-approval \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xTokenAddress...",
    "spender": "0xSpenderAddress..."
  }'
```

---

## Approval System

### USD Thresholds

Each wallet can have a single USD-denominated approval policy. When a transfer or contract call exceeds the threshold, it enters a pending state requiring Passkey approval.

```bash
curl -s -X PUT http://localhost:8080/api/wallets/WALLET_ID/policy \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "threshold_usd": "100",
    "daily_limit_usd": "5000",
    "enabled": true
  }'
```

- `threshold_usd`: Transactions above this USD value require Passkey approval.
- `daily_limit_usd` (optional): Cumulative USD spend per UTC calendar day. Transfers that would exceed this limit are hard-blocked (no approval path).
- The policy covers all currencies on the wallet (ETH, SOL, tokens). Real-time prices from CoinGecko are used for conversion; stablecoins are pegged at $1.

**View the current policy:**

```bash
curl -s http://localhost:8080/api/wallets/WALLET_ID/policy \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

**Delete a policy** (Passkey only):

```bash
curl -s -X DELETE http://localhost:8080/api/wallets/WALLET_ID/policy \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck"
```

### Daily Limits

Daily limits use a pre-deduction (auth/capture) pattern:

1. Before signing, the wallet pre-deducts the USD amount from the daily budget.
2. If signing or broadcast fails, the pre-deduction is rolled back.
3. This prevents phantom spend from failed transactions while ensuring the limit is enforced even under concurrent requests.

The daily limit resets at UTC midnight.

### Approval Flow

When a request triggers an approval:

1. The API returns `"status": "pending_approval"` with an `approval_id`.
2. The human owner opens the approval link in the web UI.
3. The web UI prompts for a fresh Passkey assertion (hardware-bound -- a stolen session token cannot approve).
4. After approval, the wallet completes the transaction (signs and broadcasts).

**List pending approvals:**

```bash
curl -s http://localhost:8080/api/approvals/pending \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

**Get approval details:**

```bash
curl -s http://localhost:8080/api/approvals/APPROVAL_ID \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

**Approve a request** (Passkey only):

```bash
curl -s -X POST http://localhost:8080/api/approvals/APPROVAL_ID/approve \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck"
```

**Reject a request** (Passkey only):

```bash
curl -s -X POST http://localhost:8080/api/approvals/APPROVAL_ID/reject \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck"
```

Pending approvals expire after the configured timeout (default: 30 minutes).

**Approval types:**

| Type | Trigger |
|------|---------|
| `transfer` | Transfer exceeds USD threshold |
| `sign` | Sign request exceeds threshold |
| `contract_call` | Contract call exceeds threshold or is a high-risk method |
| `policy_change` | Policy set via API key |
| `contract_add` | Contract whitelist addition via API key |
| `contract_update` | Contract whitelist update via API key |
| `wrap_sol` | Wrap SOL exceeds threshold |
| `unwrap_sol` | Unwrap SOL exceeds threshold |

### High-Risk Methods

The following EVM methods always require Passkey approval, regardless of auto-approve settings or threshold policies:

- `approve`
- `transferFrom`
- `increaseAllowance`
- `setApprovalForAll`
- `safeTransferFrom`

These methods grant third parties access to wallet funds and are therefore treated as unconditionally sensitive.

---

## Web UI

TEENet Wallet includes a built-in web interface served at the root URL (e.g., `http://localhost:8080`). The web UI provides:

- **Account management:** Passkey registration, login, and session management.
- **Wallet dashboard:** Create, view, rename, and delete wallets. View addresses, balances, and chain information.
- **Transfer interface:** Send native currency and tokens with a visual form.
- **Contract whitelist management:** Add, update, and remove whitelisted contracts with an interactive table.
- **Approval queue:** Review pending approval requests and approve or reject them with hardware Passkey authentication.
- **API key management:** Generate and revoke API keys for programmatic access.
- **Policy configuration:** Set and manage USD-denominated approval thresholds and daily limits.

The web UI is served with a restrictive Content Security Policy and additional security headers (`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`).

---

## AI Agent Integration

TEENet Wallet is designed to serve as the custody layer for AI agents. The `skill/tee-wallet/` directory contains an [OpenClaw](https://openclaw.io) skill definition that enables AI agents to manage wallets through natural language.

### How It Works

1. The agent platform (e.g., OpenClaw) provides two environment variables: `TEE_WALLET_API_URL` (the wallet service URL) and `TEE_WALLET_API_KEY` (an `ocw_` API key).
2. The AI agent reads the skill definition, which describes all available operations with curl examples.
3. When a user asks the agent to "send 0.1 ETH to 0x...", the agent calls the `/transfer` endpoint.
4. If the transfer is below the policy threshold, it completes immediately. If above, the agent shows the user an approval link and polls until approved.

### Best Practices for Agent Integration

**Automatic wallet selection.** Never ask the user for a wallet ID. Call `GET /api/wallets`, find wallets matching the required chain, and select automatically if there is only one match. If there are multiple matches, present a numbered list.

**No chat confirmation for transfers.** The backend approval policy is the safety net. Small amounts go through directly; large amounts trigger hardware Passkey approval automatically. Do not add an extra "are you sure?" step in the agent.

**Always include the `token` field for token transfers.** Omitting it sends native ETH/SOL instead, which is an irreversible mistake.

**Check the whitelist before token operations.** Call `GET /api/wallets/:id/contracts` before sending tokens. If the contract is not whitelisted, propose adding it (which creates a pending approval).

**Poll approvals with countdown.** When waiting for Passkey approval, poll `GET /api/approvals/:id` every 15 seconds and show the remaining time. Stop after 25 minutes.

**Use `amount_usd` for contract calls.** When calling `/contract-call` for operations that transfer value, always include the approximate USD value so threshold and daily-limit policies are enforced.

**Fetch fresh wallet lists.** Before showing balances or account-wide views, always re-fetch `GET /api/wallets` to ensure the list is current.

**Include explorer links.** After every successful transaction, provide a block explorer link so the user can verify on-chain.

---

## Supported Chains

The following chains are supported out of the box via `chains.json`:

| Chain | Name (API) | Currency | Protocol | Curve | Family |
|-------|------------|----------|----------|-------|--------|
| Ethereum Mainnet | `ethereum` | ETH | ECDSA | secp256k1 | EVM |
| Optimism Mainnet | `optimism` | ETH | ECDSA | secp256k1 | EVM |
| Sepolia Testnet | `sepolia` | ETH | ECDSA | secp256k1 | EVM |
| Holesky Testnet | `holesky` | ETH | ECDSA | secp256k1 | EVM |
| Base Sepolia Testnet | `base-sepolia` | ETH | ECDSA | secp256k1 | EVM |
| BSC Testnet | `bsc-testnet` | tBNB | ECDSA | secp256k1 | EVM |
| Solana Mainnet | `solana` | SOL | Schnorr | ed25519 | Solana |
| Solana Devnet | `solana-devnet` | SOL | Schnorr | ed25519 | Solana |

Any EVM-compatible chain can be added at runtime via `POST /api/chains` by providing a name, RPC URL, currency, and optional chain ID. Custom chains use ECDSA on secp256k1.

**Common ERC-20 token addresses:**

Ethereum Mainnet:

| Token | Contract | Decimals |
|-------|----------|----------|
| USDC | `0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48` | 6 |
| USDT | `0xdac17f958d2ee523a2206206994597c13d831ec7` | 6 |
| WETH | `0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2` | 18 |
| DAI | `0x6b175474e89094c44da98b954eedeac495271d0f` | 18 |

Sepolia Testnet:

| Token | Contract | Decimals |
|-------|----------|----------|
| USDC | `0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238` | 6 |
| WETH | `0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14` | 18 |
| LINK | `0x779877A7B0D9E8603169DdbD7836e478b4624789` | 18 |

Base Sepolia Testnet:

| Token | Contract | Decimals |
|-------|----------|----------|
| USDC | `0x036CbD53842c5426634e7929541eC2318f3dCF7e` | 6 |
| WETH | `0x4200000000000000000000000000000000000006` | 18 |

---

## Error Reference

### Common Errors and Solutions

| Error Message | Cause | Solution |
|---------------|-------|----------|
| `insufficient funds` | Wallet balance too low for the transfer plus gas | Check balance with `GET /api/wallets/:id/balance`. For ETH transfers, allow approximately 0.0005 ETH for gas. |
| `daily spend limit exceeded` | Cumulative USD spend for the day has reached the daily limit | Wait until UTC midnight for the limit to reset, or adjust the policy via Passkey. |
| `contract not whitelisted` | The contract address, token mint, or program ID is not in the wallet's whitelist | Add it via `POST /api/wallets/:id/contracts` (API key creates a pending approval) or through the web UI for instant approval. |
| `method not allowed` | The contract's `allowed_methods` list does not include the requested function | Update the contract's `allowed_methods` via `PUT /api/wallets/:id/contracts/:cid`. |
| `wallet is not ready` | The wallet is still in the `creating` state (DKG in progress) | Wait 1-2 minutes for ECDSA key generation to complete, then retry. |
| `invalid API key` | The provided API key is not valid or has been revoked | Verify the `Authorization` header value. Generate a new key if needed. |
| `approval has expired` | The pending approval was not acted on within the expiry window (default: 30 minutes) | Initiate the operation again to create a new approval request. |
| `cannot overwrite a built-in chain` | Attempted to create a custom chain with the same name as a built-in chain | Choose a different name for the custom chain. |
| `chain has existing wallets; delete them first` | Attempted to delete a custom chain that still has wallets | Delete all wallets on the chain before removing it. |
| `rate limit exceeded` | Too many requests in the current time window | Wait and retry. Default limits: 200 requests/min per API key, 5 wallet creations/min, 10 registrations/min per IP. |
| `CSRF token required` | A Passkey session request is missing the `X-CSRF-Token` header | Add `X-CSRF-Token: nocheck` (or any non-empty value) to state-changing requests. |
| `passkey session required` | The operation requires Passkey auth but was called with an API key | Use a Passkey session for this operation (wallet deletion, policy deletion, contract removal, approve/reject). |
| `max wallets reached` | User has reached the `MAX_WALLETS_PER_USER` limit | Delete unused wallets or increase the limit in the server configuration. |

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| `200` | Success |
| `201` | Resource created |
| `202` | Request accepted; pending Passkey approval |
| `400` | Invalid request (missing fields, bad format) |
| `401` | Authentication required or invalid credentials |
| `403` | Forbidden (e.g., attempting to delete a built-in chain) |
| `404` | Resource not found |
| `409` | Conflict (e.g., duplicate chain name, wallets exist on chain) |
| `429` | Rate limit exceeded |
| `500` | Internal server error |

---

## Audit Log

All wallet operations are recorded in an audit log. Query it with:

```bash
curl -s "http://localhost:8080/api/audit/logs?page=1&limit=20" \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

**Query parameters:**

| Parameter | Default | Description |
|-----------|---------|-------------|
| `page` | `1` | Page number |
| `limit` | `20` | Results per page (max: 100) |
| `action` | _(all)_ | Filter by action type |
| `wallet_id` | _(all)_ | Filter by wallet |

**Action types:**

| Action | Description |
|--------|-------------|
| `login` | Passkey login |
| `wallet_create` | Wallet created |
| `wallet_delete` | Wallet deleted |
| `transfer` | Transfer sent or pending |
| `sign` | Message signed or pending |
| `policy_update` | Approval policy set or pending |
| `approval_approve` | Approval request approved |
| `approval_reject` | Approval request rejected |
| `contract_add` | Contract added to whitelist or pending |
| `wrap_sol` | SOL wrapped into wSOL |
| `unwrap_sol` | wSOL unwrapped to SOL |
| `apikey_generate` | API key generated |
| `apikey_revoke` | API key revoked |

---

## Security Summary

| Layer | Mechanism |
|-------|-----------|
| Key storage | Private keys split across 3-5 TEE nodes via threshold cryptography (FROST/GG20). No single node holds the full key. |
| Signing | M-of-N threshold signing. The private key is never reconstructed. |
| Human approval | High-value transactions and sensitive operations require fresh WebAuthn/Passkey hardware assertion. |
| Contract security | 3-layer gate: whitelist, method restrictions, high-risk method force-approval. |
| Spend control | USD-denominated thresholds and daily limits with auth/capture pattern. |
| API protection | Per-key rate limiting, CSRF protection for browser sessions, invite-based registration. |
| Transport | Mutual TLS between wallet service and TEE-DAO cluster. |
| Data | SQLite with WAL mode, structured audit logging, Content Security Policy headers. |
