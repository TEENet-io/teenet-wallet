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

```
POST /api/wallets {chain}
        │
        ▼
  Save wallet record ──► status: "creating"
        │
        ▼
  SDK GenerateKey(scheme, curve)
        │
        ▼
  Derive chain address from public key
        │
        ▼
  Update wallet record ──► status: "ready"
```

### Send a transfer

```
POST /api/wallets/:id/transfer {to, amount}
        │
        ▼
  Build unsigned transaction
        │
        ▼
  Check approval policy ─── exceeds threshold ──► Save as pending
        │                     or daily limit          approval,
        │                                          return HTTP 202
        ▼                                         + approval_url
  SDK Sign(tx, keyName)
        │
        ▼
  Broadcast to blockchain
        │
        ▼
  Return tx hash
```

### Approve a pending request

```
Pending approval appears in Web UI (via SSE)
        │
        ▼
  User confirms with Passkey (tap / biometric)
        │
        ▼
  Verify Passkey assertion via TEENet service
        │
        ▼
  Rebuild transaction (fresh nonce + gas estimate)
        │
        ▼
  SDK Sign(tx, keyName) ──► Broadcast ──► Return tx hash
```

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
