# Architecture Overview

This page provides a mental model of how TEENet Wallet is structured, what the key terms mean, and how the major components work together.

---

## Key Terms

| Term | Definition |
|------|-----------|
| **Wallet** | A chain-specific account backed by a TEE-managed key. Each wallet has one address, one key pair, and belongs to one user. |
| **Chain** | A blockchain network (e.g. `ethereum`, `solana`, `avalanche-c`). Configured in `chains.json` or added at runtime. |
| **Approval policy** | Rules that control when a transaction requires Passkey approval. An approval policy includes a USD threshold (token transfers above it require approval), an optional daily spending limit (resets at midnight UTC), and a contract whitelist (only listed contract addresses are allowed for contract calls). |
| **API key** | A bearer token prefixed `ocw_` used by agents and automation. Can perform most operations, but sensitive actions (transfers above threshold, contract whitelist changes) create pending approvals that require Passkey confirmation. |
| **TEENet service** | The platform that the wallet is built on. Provides key generation, threshold signing, and Passkey management through an SDK. The wallet interacts with it exclusively through the SDK — never directly with TEE nodes. For local development, a mock service (`teenet-sdk/mock-server`) simulates the same SDK interface with real cryptographic operations but deterministic keys. |
| **TEE node** | A server running inside a Trusted Execution Environment (e.g. Intel SGX, AMD SEV). TEE nodes hold key shares and participate in threshold signing. |

---

## System Components

TEENet Wallet is a single Go binary with a clear internal layering:

```
┌─────────────────────────────────────────────────────┐
│  main.go                                            │
│  Route registration, middleware wiring, DI setup    │
└──────────────┬──────────────────────────────────────┘
               │
┌──────────────▼──────────────────────────────────────┐
│  handler/                                           │
│  HTTP handlers — one file per domain                │
│  (wallet, approval, auth, contract, balance, ...)   │
└──────────────┬──────────────────────────────────────┘
               │
       ┌───────┴────────┐
       │                │
┌──────▼──────┐  ┌──────▼──────────────────────┐
│  chain/     │  │  TEENet SDK                  │
│  Tx build,  │  │  (github.com/TEENet-io/      │
│  RPC calls, │  │   teenet-sdk/go)             │
│  address    │  │  Key gen, signing, passkeys  │
│  derivation │  └──────────┬──────────────────┘
└─────────────┘             │
                    ┌───────▼───────┐
                    │  TEENet       │
                    │  Service      │
                    │  (or mock)    │
                    └───────────────┘
```

**Storage:** SQLite with WAL mode, managed through GORM. All tables are auto-migrated on startup.

**Frontend:** A single-file SPA (`frontend/index.html`) served by the same Go binary. Used for Passkey registration, login, and approval confirmation.

---

## How Signing Works

```
 AI Agent / App                        Human (Browser)
     |  API Key (ocw_*)                    |  Passkey
     v                                     v
+--------------------------------------------------------------+
|  TEENet Wallet  (:8080)                                      |
|  REST API · approval policies · contract whitelist            |
+--------------------------------------------------------------+
     |  TEENet SDK
     v
+--------------------------------------------------------------+
|  TEENet Service                                              |
|  Threshold signing · keys never leave TEE hardware            |
+--------------------------------------------------------------+
```

1. Client sends a request (API key or Passkey).
2. Wallet checks whitelist, threshold, and daily limit.
3. If approval is needed, the request enters a pending state until the owner confirms with Passkey.
4. Wallet routes the signing request through the TEE cluster.
5. TEE nodes produce a threshold signature — the full key is never reconstructed.
6. Wallet broadcasts the signed transaction and returns the hash.

---

## Core Workflows

### Create a wallet

1. User sends `POST /api/wallets` with a chain name.
2. Handler creates a `"creating"` wallet record in the database.
3. SDK calls `GenerateKey` with the appropriate scheme and curve (ECDSA/secp256k1 for EVM, Ed25519 for Solana). ECDSA key generation involves distributed key generation across TEE nodes and takes 1--2 minutes. Ed25519 is near-instant.
4. The returned public key is used to derive a chain address (`chain.DeriveAddress`).
5. The wallet record is updated to `"ready"` with the key name, public key, and address.

### Send a transfer

1. Agent or user sends `POST /api/wallets/:id/transfer` with recipient and amount.
2. Handler builds the unsigned transaction (`chain/tx_eth.go` or `chain/tx_sol.go`).
3. If the wallet has an approval policy and the transfer exceeds the threshold (or the daily limit would be exceeded), the request is saved as a pending `ApprovalRequest` and the endpoint returns HTTP 202 with an `approval_url`.
4. If no approval is needed (or the caller is using a Passkey session and the policy allows direct signing), the handler calls `sdk.Sign()`, broadcasts the signed transaction, and returns the transaction hash.

### Approve a pending request

1. A pending approval appears in the user's Web UI (pushed via SSE) or is returned by `GET /api/approvals/pending`.
2. The user opens the approval URL in their browser and confirms with their Passkey (hardware key tap or biometric).
3. The approval handler verifies the fresh Passkey assertion against the TEENet service.
4. The transaction is **rebuilt** with a fresh nonce and gas estimate (not reused from the original request).
5. The rebuilt transaction is signed via `sdk.Sign()`, broadcast, and the approval record is updated with the transaction hash.

---

## Dual Authentication Model

TEENet Wallet uses two authentication modes that share the same API endpoints:

### API keys (`ocw_` prefix)

- Designed for **agents, bots, and automation**.
- Passed as `Authorization: Bearer ocw_...`.
- Can create wallets, send transfers, query balances, and manage the address book.
- **Cannot** approve or reject pending requests, delete wallets, or delete accounts.
- Transfers above the approval threshold automatically create a pending approval that must be confirmed with a Passkey.

### Passkey sessions (`ps_` prefix)

- Designed for **humans in the browser**.
- Obtained through WebAuthn login (hardware key or platform authenticator).
- Can do everything API keys can do, **plus** approve/reject pending requests, delete wallets, and delete user accounts.
- State-changing requests require a valid `X-CSRF-Token` header.

This separation ensures that even if an API key is compromised, an attacker cannot authorize high-value transactions or perform destructive operations without physical access to the user's Passkey device.

---

## Feature Comparison

| Traditional Wallet | TEENet Wallet |
|---|---|
| Private key in one place | Key shares distributed across TEE nodes |
| Single point of compromise | M-of-N threshold -- no single node can sign alone |
| Manual approval or no approval | Configurable USD thresholds + Passkey hardware approval |
| One chain at a time | Ethereum, Solana, and all EVM chains from one API |

---

## Next Steps

- [TEENet SDK Usage](sdk-usage.md) -- how the wallet uses the SDK in code
- [Signing & TEE Trust Model](signing-tee.md) -- what happens inside the TEENet service
- [Data Model](data-model.md) -- database tables and relationships
