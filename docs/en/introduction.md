# Introduction

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
[Next: Quick Start](quick-start.md)
