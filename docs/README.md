# TEENet Wallet

> Multi-chain crypto wallet where private keys never leave TEE hardware.

TEENet Wallet splits every private key across a cluster of **Trusted Execution Environment (TEE)** nodes using threshold cryptography. When a transaction needs to be signed, a quorum of nodes (e.g. 3-of-5) cooperates to produce a valid signature — the full key is never reconstructed on any single machine.

## Why TEENet Wallet?

| Traditional Wallet | TEENet Wallet |
|---|---|
| Private key exists in one place | Key shares distributed across TEE nodes |
| Single point of compromise | M-of-N threshold — no single node can sign alone |
| Manual approval or no approval | Configurable USD thresholds + Passkey hardware approval |
| One chain at a time | Ethereum, Solana, and all EVM chains from one API |

## Who is it for?

- **AI Agents** — Programmatic custody via API keys with configurable safety rails
- **DeFi Automation** — Execute trades below thresholds without human intervention
- **Institutional Teams** — Multi-layer approval policies with USD-denominated spend limits

## Quick Links

- [Quick Start](en/quick-start.md) — Create a wallet and send your first transaction in 5 minutes
- [Authentication](en/authentication.md) — API keys for agents, Passkey for humans
- [Approval System](en/approvals.md) — USD thresholds, daily limits, high-risk method gates
- [Smart Contracts](en/smart-contracts.md) — Contract whitelist, ABI encoding, Solana programs
- [AI Agent Guide](en/agent-integration.md) — Best practices for agent integration
- [API Reference](en/api-overview.md) — Full endpoint reference
- [Architecture & Security](en/whitepaper.md) — Technical deep-dive

## Supported Chains

| Chain | Currency | Family |
|-------|----------|--------|
| Ethereum Mainnet | ETH | EVM |
| Optimism Mainnet | ETH | EVM |
| Sepolia Testnet | ETH | EVM |
| Holesky Testnet | ETH | EVM |
| Base Sepolia | ETH | EVM |
| BSC Testnet | tBNB | EVM |
| Solana Mainnet | SOL | Solana |
| Solana Devnet | SOL | Solana |
| + Custom EVM chains via API | | |

---

**[中文文档 →](zh/)**
