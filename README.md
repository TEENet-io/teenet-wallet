# TEENet Wallet

[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](LICENSE) 
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8.svg?logo=go&logoColor=white)](https://go.dev) 
[![Developer Preview](https://img.shields.io/badge/Status-Developer_Preview-orange.svg)]()

```text


+-------------+      +--------------------------+    outside    +--------+---------+
| Agent / App | ---> | TEENet Wallet            | -- policy --> | User Approval    |
| API key     |      | Policy engine            |               | Passkey          |
| - balances  |      | - transfer limits        |               +--------+---------+
| - transfers |      | - contract allowlists    |                        |
| - contracts |      | - daily spend caps       |                        | approved
+-------------+      | - approval queue         |                        v
                     | - audit log / routing    |          +-------------+--------------+    
                     +-------------+------------+          | TEE Threshold Sign         |       
                                   |                       | - key shares stay inside   |
                                   |---------------------> |   enclaves                 |       
                                       within policy       | - no full key on any one   |
                                                           |   machine                  |
                                                           +----------------------------+


```

A wallet your AI agent can use without handing over your assets. Your agent handles balances, transfers, and contract calls through one API, while TEENet Wallet enforces transfer limits, contract allowlists, daily spend caps, and approval rules before anything reaches signing. Low-risk actions can execute automatically; anything outside policy pauses for a single Passkey confirmation.

Open source. Hardware-enforced rules. Passkey approval.

> **Disclaimer:** This software manages real cryptocurrency assets. Use at your own risk. The authors are not responsible for any loss of funds. Always test thoroughly on testnets before using with real assets.

## How It Works

- An agent or application authenticates with an API key and submits a balance check, transfer, or contract call.
- TEENet Wallet evaluates transfer limits, daily spend caps, contract allowlists, and approval rules before signing.
- Requests that satisfy policy go directly to threshold signing inside TEEs.
- Requests that exceed policy wait for a browser-based Passkey approval, then continue to signing.

Private keys are sharded across independent TEE nodes, never exported, and the hardware guarantees that the running code cannot be modified or bypassed.

## Features

- **Multi-chain** -- Ethereum, Solana, Optimism, Arbitrum, Base, Polygon, BNB Chain, Avalanche, and custom EVM chains
- **Dual auth** -- API keys for AI agents and automation; Passkeys (WebAuthn) for human approval of high-value operations
- **Smart contract security** -- contract whitelist + Passkey approval for all contract operations via API key
- **Approval policies** -- USD-denominated thresholds, daily spend limits, real-time price feeds
- **Address book** -- nickname-based transfers with per-chain validation
- **Agent-ready** -- [OpenClaw](https://openclaw.ai) plugin and skill integration, idempotent transfers, SSE event stream, audit logging

## Start Here

- [Quick Start](https://teenet-io.github.io/teenet-wallet/#/en/quick-start) -- local setup, first wallet, first API key
- [Architecture Overview](https://teenet-io.github.io/teenet-wallet/#/en/architecture-overview) -- system model, policy flow, trust boundaries
- [OpenAPI Spec](docs/api/openapi.yaml) -- machine-readable API schema
- [Full Documentation](https://teenet-io.github.io/teenet-wallet/#/en/overview) -- user guides, deployment notes, and API reference

## Quick Start

### Prerequisites

- Go 1.25+
- Node.js + npm (required for `make frontend`)
- SQLite3 development headers (`apt-get install libsqlite3-dev` on Debian/Ubuntu; `dnf install sqlite-devel` on RHEL/Fedora/Alibaba Cloud Linux)
- A TEENet service endpoint (`teenet-sdk/mock-server` for local development)

### One-command dev setup

```bash
./scripts/dev.sh up
```

This clones `teenet-sdk` into a sibling directory if missing, builds both services, picks matching ports (override with `MOCK_PORT=` / `WALLET_PORT=`, or set `AUTO_PORT=1` to skip over busy ports), aligns `PASSKEY_RP_ORIGIN` automatically, and health-checks both. Use `down` to stop, `status` to inspect, `logs` to tail. Runtime state (PIDs, logs, SQLite) lives in `.dev/`. If you'd rather run the pieces by hand, follow the two sections below.

### Start the Mock Service

```bash
git clone https://github.com/TEENet-io/teenet-sdk.git
cd teenet-sdk/mock-server
make run
```

The mock service listens on `http://0.0.0.0:8089` by default (bound on all interfaces so bridge-networked Docker containers can reach it; override with `MOCK_SERVER_BIND=127.0.0.1` to restrict to loopback-only). `make run` sets the Passkey defaults (`PASSKEY_RP_ORIGIN=http://localhost:18080`) that match the wallet's default origin. If you run the wallet on a non-default `PORT`, start the mock with a matching `PASSKEY_RP_ORIGIN=http://localhost:<port>` — WebAuthn requires an exact origin match. Leave this terminal running.
When the mock server starts, it prints the available test app instance IDs.
Use one of those printed values as `APP_INSTANCE_ID` in the wallet start command below.

### Build and Run the Wallet

```bash
git clone https://github.com/TEENet-io/teenet-wallet.git
cd teenet-wallet
git submodule update --init --recursive
make frontend
make build
APP_INSTANCE_ID=<mock-app-instance-id> DATA_DIR=./data SERVICE_URL=http://127.0.0.1:8089 ./teenet-wallet
```

The wallet service listens on `0.0.0.0:18080` by default. Open `http://localhost:18080`, enter your email, submit the 6-digit verification code (defaults to `999999` in mock mode — see [`DEV_FIXED_CODE`](docs/en/configuration.md)), register a Passkey, and generate an API key.
For local development against `teenet-sdk/mock-server`, `DATA_DIR=./data` keeps the SQLite database in a writable local directory.

### Verify the Service

```bash
curl -s http://localhost:18080/api/health
```

Expected response:

```json
{"status":"ok","service":"teenet-wallet","db":true}
```

For the complete local setup, including starting `teenet-sdk/mock-server` and creating your first wallet, see the [full Quick Start guide](https://teenet-io.github.io/teenet-wallet/#/en/quick-start).

### Docker

```bash
make docker
docker run -p 18080:18080 \
  --add-host=host.docker.internal:host-gateway \
  -e APP_INSTANCE_ID=<mock-app-instance-id> \
  -e SERVICE_URL=http://host.docker.internal:8089 \
  -v wallet-data:/data \
  teenet-wallet:latest
```

The Docker image still requires a reachable TEENet service endpoint via `SERVICE_URL`. On Linux, `--add-host=host.docker.internal:host-gateway` is required (Docker Desktop on macOS/Windows adds this host automatically; the flag is harmless there). `make run` on the mock already binds to `0.0.0.0` by default so the container can reach it via the host gateway.

## Documentation

Full documentation is available **[HERE](https://teenet-io.github.io/teenet-wallet/)**.

## TEENet Platform

This wallet is one application built on [TEENet](https://teenet.io) -- a platform that provides hardware-isolated runtime and managed key custody for any application that needs to protect secrets, from AI agent wallets to autonomous trading systems to cross-chain bridges. TEENet is currently in Developer Preview. [Platform docs](https://teenet-io.github.io/#/) &middot; [TEENet SDK](https://github.com/TEENet-io/teenet-sdk)

## Contributing

We welcome contributions! Whether it's bug fixes, new chain support, or documentation improvements -- see [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## Disclaimer

This software is experimental and provided "as is" without warranty. Cryptocurrency transactions are final and irreversible. TEENet is not responsible for any loss of digital assets. See [DISCLAIMER.md](DISCLAIMER.md) for full details.

## License

Copyright (C) 2026 TEENet Technology (Hong Kong) Limited.

GPL-3.0 -- see [LICENSE](LICENSE)
