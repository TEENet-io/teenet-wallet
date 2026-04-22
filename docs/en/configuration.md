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
| `API_KEY_RATE_LIMIT` | `100` | Maximum requests per minute per API key |
| `WALLET_CREATE_RATE_LIMIT` | `5` | Maximum wallet creations per minute per key (TEE DKG is expensive) |
| `REGISTRATION_RATE_LIMIT` | `10` | Maximum registration attempts per minute per IP |
| `RPC_RATE_LIMIT` | `50` | Per-user cap on every endpoint that hits upstream RPC — reads (`/call-read`, `/balance`) and fund-moving ops (`/transfer`, `/contract-call`, `/approve-token`, `/revoke-approval`, `/wrap-sol`, `/unwrap-sol`) share this bucket. |
| `APPROVAL_EXPIRY_MINUTES` | `1440` | Minutes before a pending approval request expires |
| `MAX_WALLETS_PER_USER` | `10` | Maximum wallets a single user can create |
| `MAX_API_KEYS_PER_USER` | `10` | Maximum API keys per user |
| `MAX_USERS` | `500` | Maximum registered users, 0 = unlimited |
| `SMTP_HOST` | _(empty)_ | SMTP server for sending email verification codes. When empty, the wallet runs in mock-email mode and logs codes to stdout instead of sending them. `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM` configure the sender. |
| `SMTP_PASSWORD_KEY` | _(empty)_ | **Preferred for production.** Name of a TEE-backed API key whose value is the SMTP password. When set, the wallet calls `sdk.GetAPIKey(name)` at startup and uses the returned value, so the password never appears in `docker inspect` or process env. Takes precedence over `SMTP_PASSWORD`. Startup aborts if the key is missing or unreachable. The key must be created for this `APP_INSTANCE_ID` before deploy — either through the TEENet management console (API keys tab on the instance) or programmatically via `sdk.CreateAPIKey` (see [`teenet-sdk/go/examples/apikey`](https://github.com/TEENet-io/teenet-sdk) / `examples/admin`). |
| `DEV_FIXED_CODE` | `999999` (mock mode only) | **Dev-only.** In mock mode (`SMTP_HOST` unset), every email verification code equals this value instead of a random 6 digits — lets you register locally without SMTP or log scraping. Override to any other 6-digit string if `999999` collides with your tests. Ignored when SMTP is configured. Never set in production. |
| `QUICKNODE_ENDPOINT` | _(empty)_ | QuickNode endpoint subdomain (e.g. `wispy-wiser-road`). When set, every chain in `chains.json` with a non-empty `quicknode_network` field has its `rpc_url` rewritten at startup to `https://{endpoint}.{network}.quiknode.pro/{token}/`. Unset → chains keep their public fallback RPC. |
| `QUICKNODE_TOKEN` | _(empty)_ | QuickNode access token (the path-segment part of the URL). Visible in `docker inspect` — use `QUICKNODE_TOKEN_KEY` in production instead. |
| `QUICKNODE_TOKEN_KEY` | _(empty)_ | **Preferred for production.** Name of a TEE-backed API key whose value is the QuickNode token. Loaded via `sdk.GetAPIKey(name)` at startup, so the token never touches docker env or process env. Takes precedence over `QUICKNODE_TOKEN`. Startup aborts if the key is missing or unreachable. Create the key through the TEENet management console (API keys tab on the instance) or programmatically via `sdk.CreateAPIKey`. |
| `PRICE_CACHE_TTL` | `120` (Docker image) / `60` (code default) | CoinGecko USD-price cache TTL, in seconds. Also controls how often the background refresher polls CoinGecko. The image ships with `120` because CoinGecko's free tier rate-limits at roughly one call per two minutes per IP — polling faster just produces a stream of benign 429 warnings. Lower to `60` only if you have a paid CoinGecko plan. |

RPC URLs for each blockchain are defined in `chains.json`, not as individual environment variables. To add or remove chains, edit `chains.json` and restart the service — the file is loaded once at startup.

### QuickNode RPC overrides

Public RPCs like `publicnode.com` rate-limit aggressively and occasionally go down. To route a chain through [QuickNode](https://www.quicknode.com/) instead:

1. Create a QuickNode endpoint. Enable the networks you need (one endpoint + one token can serve multiple chains via the "Multi-Chain" option).
2. Add `quicknode_network` to the chain entry in `chains.json`. The value is the network slug from your QuickNode dashboard URL — e.g. `https://wispy-wiser-road.ethereum-sepolia.quiknode.pro/...` → `"quicknode_network": "ethereum-sepolia"`. Ethereum Mainnet is a special case: QuickNode omits the subdomain entirely, so use `"quicknode_network": "-"`. For chains that need a path suffix after the token (Avalanche C-Chain's `/ext/bc/C/rpc`), also set `"quicknode_path": "/ext/bc/C/rpc"`.
3. Set `QUICKNODE_ENDPOINT` (subdomain) and either `QUICKNODE_TOKEN` (dev) or `QUICKNODE_TOKEN_KEY` (prod) on the wallet container.

At startup the wallet rewrites `rpc_url` for each matching chain. Chains without `quicknode_network` are untouched. The bundled `chains.json` ships with slugs pre-filled for every default chain.

**Runtime fallback:** when `rpc_url` is overridden, the original `chains.json` URL (publicnode et al.) is registered as a fallback. If a QuickNode request fails with a transport or HTTP error (DNS failure, timeout, 5xx, 429), the RPC layer transparently retries on the fallback for that single call — no impact on subsequent calls, which still prefer QuickNode. Application-level JSON-RPC errors (`execution reverted`, `nonce too low`) short-circuit immediately and do not trigger the fallback, since a different provider would return the same result. Host (not full URL, since it contains the token) is logged on each fallback.
