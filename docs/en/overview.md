# Overview

TEENet Wallet is a multi-chain cryptocurrency wallet where your private keys never exist on any single machine. Every key is split across multiple Trusted Execution Environment (TEE) nodes — when a transaction needs to be signed, the nodes cooperate to produce a valid signature without ever reconstructing the full key.

The wallet supports Ethereum, Solana, and all major EVM chains from a single interface. You can manage wallets through a web UI or automate operations through a REST API with configurable spending policies and Passkey-based human approval for high-value transactions.

---

## I'm a User

Set up your wallet, manage your assets, and approve transactions.

- [Getting Started](en/user-getting-started.md) — Create your account and first wallet
- [What You Can Do](en/user-commands.md) — Supported operations and chains
- [Web UI & Approvals](en/user-approvals.md) — Approve transactions with your Passkey
- [FAQ](en/user-faq.md) — Common questions

## I'm a Developer

Build on the wallet API, integrate with agent platforms, or contribute to the codebase.

- [Quick Start](en/quick-start.md) — Zero to running in 5 minutes
- [Architecture Overview](en/architecture-overview.md) — How the system works
- [API Reference](en/authentication.md) — Full endpoint reference
- [Agent Integration](en/agent-integration.md) — Best practices for agent platforms
- [Contributing](en/contributing-process.md) — Contribution process and coding standards
