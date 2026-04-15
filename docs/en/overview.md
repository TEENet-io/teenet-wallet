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

- [Quick Start](en/quick-start.md) — Zero to running in 5 minutes
- [Architecture Overview](en/architecture-overview.md) — How the system works
- [API Reference](en/authentication.md) — Full endpoint reference
- [Agent Integration](en/agent-integration.md) — Best practices for agent platforms
- [Contributing](en/contributing-process.md) — Contribution process and coding standards
