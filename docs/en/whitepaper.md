# TEENet Wallet -- Technical Whitepaper

**Version 0.2.0 | March 2026**

---

## Abstract

TEENet Wallet is a multi-chain cryptocurrency wallet that eliminates the
single-point-of-failure inherent in conventional key management by distributing
private key material across a cluster of Trusted Execution Environment (TEE)
nodes using threshold cryptography. No single machine ever holds a complete
private key. Signing operations require the cooperation of M-of-N TEE nodes
through FROST (for Ed25519/Schnorr curves) or GG20 (for ECDSA/secp256k1
curves), providing Byzantine fault tolerance while maintaining practical
transaction latency. The wallet exposes a REST API designed for dual
consumption: AI agents authenticate with API keys for programmatic automation,
while human operators authenticate with WebAuthn/Passkey hardware credentials
for sensitive operations. A layered security model comprising contract
whitelists, method-level restrictions, USD-denominated approval thresholds with
real-time price oracles, and daily spend limits with auth/capture rollback
semantics provides defense-in-depth against both compromised credentials and
software bugs. The system supports Ethereum and all EVM-compatible chains, as
well as Solana, with runtime extensibility for additional EVM networks.

---

## 1. Introduction

### 1.1 The Key Management Problem

Cryptocurrency security reduces to a single challenge: protecting private keys.
The spectrum of existing solutions presents an unsatisfying set of trade-offs:

- **Hot wallets** (MetaMask, embedded browser wallets) store keys in software
  memory or encrypted files on general-purpose hardware. They are convenient
  but vulnerable to malware, phishing, and supply-chain attacks. A single
  compromised machine means total loss of funds.

- **Hardware wallets** (Ledger, Trezor) isolate keys in secure elements but
  require physical human interaction for every signature. They are unsuitable
  for automated workflows, DeFi strategies, or AI agent operation. They also
  remain single devices -- a lost or damaged unit can mean permanent key loss
  without careful seed phrase management.

- **MPC wallets** (Fireblocks, Fordefi) distribute key shares across multiple
  parties, eliminating single points of failure. However, commercial MPC
  solutions are typically opaque, proprietary, and expensive. The key shares
  reside in the vendor's infrastructure, creating custodial trust assumptions.

- **Smart contract wallets** (Safe, ERC-4337 Account Abstraction) move
  authorization logic on-chain. They are chain-specific, require gas for
  configuration changes, and cannot sign arbitrary messages for off-chain
  protocols.

### 1.2 TEENet Wallet's Approach

TEENet Wallet combines the security properties of hardware isolation with the
operational flexibility of software wallets by leveraging a distributed TEE-DAO
cluster for all cryptographic operations:

1. **Threshold cryptography** -- private keys are generated and used in
   distributed form. The complete key is never reconstructed on any single
   machine.

2. **TEE hardware isolation** -- key shares reside inside Trusted Execution
   Environments, providing hardware-enforced memory encryption and attestation.

3. **Dual authentication** -- API keys enable AI agent automation, while
   WebAuthn/Passkey credentials gate sensitive operations with hardware-backed
   human presence verification.

4. **Defense-in-depth** -- contract whitelists, method restrictions, USD
   approval thresholds, daily spend limits, and CSRF protection create multiple
   independent security layers.

5. **Multi-chain support** -- a single deployment handles Ethereum, Solana, and
   any EVM-compatible chain, with runtime chain addition.

---

## 2. System Architecture

### 2.1 Component Overview

```
AI Agent / Application              Human Operator (Browser)
    |  Authorization: Bearer ocw_*      |  Authorization: Bearer ps_*
    v                                    v
+--------------------------------------------------------------+
|  TEENet Wallet  (:8080)                                      |
|  +-- Transaction Builder (EVM EIP-1559, Solana)              |
|  +-- Contract Whitelist + Method Gate                        |
|  +-- Approval Policy Engine (USD thresholds, daily limits)   |
|  +-- Approval Queue (pending/approved/rejected/expired)      |
|  +-- Audit Logger                                            |
|  +-- Price Oracle (CoinGecko, 10s cache)                     |
|  +-- Nonce Manager (concurrent EVM tx safety)                |
|  +-- Idempotency Store (duplicate transfer prevention)       |
+--------------------------------------------------------------+
    |  TEENet SDK (HTTP REST)
    v
+--------------------------------------------------------------+
|  TEENet Service  (:8089)                                    |
|  +-- M-of-N Voting Coordination                             |
|  +-- Config Cache (apps, instances, keys from UMS)           |
+--------------------------------------------------------------+
    |  tee-dao-key-management-sdk (gRPC + mutual TLS)
    v
+--------------------------------------------------------------+
|  TEE-DAO Key Management Cluster  (3-5 nodes)                 |
|  +-- Raft Consensus (hashicorp/raft)                         |
|  +-- Threshold Signing (FROST / GG20)                        |
|  +-- Distributed Key Generation (DKG)                        |
|  +-- Key Shares in TEE Enclaves                              |
|  +-- Mutual TLS between all nodes                            |
+--------------------------------------------------------------+
```

### 2.2 Data Flow

**Wallet Creation (DKG):**

1. Client sends `POST /api/wallets` with chain selection.
2. TEENet Wallet validates the request and determines the required
   cryptographic protocol (ECDSA for EVM, Schnorr for Solana).
3. The TEENet SDK initiates Distributed Key Generation across the TEE-DAO
   cluster. For ECDSA (GG20), this involves multiple rounds of communication
   and can take 60-120 seconds.
4. Each TEE node generates and stores its key share. The public key is
   aggregated and returned.
5. TEENet Wallet derives the blockchain address from the public key (Keccak-256
   truncation for EVM, Ed25519 encoding for Solana) and persists the wallet
   record.

**Transaction Signing:**

1. Client sends a transfer or contract call request.
2. TEENet Wallet builds the unsigned transaction server-side (EIP-1559 for EVM,
   Solana transaction message for Solana).
3. The approval policy engine evaluates whether the transaction requires human
   approval (based on USD amount and authentication type).
4. If approved (or below threshold), the signing hash is sent to the
   TEENet service via the TEENet SDK.
5. The TEENet service coordinates M-of-N threshold signing across the TEE-DAO
   cluster.
6. The signature is returned to TEENet Wallet, which assembles the signed
   transaction and broadcasts it to the blockchain RPC endpoint.
7. The transaction hash is returned to the client.

### 2.3 Database Design

TEENet Wallet uses SQLite with WAL (Write-Ahead Logging) mode for concurrent
read performance and operational simplicity. The schema comprises seven tables:

| Table | Purpose |
|-------|---------|
| `users` | Local user accounts linked to UMS Passkey identities |
| `wallets` | TEE-backed wallets with chain, address, key reference, and status |
| `allowed_contracts` | Per-wallet contract whitelist with method restrictions |
| `approval_policies` | Per-wallet USD threshold and daily limit configuration |
| `approval_requests` | Pending/completed approval queue with transaction context |
| `audit_logs` | Immutable operation log with auth mode, IP, and details |
| `idempotency_records` | Transfer deduplication cache |
| `custom_chains` | User-added EVM chain configurations |

---

## 3. Cryptographic Design

### 3.1 Threshold Cryptography

TEENet Wallet relies on two threshold signature schemes, selected automatically
based on the target blockchain's requirements:

**FROST (Flexible Round-Optimized Schnorr Threshold Signatures):**
Used for Ed25519 curves (Solana). FROST provides a two-round signing protocol
that is significantly faster than GG20. The scheme produces standard Ed25519
signatures that are indistinguishable from single-signer signatures on-chain.

**GG20 (Gennaro-Goldfeder 2020):**
Used for ECDSA on secp256k1 (Ethereum, all EVM chains) and secp256r1 (NIST
P-256, used for WebAuthn). GG20 enables distributed ECDSA signing without a
trusted dealer. The protocol requires pre-computed parameters (safe primes,
Paillier keys) which are generated by the prime-service to avoid blocking
signing operations.

| Protocol | Curve | Blockchain | DKG Time | Sign Time |
|----------|-------|-----------|----------|-----------|
| FROST | Ed25519 | Solana | ~5-10s | <1s |
| GG20 | secp256k1 | Ethereum, EVM | 60-120s | 2-5s |
| GG20 | secp256r1 | WebAuthn (internal) | 60-120s | 2-5s |

### 3.2 Distributed Key Generation

Key generation is performed collaboratively across all TEE-DAO nodes using a
verifiable secret sharing protocol. The process ensures:

1. **No trusted dealer** -- no single party chooses the private key.
2. **Verifiability** -- each node can verify that other nodes provided valid
   shares through zero-knowledge proofs (Feldman VSS for FROST, Pedersen
   commitments for GG20).
3. **Threshold property** -- any subset of M nodes (out of N total) can
   reconstruct a valid signature, but fewer than M nodes learn nothing about
   the private key.

The typical configuration is 3-of-5: five TEE nodes participate in key
generation, and any three can cooperate to sign. This tolerates the failure or
compromise of up to two nodes.

### 3.3 M-of-N Signing

The signing flow is coordinated by the TEENet service, which acts as the
session coordinator:

1. The coordinator receives the message hash to be signed.
2. It selects M available TEE nodes from the cluster.
3. Each selected node performs its local computation using its key share.
4. The partial signatures are combined into a complete threshold signature.
5. The resulting signature is a standard ECDSA or Schnorr signature,
   indistinguishable from a single-signer signature.

The M-of-N property means that even if an attacker compromises M-1 TEE nodes,
they cannot produce a valid signature. Combined with TEE hardware isolation,
this provides strong security guarantees.

### 3.4 Key Share Isolation

Each TEE node stores its key share inside the Trusted Execution Environment.
TEE technology (Intel SGX, AMD SEV, ARM TrustZone) provides:

- **Memory encryption** -- key material is encrypted in RAM, inaccessible even
  to the host operating system or hypervisor.
- **Remote attestation** -- clients can cryptographically verify that the
  correct software is running inside the TEE before sending sensitive data.
- **Sealed storage** -- key shares are encrypted to the specific TEE instance
  and cannot be extracted to run on different hardware.

---

## 4. Security Model

### 4.1 Threat Model

TEENet Wallet is designed to resist the following threats:

| Threat | Mitigation |
|--------|------------|
| Stolen API key | API keys cannot delete wallets, remove contracts, change policies, or approve contract calls or sensitive operations. All such operations require Passkey. |
| Stolen Passkey session token | Approvals, deletions, and API key generation require a *fresh* WebAuthn assertion (hardware key interaction), not just the session token. |
| Compromised single TEE node | M-of-N threshold signing ensures that fewer than M compromised nodes cannot produce signatures. |
| Malicious contract interaction | Address whitelist + mandatory Passkey approval for all contract calls via API key. |
| Price manipulation for threshold bypass | Stablecoins are hardcoded to $1; volatile asset prices use CoinGecko with 10-second caching. The `amount_usd` field for DeFi calls provides an additional caller-reported value, and the system uses the larger of computed and reported values. |
| Replay attacks (duplicate transactions) | Idempotency-Key header prevents duplicate transfers. EVM nonce management prevents on-chain replay. |
| CSRF attacks on browser sessions | All state-changing Passkey session requests require an X-CSRF-Token header. |
| DKG resource exhaustion | Rate limiting on wallet creation (per-user) and registration (per-IP) prevents TEE cluster abuse. |

### 4.2 Authentication Layers

TEENet Wallet implements a dual-authentication architecture designed for the
coexistence of automated agents and human operators:

**API Key Authentication (`ocw_` prefix):**
- Generated via Passkey-authenticated endpoint (requires fresh WebAuthn
  assertion).
- Stored as salted SHA-256 hash; the raw key is shown only once at generation.
- Suitable for AI agents, backend services, and automated trading bots.
- Cannot perform destructive operations (wallet deletion, contract removal,
  policy deletion, account deletion).
- Cannot approve pending approval requests.
- Subject to per-key rate limiting (configurable, default 200 req/min).

**Passkey Session Authentication (`ps_` prefix):**
- Created via WebAuthn login ceremony (hardware key interaction).
- Sessions expire after 24 hours.
- All state-changing requests require an X-CSRF-Token header.
- Sensitive operations (approvals, deletions, key generation) additionally
  require a *fresh* WebAuthn assertion within the same request body.

This "fresh credential" requirement is a critical security property: even if an
attacker obtains a valid session token (e.g., through XSS), they cannot approve
high-value transactions without physical access to the user's hardware key.

### 4.3 Contract Whitelist (3-Layer Model)

Smart contract interactions pass through three independent security layers:

**Layer 1 -- Address Whitelist:**
Only pre-approved contract addresses (EVM) or program IDs (Solana) can be
called. Any interaction with a non-whitelisted address is rejected with HTTP
403. Adding a contract to the whitelist requires Passkey authentication (or
creates a pending approval if initiated by an API key).

**Layer 2 -- Mandatory Approval for API Key Auth:**
All contract calls initiated via API key require Passkey approval. This is a
deliberate design choice: contract interactions carry higher risk than simple
transfers, and the wallet enforces human-in-the-loop confirmation for every
contract call initiated by an agent. Contract calls made via a Passkey session
(where the human is already authenticated) execute immediately.

The convenience endpoints `approve-token` and `revoke-approval` also always
require Passkey approval because they grant or revoke third-party spending
access.

### 4.4 Approval Policies

Each wallet can have a single USD-denominated approval policy with two
configurable thresholds:

**Single-Transaction Threshold (`threshold_usd`):**
When a transfer or contract call involves a USD amount exceeding this threshold,
the request is queued as a pending approval (HTTP 202) instead of being
executed immediately. The user must approve it via the web UI with a fresh
Passkey assertion.

**Daily Spend Limit (`daily_limit_usd`):**
An optional hard cap on total USD spent per UTC day. Unlike the threshold
(which merely triggers approval), the daily limit is a hard block -- once
reached, no further transfers are permitted regardless of authentication mode.

**Price Conversion:**
USD amounts are computed at request time using real-time prices:
- Volatile assets (ETH, SOL): CoinGecko API with 10-second cache.
- Stablecoins (USDC, USDT, DAI, etc.): hardcoded to $1.00.
- DeFi calls: the optional `amount_usd` field lets callers report the USD
  value of complex operations (e.g., a Uniswap swap). The system uses the
  larger of the computed native value and the caller-reported value.

**Auth/Capture Pattern:**
Daily spend tracking uses a pre-deduction model with rollback:

1. Before signing, the estimated USD amount is deducted from the daily budget.
2. If signing or broadcasting fails, the deduction is rolled back.
3. If the transaction requires approval (threshold exceeded), the pre-deduction
   is rolled back immediately; it will be re-applied when the approval is
   executed.

This prevents "phantom spend" from failed transactions while ensuring the
daily limit is never silently exceeded during concurrent requests. A per-wallet
mutex serializes the check-and-deduct operation to prevent TOCTOU races.

### 4.5 CSRF and Rate Limiting

**CSRF Protection:**
All state-changing API requests from Passkey sessions (POST, PUT, PATCH,
DELETE) must include an `X-CSRF-Token` header matching the token returned at
login. This prevents cross-site request forgery attacks against the browser-
based web UI.

**Rate Limiting:**
Three independent rate limiters protect different resources:
- General API rate limit: per-API-key, configurable (default 200 req/min).
- Wallet creation rate limit: per-user, configurable (default 5/min). This is
  critical because each wallet creation triggers a TEE DKG operation that
  consumes significant cluster compute.
- Registration rate limit: per-IP, configurable (default 10/min). Prevents
  anonymous abuse of the public registration endpoints.

---

## 5. Multi-Chain Support

### 5.1 EVM Chains (Ethereum, L2s, Testnets)

All EVM chains use ECDSA on secp256k1. TEENet Wallet builds EIP-1559
(type-2) transactions with dynamic fee estimation:

- Base fee and priority fee are queried from the chain RPC at build time.
- Gas limit is estimated via `eth_estimateGas`.
- A built-in nonce manager tracks pending nonces per address to support
  concurrent transaction submission without nonce collisions.
- Transactions are assembled with the TEE-provided ECDSA signature and
  broadcast via `eth_sendRawTransaction`.

**Supported built-in chains:**
Ethereum Mainnet, Optimism Mainnet, Sepolia Testnet, Holesky Testnet, Base
Sepolia Testnet, BSC Testnet.

**Custom chain extensibility:**
Additional EVM chains can be added at runtime via `POST /api/chains` (Passkey
required), specifying a name, RPC URL, currency symbol, and chain ID. Custom
chains are persisted in the database and survive restarts.

### 5.2 Solana

Solana uses Ed25519 signatures produced via the FROST threshold scheme. TEENet
Wallet supports:

- **Native SOL transfers** -- standard system program transfers.
- **SPL token transfers** -- TransferChecked instruction with automatic
  Associated Token Account (ATA) creation for recipients who do not yet have
  one.
- **Wrap/Unwrap SOL** -- create and close wSOL ATAs for DeFi interoperability.
- **Generic program calls** -- arbitrary Solana program instructions with
  caller-specified account lists and instruction data.

Solana transactions are rebuilt with a fresh blockhash at approval time (since
blockhashes expire in approximately 60 seconds), ensuring that approved
transactions remain valid.

### 5.3 Address Derivation

Address derivation is performed server-side from the TEE-generated public key:

- **EVM:** Keccak-256 hash of the uncompressed public key (without the 0x04
  prefix byte), taking the last 20 bytes, with EIP-55 mixed-case checksum
  encoding.
- **Solana:** The Ed25519 public key is directly used as the address,
  encoded in Base58.

---

## 6. AI Agent Integration

### 6.1 Dual-Auth Design for Agent Safety

TEENet Wallet's authentication model is specifically designed for the emerging
pattern of AI agents managing cryptocurrency:

1. **API keys grant operational access** -- an AI agent can create wallets,
   check balances, and transfer tokens below the threshold.

2. **Passkeys gate irreversible actions** -- the human owner retains exclusive
   control over wallet deletion, contract whitelist management, policy changes,
   and high-value transaction approval.

3. **The approval queue bridges the gap** -- when an agent initiates an action
   that exceeds its authority (e.g., a transfer above the USD threshold), the
   request is queued for human review rather than rejected outright. The agent
   receives an `approval_id` and `approval_url` that it can present to the
   user.

This design allows AI agents to operate autonomously within well-defined
boundaries while preserving human oversight for security-critical decisions.

### 6.2 USD Reporting for DeFi Calls

The `amount_usd` field on the `contract-call` endpoint addresses a fundamental
challenge in DeFi: the on-chain value of a transaction is often opaque.

For example, a Uniswap swap encodes the trade parameters in the calldata, but
the native ETH value sent may be zero (for token-to-token swaps). Without
`amount_usd`, the approval policy engine has no way to know whether a
$10 or $10,000 swap is being executed.

By accepting a caller-reported USD value, TEENet Wallet enables AI agents to
provide their own valuation of complex DeFi operations. The system uses the
larger of the computed native value and the reported `amount_usd`, preventing
agents from under-reporting to bypass thresholds.

### 6.3 OpenClaw Skill System

TEENet Wallet includes an OpenClaw skill definition (`skill/tee-wallet/`)
that enables natural language interaction with the wallet through AI assistant
platforms. The skill exposes wallet management, balance checking, and
transaction operations through a structured interface that AI agents can
invoke.

---

## 7. Operational Considerations

### 7.1 Deployment

TEENet Wallet is distributed as a statically-compiled Go binary or Docker
image:

- **Docker image** uses a multi-stage build with a non-root runtime user.
- The only runtime dependency is a TEENet mesh node reachable on the
  configured `SERVICE_URL`.
- All configuration is via environment variables (12-factor app style).
- The server binds to `0.0.0.0:8080` by default.

### 7.2 Database

SQLite with WAL mode provides:
- **Concurrent reads** -- WAL mode allows multiple readers without blocking
  writers, suitable for the read-heavy workload pattern.
- **Operational simplicity** -- no external database process to manage. The
  database is a single file in the configured `DATA_DIR`.
- **Busy timeout** -- configured at 5 seconds to handle concurrent write
  contention gracefully.
- **Automatic migrations** -- the schema is created and updated via GORM
  AutoMigrate at startup.

For production deployments handling high write concurrency, the data directory
should be on local SSD storage (not network-attached) to minimize WAL
checkpoint latency.

### 7.3 Graceful Shutdown

The server handles SIGINT and SIGTERM signals with a 30-second graceful
shutdown period:

1. Stop accepting new connections.
2. Wait for in-flight requests to complete (up to 30 seconds).
3. Close the database connection pool.
4. Stop background goroutines (session cleanup, rate limiter cleanup,
   idempotency cleanup, price service).

This ensures that in-progress signing operations are not interrupted, which
could leave nonces in an inconsistent state.

### 7.4 Audit Logging

Every significant operation is recorded in the `audit_logs` table with:

- **User ID** -- who performed the action.
- **Action** -- what was done (e.g., `transfer`, `wallet_create`,
  `approval_approve`, `apikey_generate`).
- **Auth mode** -- how the user authenticated (`passkey` or `apikey`).
- **IP address** -- where the request originated.
- **Wallet ID** -- which wallet was affected (if applicable).
- **Details** -- JSON context including transaction hashes, amounts,
  recipients, and approval IDs.
- **Status** -- outcome (`success`, `pending`, `failed`).

Audit writes are best-effort (errors are logged but do not fail the request)
to prevent audit infrastructure issues from blocking critical operations.

### 7.5 Security Headers

The web UI is served with restrictive security headers:

- `Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'`
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: strict-origin-when-cross-origin`

---

## 8. Comparison with Alternatives

### 8.1 vs. Hot Wallets (MetaMask, Embedded Wallets)

| Dimension | Hot Wallet | TEENet Wallet |
|-----------|-----------|---------------|
| Key storage | Software (browser, disk) | Distributed across TEE hardware |
| Single point of failure | Yes (one device) | No (M-of-N threshold) |
| Automation support | Limited (requires browser extension) | Native API key support |
| Human oversight | None (any software can sign) | Passkey gate for sensitive ops |
| Recovery | Seed phrase (single secret) | Key resharing across TEE cluster |

### 8.2 vs. Hardware Wallets (Ledger, Trezor)

| Dimension | Hardware Wallet | TEENet Wallet |
|-----------|----------------|---------------|
| Automation | Impossible (requires physical button press) | Full API automation within policy bounds |
| Redundancy | Single device (backup via seed phrase) | M-of-N across multiple TEE nodes |
| Multi-chain | Device firmware dependent | Server-side, extensible at runtime |
| AI agent integration | None | First-class dual-auth design |
| Transaction review | Device screen (limited) | Web UI with full context |

### 8.3 vs. MPC Wallets (Fireblocks, Fordefi)

| Dimension | Commercial MPC | TEENet Wallet |
|-----------|---------------|---------------|
| Infrastructure | Vendor-hosted (custodial trust) | Self-hosted TEE cluster |
| Key share location | Vendor + client | User-controlled TEE nodes |
| Cost | Per-transaction or subscription | Infrastructure cost only |
| Transparency | Proprietary protocols | Open threshold crypto (FROST, GG20) |
| Customization | Vendor-defined policies | Fully configurable policies, whitelists |
| AI agent support | Varies by vendor | Native dual-auth design |

### 8.4 vs. Smart Contract Wallets (Safe, Account Abstraction)

| Dimension | Smart Contract Wallet | TEENet Wallet |
|-----------|----------------------|---------------|
| Chain support | Single chain per deployment | Multi-chain from single instance |
| Gas for config changes | Yes (on-chain transactions) | No (off-chain policy management) |
| Arbitrary message signing | Limited (EIP-1271) | Full support (any message format) |
| Solana support | No (EVM only) | Yes |
| Recovery | On-chain social recovery | TEE key resharing |
| Latency | On-chain confirmation for policy | Instant off-chain policy updates |

---

## 9. Future Work

The following areas are under active development or consideration:

- **Bitcoin support** -- BIP-340 Schnorr signatures on secp256k1 via FROST,
  enabling native Bitcoin and Taproot transactions.

- **Key resharing** -- redistribute key shares across a changed set of TEE
  nodes without generating a new key or changing the on-chain address. The
  underlying TEE-DAO cluster already supports resharing; integration into the
  wallet API is planned.

- **Multi-user approval workflows** -- require M-of-N human approvers
  (in addition to M-of-N TEE nodes) for high-value transactions, suitable
  for organizational treasury management.

- **Spending policy DSL** -- a richer policy language supporting per-contract
  limits, time-of-day restrictions, recipient whitelists, and velocity checks.

- **Hardware attestation verification** -- cryptographic verification of TEE
  attestation reports by the wallet, providing end-to-end proof that key
  shares are running inside genuine TEE enclaves.

- **ERC-4337 bundler integration** -- support for Account Abstraction
  UserOperations, enabling gasless transactions and batched operations.

- **Cross-chain atomic operations** -- coordinated signing across multiple
  chains for bridge operations and cross-chain DeFi.

- **Enhanced audit and compliance** -- exportable audit trails, webhook
  notifications for approvals, and integration with compliance monitoring
  services.

---

## 10. Conclusion

TEENet Wallet demonstrates that threshold cryptography and Trusted Execution
Environments can provide a practical, production-grade alternative to
conventional cryptocurrency key management. By distributing key shares across
multiple TEE nodes and requiring M-of-N cooperation for every signature, the
system eliminates the single point of failure that plagues hot wallets while
maintaining the programmability that hardware wallets lack.

The dual-authentication model -- API keys for agents, Passkeys for humans --
addresses the emerging need for AI-agent-operated wallets with human oversight.
The layered security model (contract whitelists, method gates, USD thresholds,
daily limits) provides configurable defense-in-depth that can be tuned to the
risk tolerance of each deployment.

Multi-chain support across EVM and Solana ecosystems, combined with runtime
chain extensibility, ensures the wallet remains relevant as the blockchain
landscape evolves. The self-hosted architecture avoids the custodial trust
assumptions of commercial MPC solutions while providing comparable security
guarantees through open, well-studied cryptographic protocols.

---

*TEENet Wallet is open-source software released under the MIT license.*

---
[Previous: Developer Guide](/en/developer-guide.md)
