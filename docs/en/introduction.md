# Introduction

## What is TEENet Wallet?

TEENet Wallet is a multi-chain cryptocurrency wallet where private keys never exist on any single machine. Every key is split across a cluster of Trusted Execution Environment (TEE) nodes using threshold cryptography. When a transaction needs to be signed, multiple TEE nodes cooperate to produce a valid signature -- the full key is never reconstructed.

The result is a wallet that is as easy to use as a hosted API, but with security guarantees that exceed traditional custody solutions.

---

## Why TEENet Wallet?

| Traditional Wallet | TEENet Wallet |
|---|---|
| Private key in one place | Key shares distributed across TEE nodes |
| Single point of compromise | M-of-N threshold -- no single node can sign alone |
| Manual approval or no approval | Configurable USD thresholds + Passkey hardware approval |
| One chain at a time | Ethereum, Solana, and all EVM chains from one API |

---

## Key Features

- **Multi-chain** -- Ethereum, Solana, and any EVM-compatible chain out of the box. Add custom chains at runtime.
- **Two authentication modes** -- API keys for bots and agents, Passkey for human approval. Both share the same API.
- **USD spending controls** -- Per-wallet thresholds and daily limits. Stablecoins pegged at $1, volatile assets priced via CoinGecko.
- **Contract whitelist** -- Only pre-approved contracts can be called. All contract calls via API key require human approval.
- **Token transfers** -- ERC-20 and SPL tokens built, signed, and broadcast by the backend. Solana ATAs created automatically.
- **Full audit trail** -- Every operation logged with auth mode, API key label, and timestamp.

---

## How Signing Works

```
 AI Agent / App                        Human (Browser)
     |  API Key (ocw_*)                    |  Passkey (ps_*)
     v                                     v
+--------------------------------------------------------------+
|  TEENet Wallet  (:8080)                                      |
|  REST API · approval policies · contract whitelist            |
+--------------------------------------------------------------+
     |  TEENet SDK
     v
+--------------------------------------------------------------+
|  app-comm-consensus  (:8089)                                 |
|  M-of-N voting coordination                                  |
+--------------------------------------------------------------+
     |  gRPC + mutual TLS
     v
+--------------------------------------------------------------+
|  TEE-DAO Key Management Cluster  (3-5 nodes)                 |
|  Threshold signing · keys never leave TEE hardware            |
+--------------------------------------------------------------+
```

1. Client sends a request (API key or Passkey).
2. Wallet checks whitelist, threshold, and daily limit.
3. If approval is needed, the request enters a pending state until the owner confirms with Passkey.
4. Wallet routes the signing request through the TEE cluster.
5. TEE nodes produce a threshold signature -- the full key is never reconstructed.
6. Wallet broadcasts the signed transaction and returns the hash.

---
[I'm a User → Getting Started](/en/user-getting-started.md) | [I'm Integrating → Quick Start](/en/quick-start.md)
