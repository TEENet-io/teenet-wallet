# Overview

TEENet Wallet is a multi-chain crypto wallet that lets your AI agent handle routine transactions — balance checks, transfers, contract calls — while you keep approval for what matters. Set transfer limits, restrict which contracts your agent can interact with, and confirm high-value actions with a single Passkey tap. Your rules are enforced inside hardware-protected enclaves, not just in application code.

Private keys never exist on any single machine. They are generated inside TEE nodes, sharded across multiple independent nodes using threshold cryptography, and never exported or reconstructed. Signing requires cooperation from multiple nodes — no operator, cloud provider, or compromised server can unilaterally access your keys.

---

## I'm a User

Set up your wallet, manage your assets, and approve transactions.

- [Getting Started](en/user-getting-started.md) — Create your account and first wallet
- [What You Can Do](en/user-commands.md) — Supported operations and chains
- [Web UI & Approvals](en/user-approvals.md) — Approve transactions with your Passkey
- [FAQ](en/user-faq.md) — Common questions

## I'm a Developer

Build on the wallet API, integrate with agent platforms, or contribute to the codebase.

- [Quick Start](en/quick-start.md) — Get Started
- [Architecture Overview](en/architecture-overview.md) — Concepts
- [Add a Chain](en/howto-add-chain.md) — How-To Guides
- [Agent Integration](en/agent-integration.md) — Agent Integration
- [Contribution Process](en/contributing-process.md) — Contributing
