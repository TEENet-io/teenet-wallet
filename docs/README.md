# TEENet Wallet

> Multi-chain crypto wallet where private keys never leave TEE hardware.

TEENet Wallet splits every private key across a cluster of **Trusted Execution Environment (TEE)** nodes using threshold cryptography. When a transaction needs to be signed, a quorum of nodes (e.g. 3-of-5) cooperates to produce a valid signature -- the full key is never reconstructed on any single machine.

## Why TEENet Wallet?

| Traditional Wallet | TEENet Wallet |
|---|---|
| Private key exists in one place | Key shares distributed across TEE nodes |
| Single point of compromise | M-of-N threshold -- no single node can sign alone |
| Manual approval or no approval | Configurable USD thresholds + Passkey hardware approval |
| One chain at a time | Ethereum, Solana, and all EVM chains from one API |

---

## I'm a User

Use TEENet Wallet through OpenClaw -- no coding required.

- [Getting Started](en/user-getting-started.md) -- Create your account, connect OpenClaw, and set up your first wallet
- [Talking to OpenClaw](en/user-commands.md) -- What you can say to manage wallets and send crypto
- [Approvals & Web UI](en/user-approvals.md) -- How to approve transactions and use the web dashboard
- [Security & FAQ](en/user-faq.md) -- How your keys are protected, plus common questions

## I'm Integrating

Build on TEENet Wallet with the REST API.

- [Quick Start](en/quick-start.md) -- Build from source, deploy, and make your first API call
- [Authentication](en/authentication.md) -- API keys for agents, Passkey for humans
- [Wallet Management](en/wallets.md) -- Create, list, and manage wallets via API
- [Transfers](en/transfers.md) -- Send native tokens, ERC-20, and SPL tokens
- [Smart Contracts](en/smart-contracts.md) -- Contract whitelist, ABI encoding, Solana programs
- [Approval System](en/approvals.md) -- USD thresholds, daily limits, contract call approval
- [AI Agent Integration](en/agent-integration.md) -- Best practices for agent platforms
- [API Reference](en/api-overview.md) -- Full endpoint reference
- [Configuration](en/configuration.md) -- Environment variables and deployment options
- [Architecture & Security](en/whitepaper.md) -- Technical deep-dive

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
