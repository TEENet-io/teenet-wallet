# TEENet Wallet

A wallet your AI agent can use -- without putting your assets at risk.

Your agent handles routine tasks like balances, transfers, and activity checks, while you set the rules: transfer limits, contract allowlists, and approval requirements. When an action exceeds your rules, you step in with a single Passkey confirmation.

> **Disclaimer:** This software manages real cryptocurrency assets. Use at your own risk. The authors are not responsible for any loss of funds. Always test thoroughly on testnets before using with real assets.

---

## What Makes This Different

- **Keys never reconstructed** -- Private keys are sharded across TEE nodes using threshold cryptography. No single machine ever holds a full key.
- **Dual auth** -- API keys for AI agents and automation; Passkeys (WebAuthn) for human approval of high-value operations.
- **Spending controls** -- USD-denominated thresholds, daily limits, and contract whitelists enforced before signing.
- **Multi-chain, one API** -- Ethereum, Solana, and all EVM-compatible chains from a single REST API.

---

## I'm a User

Use TEENet Wallet through [OpenClaw](https://openclaw.ai) -- no coding required.

- [Getting Started](en/user-getting-started.md) -- Create your account and set up your first wallet
- [Agent Commands](en/user-commands.md) -- What you can say to manage wallets and send crypto
- [Web UI & Approvals](en/user-approvals.md) -- How to use the web dashboard and approve transactions
- [FAQ](en/user-faq.md) -- Common questions about security, keys, and usage

## I'm a Developer

Build on TEENet Wallet with the REST API, or contribute to the project.

- [Introduction](en/introduction.md) -- Architecture, how signing works, and key features
- [Quick Start](en/quick-start.md) -- Build from source, deploy, and make your first API call
- [API Reference](en/api-overview.md) -- Full endpoint reference
- [AI Agent Integration](en/agent-integration.md) -- Best practices for agent platforms
- [Building & Contributing](en/developer-guide.md) -- Development setup and contribution guidelines
- [Architecture & Security](en/whitepaper.md) -- Technical deep-dive

---

## Supported Signature Schemes

TEENet Wallet supports all major signature schemes used by blockchain systems through the [TEENet platform](https://teenet.io). Chains marked with **✓** have been tested end-to-end.

| Scheme | Blockchains |
|--------|-------------|
| ECDSA secp256k1 | Ethereum **✓**, Optimism **✓**, Base **✓**, BNB Chain **✓**, Arbitrum, Polygon, Avalanche, Bitcoin, + any EVM chain |
| Ed25519 | Solana **✓** |

---

## TEENet Platform

This wallet is one application built on [TEENet](https://teenet.io) -- a platform that provides hardware-isolated runtime and managed key custody for any application that needs to protect secrets. TEENet is currently in Developer Preview.

[Platform docs](https://teenet-io.github.io/#/) · [TEENet SDK](https://github.com/TEENet-io/teenet-sdk) · [GitHub](https://github.com/TEENet-io/teenet-wallet)

**[中文文档 →](zh/)**
