# Installation & Setup

Complete installation reference for teenet-wallet. For the fastest path, see [Quick Start](quick-start.md).

---

## Supported platforms

| Platform | Notes |
|----------|-------|
| Linux (Debian/Ubuntu) | Primary target |
| Linux (RHEL/Fedora/Rocky/AlmaLinux/Alibaba Cloud Linux) | Install `sqlite-devel` via `dnf` |
| Linux (Alpine) | Requires extra packages for CGo |
| macOS | Requires Xcode Command Line Tools |
| Docker | Multi-stage build included |

---

## Go version

Go **1.25+** is required. CGo must be enabled (`CGO_ENABLED=1`, which is the default) because the SQLite driver is a C library.

---

## Dependencies

SQLite3 development headers are the only external dependency.

```bash
# Debian / Ubuntu
sudo apt-get install libsqlite3-dev

# RHEL / Fedora / Rocky / AlmaLinux / Alibaba Cloud Linux
sudo dnf install sqlite-devel

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
git submodule update --init --recursive
make frontend
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
docker run -p 18080:18080 \
  --add-host=host.docker.internal:host-gateway \
  -e APP_INSTANCE_ID=<mock-app-instance-id> \
  -e SERVICE_URL=http://host.docker.internal:8089 \
  -v wallet-data:/data \
  teenet-wallet:latest
```

The image uses a multi-stage build. On Linux, `--add-host=host.docker.internal:host-gateway` is required — without it the container cannot resolve `host.docker.internal`. Docker Desktop (macOS/Windows) adds this host automatically and the flag is harmless there.

`make run` (mock-server) binds to `0.0.0.0` by default so the container can reach it via the host gateway; set `MOCK_SERVER_BIND=127.0.0.1` if you want to restrict the mock to loopback-only.

---

## Mock service

The mock service stands in for the TEENet service during development. It implements the full TEENet service HTTP API with real cryptographic signing, so the wallet behaves as if talking to production.

> **Shortcut:** `./scripts/dev.sh up` in the wallet repo clones the SDK (if missing), builds both services, starts them with matching ports and WebAuthn origin, and health-checks. See the [Quick Start](quick-start.md) for the full menu of subcommands and env overrides. The rest of this section covers running the mock by hand.

```bash
git clone https://github.com/TEENet-io/teenet-sdk.git
cd teenet-sdk/mock-server
make run
```

The mock service listens on `0.0.0.0:8089` by default.

**WebAuthn origin.** The mock server validates Passkey registrations against `PASSKEY_RP_ORIGIN`. `make run`'s default is `http://localhost:18080`, which matches the wallet's default port. If you run the wallet on a different `PORT`, start the mock with a matching origin:

```bash
PASSKEY_RP_ORIGIN=http://localhost:<wallet-port> make run
```

Browser Passkey registration requires an exact `scheme://host:port` match; any mismatch fails with an origin-mismatch error.

**Custom port and bind address:**

```bash
MOCK_SERVER_PORT=9090 MOCK_SERVER_BIND=0.0.0.0 ./mock-server
```

If you change the mock port, update `SERVICE_URL` to match when starting the wallet.

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

For local development against `teenet-sdk/mock-server`, also set `APP_INSTANCE_ID` to one of the app IDs printed by the mock server at startup. When running from source outside Docker, set `DATA_DIR` to a writable local path such as `./data`.

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
curl -s http://localhost:18080/api/health
```

**Create a user:** Open [http://localhost:18080](http://localhost:18080), enter your email → submit the 6-digit code (defaults to `999999` in mock mode — see [`DEV_FIXED_CODE`](configuration.md)) → register with a Passkey.

**Create a wallet:** Generate an API key from Settings, then create a wallet via the API.

See [Quick Start](quick-start.md) for the full step-by-step walkthrough.
