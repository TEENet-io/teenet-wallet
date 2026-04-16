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

# macOS (included with Xcode Command Line Tools)
xcode-select --install

# Alpine
apk add sqlite-dev gcc musl-dev
```

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
..."msg":"server starting","addr":"0.0.0.0:8080"...
```

---

## 3. Verify it's running

```bash
curl -s http://localhost:8080/api/health
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

Open the web UI at [http://localhost:8080](http://localhost:8080). Register with a Passkey, then go to **Settings** and generate an API key. The key starts with `ocw_` and is shown only once -- save it.

Then create a wallet:

```bash
export API_KEY="ocw_..."
curl -s -X POST http://localhost:8080/api/wallets \
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
    "chain": "sepolia",
    "address": "0x1234abcd5678ef90...",
    "label": "Test Wallet",
    "status": "ready"
  }
}
```

---

## 5. Run the test suite

```bash
make test
```

---

## You're running

- [Installation & Setup](installation.md) -- full build options, Docker, environment variables
- [Concepts](architecture-overview.md) -- understand the architecture
