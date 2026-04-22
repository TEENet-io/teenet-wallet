# Quick Start

Get from zero to a running wallet in 5 minutes. Every step is copy-paste.

---

## Prerequisites

- **Go 1.25+**
- **SQLite3 development headers**

Install SQLite headers for your platform:

```bash
# Debian / Ubuntu
sudo apt-get install libsqlite3-dev

# RHEL / Fedora / Rocky / AlmaLinux / Alibaba Cloud Linux
sudo dnf install sqlite-devel

# macOS (included with Xcode Command Line Tools)
xcode-select --install

# Alpine
apk add sqlite-dev gcc musl-dev
```

---

## TL;DR — one-command setup

Once the prerequisites are in place, the whole flow below (clone the SDK, build both services, start mock + wallet with matching ports and origin, health-check) collapses into:

```bash
./scripts/dev.sh up
```

Useful overrides: `MOCK_PORT=` / `WALLET_PORT=` / `APP_INSTANCE_ID=`, or `AUTO_PORT=1` to skip over busy ports. `down` / `status` / `logs` round out the script. Runtime state (PIDs, logs, SQLite) lives in `.dev/`.

Skip to **Step 3** to verify, then **Step 4** to register and create your first wallet. The rest of this page walks the same steps by hand in case you want to understand what's happening or run them piece by piece.

---

## 1. Start the mock service

The mock service stands in for the TEENet service during development. Open a terminal:

```bash
git clone https://github.com/TEENet-io/teenet-sdk.git
cd teenet-sdk/mock-server
make run
```

Expected output:

```
Mock server starting on port 8089
Available test App IDs: ...
```

Use one of the printed app IDs as `APP_INSTANCE_ID` when starting the wallet.

Leave this running.

> **Non-default wallet port?** WebAuthn requires an exact `scheme://host:port` match, so if you override `PORT` in Step 2, start the mock with a matching origin: `PASSKEY_RP_ORIGIN=http://localhost:<port> make run`. The default (`:18080`) matches the wallet's default.
>
> **Port 8089 already in use?** Override with `MOCK_SERVER_PORT=<port>` and set `SERVICE_URL` in Step 2 accordingly.

---

## 2. Build and run the wallet

Open a **new terminal**:

```bash
git clone https://github.com/TEENet-io/teenet-wallet.git
cd teenet-wallet
git submodule update --init --recursive
make frontend
make build
APP_INSTANCE_ID=<mock-app-instance-id> DATA_DIR=./data SERVICE_URL=http://127.0.0.1:8089 ./teenet-wallet
```

`DATA_DIR=./data` keeps the SQLite database in a writable project directory.

Expected output:

```
..."msg":"server starting","addr":"0.0.0.0:18080"...
```

> **Port 18080 already in use?** Set `PORT=<port>` on the wallet command line, and start the mock service in Step 1 with a matching `PASSKEY_RP_ORIGIN=http://localhost:<port>`.

---

## 3. Verify it's running

```bash
curl -s http://localhost:18080/api/health
```

Expected response:

```json
{
  "db": true,
  "service": "teenet-wallet",
  "status": "ok"
}
```

---

## 4. Create your first wallet

Open the web UI at [http://localhost:18080](http://localhost:18080) and register:

1. Enter your email and click **Send code**
2. Enter the 6-digit verification code. **In mock mode (no SMTP) the code is always `999999`** by default, so you don't have to check any inbox (override via [`DEV_FIXED_CODE`](configuration.md); real email is sent only when `SMTP_HOST` is set)
3. Register with a Passkey

After registration, go to **Settings** and generate an API key. The key starts with `ocw_` and is shown only once -- save it.

Then create a wallet:

```bash
export API_KEY="ocw_..."
curl -s -X POST http://localhost:18080/api/wallets \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"chain": "sepolia", "label": "Test Wallet"}'
```

Expected response:

```json
{
  "success": true,
  "wallet": {
    "id": "8a2fbc16-faf4-451a-be34-9fc5c49cde00",
    "user_id": 1,
    "chain": "sepolia",
    "key_name": "wallet-8a2fbc16...",
    "public_key": "03abcd...",
    "address": "0x1234abcd5678ef90...",
    "label": "Test Wallet",
    "curve": "secp256k1",
    "protocol": "ecdsa",
    "status": "ready",
    "created_at": "2026-04-22T10:30:00Z"
  }
}
```

---

## 5. Run the test suite

```bash
make test
```

---

## 6. Connect a local OpenClaw agent (optional)

With the API key from Step 4 in hand, you can exercise the full agent flow by pointing a local OpenClaw instance at your wallet.

In an OpenClaw chat, install the skill:

```
Install this skill: https://github.com/TEENet-io/teenet-wallet/blob/main/skill/teenet-wallet/SKILL.md
```

When prompted, set:

- **TEENET_WALLET_API_URL** -- `http://localhost:18080` (or your machine's LAN IP if OpenClaw runs on a different host)
- **TEENET_WALLET_API_KEY** -- the `ocw_...` key from Step 4

Then ask the agent to run a quick end-to-end check:

```
/test
```

This exercises balance checks, the testnet faucet, transfers, approval thresholds, and the contract whitelist against your local wallet.

---

## You're running

- [Installation & Setup](installation.md) -- full build options, Docker, environment variables
- [Concepts](architecture-overview.md) -- understand the architecture
