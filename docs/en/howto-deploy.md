# Request a TEENet Deployment

> **Alpha note:** The public TEENet Wallet is currently in alpha. The public deployment runs with `ALPHA_MODE=true`, which hides mainnet chains at startup and exposes only 8 testnets (Sepolia, Optimism Sepolia, Arbitrum Sepolia, Base Sepolia, Polygon Amoy, BSC Testnet, Avalanche Fuji, Solana Devnet). All mainnet chains live in `chains.json` and work the same way — they're just filtered out during alpha. **Managed deployments requested through this page are not constrained by the alpha chain set** — your own production instance runs without `ALPHA_MODE` and can use any supported chain, including mainnet.

Running your own wallet instance on self-hosted infrastructure against the mock service is covered in [Quick Start](quick-start.md) and [Installation & Setup](installation.md). That path is useful for local development, integration testing, and internal evaluation — but it uses deterministic keys from the mock service and is **not safe for real funds**.

This page is about **production deployment on the TEENet platform**, where real TEE nodes hold your key shares and perform threshold signing across independent enclaves.

> The managed deployment process described below applies to **this wallet** as well as **any application built on the [TEENet SDK](https://github.com/TEENet-io/teenet-sdk)** (Go or TypeScript). If you've built a custom agent, trading system, custody service, or other SDK-based app, you can request the same managed deployment using the link at the bottom of this page.

## Managed, Not Self-Serve

TEENet-platform deployment is handled by the TEENet team end-to-end. We take care of:

- Provisioning TEE nodes and secure networking
- Registering your application with the TEENet service
- Bootstrapping key material and Passkey infrastructure
- Configuring your production base URL, approval callbacks, and monitoring
- Ongoing platform operations and upgrades

You describe your requirements; we hand back a live instance with admin access.

## What You Get

- Your application (this wallet, or your custom SDK-based app) running on the TEENet platform
- Real threshold signing across TEE nodes — keys never leave enclave hardware
- Platform maintenance handled by the TEENet team

For **wallet deployments** specifically, this also includes:

- The wallet's public URL, where users self-register (email → 6-digit code → Passkey) and manage their own API keys from the Settings page
- A pre-configured chain set, customizable per request

## What We Need From You

- Organization or project requesting the deployment
- A primary contact
- What you're deploying (this wallet / a custom SDK-based app) and your use case
- Expected user count or load profile
- Chains needed, if deploying the wallet and the default set isn't enough
- Compliance or data-residency requirements
- Target timeline

## How to Request

Open a deployment request issue on GitHub:

**[→ Request a Deployment](https://github.com/TEENet-io/teenet-wallet/issues/new?template=deployment-request.yml)**

The form collects everything we need to scope your deployment — whether it's this wallet or a custom SDK-based app. We'll follow up on the issue itself or via the contact you provide.

## After Deployment

Once your instance is live, you'll receive:

- Your application's public URL
- SDK and integration configuration for your agents or applications

For wallet deployments, the first user signs up via the public URL the same way as everyone else (email → code → Passkey); there is no separate admin account. After that, standard wallet usage applies — see [Getting Started](user-getting-started.md).

## Development vs Production

Use the **mock service** for:

- Local development
- Integration testing
- API experimentation

Do **not** use the mock service for:

- Mainnet wallets
- Real assets
- Any production flow with real value

## Related

- [TEENet Platform](https://teenet.io) — platform overview
- [TEENet SDK](https://github.com/TEENet-io/teenet-sdk) — build your own SDK-based app
- [Architecture Overview](architecture-overview.md) — how the wallet depends on TEE nodes
- [Community & Support](community.md) — general project channels
