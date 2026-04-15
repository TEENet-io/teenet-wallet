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

See the [full documentation](https://teenet-io.github.io/teenet-wallet/#/en/introduction) for details.

## Quick Start

### Prerequisites

- Go 1.25+
- SQLite3 development headers (`apt-get install libsqlite3-dev` on Debian/Ubuntu)

### Build and Run

```bash
make build
./teenet-wallet
```

The wallet service starts on `http://0.0.0.0:8080` by default.

### Docker

```bash
make docker
docker run -p 8080:8080 \
  -e SERVICE_URL=http://host.docker.internal:8089 \
  -v wallet-data:/data \
  teenet-wallet:latest
```

## Documentation

Full documentation is available at **[teenet-io.github.io/teenet-wallet](https://teenet-io.github.io/teenet-wallet/)**.

## TEENet Platform

This wallet is one application built on [TEENet](https://teenet.io) -- a platform that provides hardware-isolated runtime and managed key custody for any application that needs to protect secrets, from AI agent wallets to autonomous trading systems to cross-chain bridges. TEENet is currently in Developer Preview. [Platform docs](https://teenet-io.github.io/#/) &middot; [TEENet SDK](https://github.com/TEENet-io/teenet-sdk)

## Contributing

We welcome contributions! Whether it's bug fixes, new chain support, or documentation improvements -- see [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## Disclaimer

This software is experimental and provided "as is" without warranty. Cryptocurrency transactions are final and irreversible. TEENet is not responsible for any loss of digital assets. See [DISCLAIMER.md](DISCLAIMER.md) for full details.

## License

Copyright (C) 2026 TEENet Technology (Hong Kong) Limited.

GPL-3.0 -- see [LICENSE](LICENSE)
