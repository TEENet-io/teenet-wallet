# Request a TEENet Deployment

> **Alpha note:** The public TEENet Wallet is in alpha and shows testnets only. Managed deployments requested through this page are separate TEENet-hosted instances; they can run without `ALPHA_MODE` and use any supported chain, including mainnet.

This page is for developers who want to run TEENet Wallet or a [TEENet SDK](https://github.com/TEENet-io/teenet-sdk) application on the TEENet platform. A deployment can be for a pilot, private demo, staging environment, or production workload.

For local development, follow [Quick Start](quick-start.md) or [Installation & Setup](installation.md) and use the mock service. The mock service uses pre-generated keys and is **not safe for real funds**.

## When to Request a Deployment

Request a managed deployment when you are ready to move beyond the local mock service and need:

- A hosted TEENet Wallet for users, with mainnet or custom chain support if needed
- A managed deployment for your TEENet SDK-based app

SDK app deployments can cover agents, trading systems, custody services, or other Go/TypeScript integrations.

## What TEENet Handles

TEENet deployment is managed end-to-end. We handle:

- Provisioning TEE nodes and secure networking
- Registering your application with the TEENet service
- Bootstrapping key material and Passkey infrastructure
- Configuring your deployment URL, approval callbacks, and monitoring
- Ongoing platform operations and upgrades

## What You Get

- Your wallet or SDK-based app running on the TEENet platform
- Real threshold signing across independent TEE nodes; key shares stay inside enclave hardware
- Deployment URLs and integration settings for your app or agents

For wallet deployments, you also get:

- A public wallet URL where users sign up with email, code, and Passkey
- A pre-configured chain set, customized for your deployment

## What We Need From You

- Organization or project requesting the deployment
- A primary contact
- Deployment type, use case, and integration flow
- Expected user count or load profile
- For wallet deployments, your required chain set
- Callback URLs, domains, or SDK app integration details
- Compliance or data-residency requirements
- Target timeline

## How to Request

Open a deployment request issue on GitHub:

**[→ Request a Deployment](https://github.com/TEENet-io/teenet-wallet/issues/new?template=deployment-request.yml)**

Use the issue to share the details above. We'll follow up there or via the contact you provide.

## What Happens Next

Once your instance is live, you'll receive:

- Your deployment URL
- SDK and integration configuration for your agents or applications
- Chain configuration details

For wallet deployments, the first user signs up through the public URL like any other user; there is no separate admin account. After that, standard wallet usage applies. See [Getting Started](user-getting-started.md).

## Related

- [TEENet Platform](https://teenet.io) — platform overview
- [TEENet SDK](https://github.com/TEENet-io/teenet-sdk) — build your own SDK-based app
- [Architecture Overview](architecture-overview.md) — how the wallet depends on TEE nodes
- [Community & Support](community.md) — general project channels
