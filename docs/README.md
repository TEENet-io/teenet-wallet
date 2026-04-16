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

## Use TEENet Wallet

Use TEENet Wallet through [OpenClaw](https://openclaw.ai) -- no coding required.

- [Getting Started](en/user-getting-started.md) -- Create your account and set up your first wallet
- [What You Can Do](en/user-commands.md) -- Manage wallets, send crypto, and interact with DeFi
- [Web UI & Approvals](en/user-approvals.md) -- How to use the web dashboard and approve transactions
- [FAQ](en/user-faq.md) -- Common questions about security, keys, and usage

## Build & Integrate

Build on TEENet Wallet with the REST API, integrate it into agent platforms, or prepare it for deployment on TEENet.

- **Getting Started**
  - [Quick Start](en/quick-start.md) -- Zero to running in 5 minutes
  - [Installation & Setup](en/installation.md) -- Build options, environment variables, and Docker
  - [Troubleshooting](en/troubleshooting.md) -- Common setup and runtime issues
- **Integrations**
  - [Agent Integration](en/agent-integration.md) -- Best practices for agent platforms
  - [TEENet SDK Usage](en/sdk-usage.md) -- How the wallet uses the SDK and mock service
- **Concepts**
  - [Architecture Overview](en/architecture-overview.md) -- How the system works
  - [Signing & TEE Trust Model](en/signing-tee.md) -- Threshold signing and custody model
- **How-To Guides**
  - [Add a Chain](en/howto-add-chain.md) -- Extend chain support safely
  - [Add a Plugin Tool](en/howto-add-plugin-tool.md) -- Extend the OpenClaw plugin
  - [Deploy Your Wallet App on TEENet](en/howto-deploy.md) -- Deployment scope and prerequisites
- **API Reference**
  - [OpenAPI Spec](api/openapi.yaml) -- Machine-readable API schema
  - [Authentication](en/authentication.md) -- API reference starting point
  - [Wallets](en/wallets.md) -- Wallet lifecycle endpoints
  - [Transfers](en/transfers.md) -- Native-asset transfer endpoints
  - [Address Book](en/addressbook.md) -- Saved recipient management
  - [Smart Contracts](en/smart-contracts.md) -- Contract call and token interaction endpoints
  - [Approval System](en/approvals.md) -- Approval request and confirmation flow
- **Reference**
  - [Configuration](en/configuration.md) -- Environment variables and runtime behavior
  - [Error Codes & Status Codes](en/error-codes.md) -- Error model and HTTP semantics
  - [Audit Log](en/audit-log.md) -- Audit trail endpoints and usage
  - [chains.json Schema](en/chains-schema.md) -- Chain definition format
  - [Data Model](en/data-model.md) -- Database entities and relationships

## Contribute

- [Contribution Process](en/contributing-process.md) -- How to contribute
- [Coding Standards & CI](en/coding-standards.md) -- Development expectations and checks

## Support

- [Community & Support](en/community.md) -- Project links, security reporting, and maintainer info

---

## Supported Signature Schemes

TEENet Wallet supports all major signature schemes used by blockchain systems through the [TEENet platform](https://teenet.io). Chains marked with **✓** have been tested end-to-end.

| Scheme | Blockchains |
|--------|-------------|
| ECDSA secp256k1 | Ethereum **✓**, Optimism **✓**, Base **✓**, BNB Chain **✓**, Avalanche **✓**, Arbitrum, Polygon, Bitcoin, + any EVM chain |
| Ed25519 | Solana **✓** |

---

## TEENet Platform

This wallet is one application built on [TEENet](https://teenet.io) -- a platform that provides hardware-isolated runtime and managed key custody for any application that needs to protect secrets. TEENet is currently in Developer Preview.

[Platform docs](https://teenet-io.github.io/#/) · [TEENet SDK](https://github.com/TEENet-io/teenet-sdk) · [GitHub](https://github.com/TEENet-io/teenet-wallet)

**[中文文档 →](zh/)**
