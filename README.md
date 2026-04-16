# TEENet Wallet

[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8.svg?logo=go&logoColor=white)](https://go.dev)
[![Developer Preview](https://img.shields.io/badge/Status-Developer_Preview-orange.svg)]()

A wallet your AI agent can use -- without putting your assets at risk. Your agent handles routine tasks like balances, transfers, and activity checks, while you set the rules: transfer limits, contract allowlists, and approval requirements. When an action exceeds your rules, you step in with a single Passkey confirmation.

Open source. Hardware-enforced rules. Passkey approval.

> **Disclaimer:** This software manages real cryptocurrency assets. Use at your own risk. The authors are not responsible for any loss of funds. Always test thoroughly on testnets before using with real assets.

## How It Works

```
AI Agent / App                       User (Browser)
    |  (API Key)                         |  (Passkey Session)
    v                                    v
+--------------------------------------------------+
|                 TEENet Platform                   |
|  +--------------------------------------------+  |
|  |  TEENet Wallet (application)               |  |
|  |  - Builds transactions                     |  |
|  |  - Enforces contract whitelist              |  |
|  |  - Manages approval policies + daily limits |  |
|  |  - Routes to approval queue or direct sign  |  |
|  +--------------------------------------------+  |
|                       |                           |
|  +--------------------------------------------+  |
|  |  Key Custody & Signing                     |  |
|  |  - Keys sharded across TEE nodes           |  |
|  |  - Threshold signing (no full key anywhere)|  |
|  +--------------------------------------------+  |
|                                                   |
|  Hardware TEE Layer (Intel TDX / AMD SEV)         |
+--------------------------------------------------+
```

Private keys are sharded across independent TEE nodes, never exported, and the hardware guarantees that the running code cannot be modified or bypassed.

## Features

- **Multi-chain** -- Ethereum, Solana, Optimism, Arbitrum, Base, Polygon, BNB Chain, Avalanche, and custom EVM chains
- **Dual auth** -- API keys for AI agents and automation; Passkeys (WebAuthn) for human approval of high-value operations
- **Smart contract security** -- contract whitelist + Passkey approval for all contract operations via API key
- **Approval policies** -- USD-denominated thresholds, daily spend limits, real-time price feeds
- **Address book** -- nickname-based transfers with per-chain validation
- **Agent-ready** -- [OpenClaw](https://openclaw.ai) plugin and skill integration, idempotent transfers, SSE event stream, audit logging

See the [full documentation](https://teenet-io.github.io/teenet-wallet/#/en/overview) for details.

## Quick Start

### Prerequisites

- Go 1.25+
- Node.js + npm (required for `make frontend`)
- SQLite3 development headers (`apt-get install libsqlite3-dev` on Debian/Ubuntu)
- A TEENet service endpoint (`teenet-sdk/mock-server` for local development)

### Start the Mock Service

```bash
git clone https://github.com/TEENet-io/teenet-sdk.git
cd teenet-sdk/mock-server
make run
```

The mock service listens on `http://127.0.0.1:8089` by default and `make run` sets the localhost Passkey defaults it needs for local development. Leave it running in a separate terminal.
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

The wallet service listens on `0.0.0.0:8080` by default. Open `http://localhost:8080`, register a Passkey, and generate an API key.
For local development against `teenet-sdk/mock-server`, `DATA_DIR=./data` keeps the SQLite database in a writable local directory.

### Verify the Service

```bash
curl -s http://localhost:8080/api/health
```

Expected response:

```json
{"status":"ok","service":"teenet-wallet","db":true}
```

For the complete local setup, including starting `teenet-sdk/mock-server` and creating your first wallet, see the [full Quick Start guide](https://teenet-io.github.io/teenet-wallet/#/en/quick-start).

### Docker

```bash
make docker
docker run -p 8080:8080 \
  -e APP_INSTANCE_ID=<mock-app-instance-id> \
  -e SERVICE_URL=http://host.docker.internal:8089 \
  -v wallet-data:/data \
  teenet-wallet:latest
```

The Docker image still requires a reachable TEENet service endpoint via `SERVICE_URL`.
On Linux, `host.docker.internal` may require `--add-host=host.docker.internal:host-gateway` or an equivalent host-networking setup.

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
