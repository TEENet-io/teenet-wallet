# Installation & Setup

Complete installation reference for teenet-wallet. For the fastest path, see [Quick Start](quick-start.md).

---

## Supported platforms

| Platform | Notes |
|----------|-------|
| Linux (Debian/Ubuntu) | Primary target |
| Linux (Alpine) | Requires extra packages for CGo |
| macOS | Requires Xcode Command Line Tools |
| Docker | Multi-stage build included |

---

## Go version

Go **1.24+** is required. CGo must be enabled (`CGO_ENABLED=1`, which is the default) because the SQLite driver is a C library.

---

## Dependencies

SQLite3 development headers are the only external dependency.

```bash
# Debian / Ubuntu
sudo apt-get install libsqlite3-dev

# Alpine
apk add sqlite-dev gcc musl-dev

# macOS (included with Xcode Command Line Tools)
xcode-select --install
```

---

## Build from source

```bash
git clone https://github.com/TEENet-io/teenet-wallet.git
cd teenet-wallet
make build
```

The binary is output to `./teenet-wallet` in the project root.

To build the frontend as well:

```bash
git submodule update --init
make frontend
```

---

## Docker

```bash
make docker
docker run -p 8080:8080 \
  -e SERVICE_URL=http://host.docker.internal:8089 \
  -v wallet-data:/data \
  teenet-wallet:latest
```

The image uses a multi-stage build. `host.docker.internal` routes to the host machine so the container can reach a mock service running outside Docker.

---

## Mock service

The mock service stands in for the TEENet service during development. It implements the full consensus HTTP API with real cryptographic signing, so the wallet behaves as if talking to production.

```bash
git clone https://github.com/TEENet-io/teenet-sdk.git
cd teenet-sdk/mock-server
go build && ./mock-server
```

The mock service listens on `127.0.0.1:8089` by default.

**Custom port and bind address:**

```bash
MOCK_SERVER_PORT=9090 MOCK_SERVER_BIND=0.0.0.0 ./mock-server
```

If you change the port, update `SERVICE_URL` to match when starting the wallet.

> The mock service is in-memory only -- state resets on restart. Do not use in production.

---

## Environment variables

The most important variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVICE_URL` | `http://localhost:8089` | TEENet service endpoint |
| `DATA_DIR` | `/data` | Directory for the SQLite database file (`wallet.db`) |
| `BASE_URL` | `http://localhost:<PORT>` | Public-facing URL used in approval links |
| `FRONTEND_URL` | _(empty)_ | Allowed CORS origin for the web UI |

See [Configuration](configuration.md) for the full environment variable reference.

---

## chains.json

Chain definitions (name, RPC URL, chain ID, curve, protocol) live in `chains.json` at the project root. Additional EVM chains can be added at runtime via the API.

See [chains.json Schema](chains-schema.md) for the full field reference.

---

## Frontend submodule

The web UI is a single-file SPA stored in a git submodule. To initialize it:

```bash
git submodule update --init
make frontend
```

Frontend files must be in the `./frontend/` directory for the server to serve them.

---

## Verify the installation

**Health check:**

```bash
curl -s http://localhost:8080/api/health
```

**Create a user:** Open [http://localhost:8080](http://localhost:8080) and complete the Passkey registration flow.

**Create a wallet:** Generate an API key from Settings, then create a wallet via the API.

See [Quick Start](quick-start.md) for the full step-by-step walkthrough.

---
[Previous: Quick Start](quick-start.md) | [Next: Troubleshooting](troubleshooting.md)
