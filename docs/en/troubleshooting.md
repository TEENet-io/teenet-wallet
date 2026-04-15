# Troubleshooting

Common issues and how to fix them.

---

## CGo / SQLite build failures

**Problem:** Build fails with errors about missing SQLite symbols or C compiler issues.

**Cause:** Missing SQLite3 development headers, wrong Go version, or CGo disabled.

**Fix:**

- Install the headers for your platform:
  ```bash
  # Debian / Ubuntu
  sudo apt-get install libsqlite3-dev

  # Alpine
  apk add sqlite-dev gcc musl-dev
  ```
- Verify Go version is 1.24+: `go version`
- Verify CGo is enabled: `go env CGO_ENABLED` should print `1`. If it prints `0`, run with `CGO_ENABLED=1 make build`. CGo is enabled by default, but some environments override it.

---

## Connection refused on :8089

**Problem:** The wallet logs connection errors to port 8089.

**Cause:** The mock service is not running, or it is running on a different port.

**Fix:**

- Start the mock service: `cd teenet-sdk/mock-server && go build && ./mock-server`
- If you used a custom port (`MOCK_SERVER_PORT=9090`), update the wallet's `SERVICE_URL` to match:
  ```bash
  SERVICE_URL=http://127.0.0.1:9090 ./teenet-wallet
  ```

---

## Invalid API key

**Problem:** API requests return `401 Unauthorized` with an "invalid API key" error.

**Cause:** The API key was copied incorrectly, or it has been revoked. API keys are shown only once when generated.

**Fix:** Generate a new API key from the web UI under **Settings**.

---

## Inspecting the SQLite database

When debugging, you can query the database directly:

```bash
sqlite3 /data/wallet.db
.tables
SELECT id, chain, address, status FROM wallets;
SELECT * FROM approval_requests WHERE status = 'pending';
SELECT action, created_at FROM audit_logs ORDER BY created_at DESC LIMIT 10;
```

If you changed `DATA_DIR`, replace `/data` with your configured directory.

---

## Rate limit hit during development

**Problem:** API requests return `429 Too Many Requests`.

**Cause:** Default rate limits are conservative: 200 requests/min per API key, 5 wallet creates/min.

**Fix:** Override the limits with environment variables:

```bash
API_KEY_RATE_LIMIT=1000 WALLET_CREATE_RATE_LIMIT=50 ./teenet-wallet
```

---

## Frontend not loading

**Problem:** Opening http://localhost:8080 shows a blank page or 404.

**Cause:** The frontend submodule has not been initialized, or the built files are missing.

**Fix:**

```bash
git submodule update --init
make frontend
```

Frontend files must be in the `./frontend/` directory. Restart the wallet after building.
