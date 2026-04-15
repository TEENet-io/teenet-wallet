# Deploy Your Wallet App on TEENet

This page explains how TEENet Wallet deployment is scoped and what you need before deploying it on TEENet.

## Current Status

TEENet Wallet is deployed as an application on the **TEENet platform**. This page covers the wallet-side deployment requirements.

Production deployment requires a running **TEENet service** for:

- key generation
- threshold signing
- Passkey management
- approval-time verification

The local mock service is for development only. It uses deterministic keys and must never be used with real funds.

If you do not yet have access to a TEENet environment, start with the TEENet platform access/onboarding page when it is available. That page should explain how to obtain access and prepare an environment. This page assumes you are already deploying into TEENet.

## What This Page Covers

- what TEENet Wallet needs in order to run on TEENet
- how deployment on TEENet differs from local development
- which local docs still matter before deployment

This page does not cover:

- how to obtain access to the TEENet platform
- how to provision a TEENet environment
- TEENet platform operator workflows outside the wallet app itself

## Prerequisites for TEENet Deployment

Before deploying on TEENet, you need:

- a TEENet environment with a reachable TEENet service endpoint
- the wallet application built from this repository
- chain RPC configuration appropriate for the networks you want to support
- a public base URL for approval links and browser access
- the TEENet platform access/onboarding steps completed

For local build instructions and runtime configuration, see [Installation & Setup](installation.md) and [Configuration](configuration.md).

## Development vs Production

Use the mock service only for:

- local development
- integration testing
- API experimentation

Do not use the mock service for:

- mainnet wallets
- real assets
- production demos with real value

## Before You Deploy

Review these wallet-specific pages first:

- [Installation & Setup](installation.md) for build requirements and the local development workflow
- [Configuration](configuration.md) for runtime environment variables
- [Architecture Overview](architecture-overview.md) for how the wallet depends on the TEENet service

Then use the TEENet platform access/onboarding page to obtain access and prepare your TEENet environment.

## Where To Go Next

- TEENet platform access/onboarding page for environment access and platform-side setup
- [TEENet Platform docs](https://teenet-io.github.io/#/) for platform context
- [Community & Support](community.md) for project links and maintainer contacts
