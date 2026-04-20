# Configuration

All configuration is via environment variables. No configuration files are required for the wallet service itself (chain definitions live in `chains.json`).

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVICE_URL` | `http://localhost:8089` | URL of the local TEENet service node |
| `HOST` | `0.0.0.0` | Bind address for the HTTP server |
| `PORT` | `18080` | HTTP listen port |
| `DATA_DIR` | `/data` | Directory for the SQLite database file (`wallet.db`) |
| `BASE_URL` | `http://localhost:<PORT>` | Public-facing URL used in approval links and callbacks |
| `FRONTEND_URL` | _(empty)_ | Allowed CORS origin; empty disables CORS headers entirely |
| `CHAINS_FILE` | `./chains.json` | Path to the chain configuration file |
| `APP_INSTANCE_ID` | _(from TEENet)_ | TEENet application instance identifier; usually set automatically |
| `API_KEY_RATE_LIMIT` | `200` | Maximum requests per minute per API key |
| `WALLET_CREATE_RATE_LIMIT` | `5` | Maximum wallet creations per minute per key (TEE DKG is expensive) |
| `REGISTRATION_RATE_LIMIT` | `10` | Maximum registration attempts per minute per IP |
| `APPROVAL_EXPIRY_MINUTES` | `1440` | Minutes before a pending approval request expires |
| `MAX_WALLETS_PER_USER` | `10` | Maximum wallets a single user can create |
| `MAX_API_KEYS_PER_USER` | `10` | Maximum API keys per user |
| `MAX_USERS` | `500` | Maximum registered users, 0 = unlimited |
| `SMTP_HOST` | _(empty)_ | SMTP server for sending email verification codes. When empty, the wallet runs in mock-email mode and logs codes to stdout instead of sending them. `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM` configure the sender. |
| `DEV_FIXED_CODE` | `999999` (mock mode only) | **Dev-only.** In mock mode (`SMTP_HOST` unset), every email verification code equals this value instead of a random 6 digits — lets you register locally without SMTP or log scraping. Override to any other 6-digit string if `999999` collides with your tests. Ignored when SMTP is configured. Never set in production. |

RPC URLs for each blockchain are defined in `chains.json`, not as individual environment variables. Additional EVM chains can be added at runtime via `POST /api/chains` (Passkey required); these are persisted in the database and survive restarts.
