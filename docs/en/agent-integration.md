# AI Agent Integration

TEENet Wallet is designed to serve as the custody layer for AI agents. Two integration paths are supported:

1. **Skill-based (REST)** -- `skill/tee-wallet/SKILL.md` describes every operation with curl examples. The agent reads the skill and makes HTTP calls directly. Works with any agent platform that supports skills.
2. **OpenClaw plugin (native tools)** -- `plugin/` contains a first-class [OpenClaw](https://openclaw.io) plugin written in TypeScript. Each wallet operation is exposed as a strongly-typed tool, and an SSE approval watcher lets the agent continue automatically after the user approves via Passkey.

### How It Works

1. The agent platform (e.g., OpenClaw) provides two environment variables: `TEE_WALLET_API_URL` (the wallet service URL) and `TEE_WALLET_API_KEY` (an `ocw_` API key).
2. The AI agent reads the skill definition, which describes all available operations with curl examples.
3. When a user asks the agent to "send 0.1 ETH to 0x...", the agent calls the `/transfer` endpoint.
4. If the transfer is below the policy threshold, it completes immediately. If above, the agent shows the user an approval link and polls until approved.

### Best Practices for Agent Integration

**Automatic wallet selection.** Never ask the user for a wallet ID. Call `GET /api/wallets`, find wallets matching the required chain, and select automatically if there is only one match. If there are multiple matches, present a numbered list.

**No chat confirmation for transfers.** The backend approval policy is the safety net. Small amounts go through directly; large amounts trigger hardware Passkey approval automatically. Do not add an extra "are you sure?" step in the agent.

**Always include the `token` field for token transfers.** Omitting it sends native ETH/SOL instead, which is an irreversible mistake.

**Check the whitelist before token operations.** Call `GET /api/wallets/:id/contracts` before sending tokens. If the contract is not whitelisted, propose adding it (which creates a pending approval).

**Poll approvals with countdown.** When waiting for Passkey approval, poll `GET /api/approvals/:id` every 15 seconds and show the remaining time. Stop after 25 minutes.

**Use `amount_usd` for contract calls.** When calling `/contract-call` for operations that transfer value, always include the approximate USD value so threshold and daily-limit policies are enforced.

**Fetch fresh wallet lists.** Before showing balances or account-wide views, always re-fetch `GET /api/wallets` to ensure the list is current.

**Include explorer links.** After every successful transaction, provide a block explorer link so the user can verify on-chain.

---

## OpenClaw Plugin (`plugin/`)

For users running [OpenClaw](https://openclaw.io) >= `2026.3.24-beta.2`, the `plugin/` directory ships a native TypeScript plugin that registers wallet operations as first-class tools. Unlike the skill-based flow (where the agent issues raw HTTP calls), the plugin provides strongly-typed tool schemas and a live SSE event stream so the agent can react to Passkey approvals without polling.

### Installation

```bash
openclaw plugins install "/path/to/teenet-wallet/plugin"
openclaw config set plugins.entries.teenet-wallet.config.apiUrl "https://your-wallet-instance/"
openclaw config set plugins.entries.teenet-wallet.config.apiKey "ocw_your_api_key"
openclaw config set plugins.entries.teenet-wallet.enabled true
openclaw gateway restart
openclaw plugins inspect teenet-wallet   # expect Status: loaded
```

Configuration schema (from `openclaw.plugin.json`):

| Parameter | Required | Description |
|-----------|----------|-------------|
| `apiUrl` | Yes | Wallet backend URL (e.g. `https://test.teenet.io/instance/xxx/`) |
| `apiKey` | Yes | API key with `ocw_` prefix |

> **Watch out for `tools.profile`.** The plugin requires the `full` profile (the default). If it's set to `coding`, `messaging`, or `minimal`, tools are silently blocked with no error. Check with `openclaw config get tools.profile` and clear it via `openclaw config unset tools.profile` if needed.

### Available Tools

All tool names are prefixed with `teenet_wallet_`:

| Category | Tools |
|----------|-------|
| Wallet | `create`, `list`, `get`, `rename`, `balance` |
| Transfer | `transfer`, `wrap_sol`, `unwrap_sol` |
| Contracts | `list_contracts`, `add_contract`, `update_contract`, `contract_call`, `approve_token`, `revoke_approval` |
| Policy | `get_policy`, `set_policy`, `daily_spent` |
| Address Book | `list_contacts`, `add_contact`, `update_contact` |
| Approvals | `pending_approvals`, `check_approval` |
| Utility | `list_chains`, `health`, `faucet`, `audit_logs`, `prices`, `get_pubkey` |

### Approval Flow with SSE

```
User (chat)  →  Agent  →  Plugin tool  →  Wallet backend
                 ↑                             |
            subagent.run()                pending_approval
            (deliver=true)                     ↓
                 ↑                        SSE event stream
                 └── ApprovalWatcher ←─────────┘
```

1. The agent calls a tool like `teenet_wallet_transfer`.
2. If the backend returns `pending_approval`, the agent sends the user an approval link.
3. The user verifies with Passkey (fingerprint / security key).
4. `ApprovalWatcher` receives an SSE event and triggers `subagent.run()` in the original chat -- no polling, no "did you approve yet?".
5. The agent reports the tx hash with an explorer link.

### Security Notes

- **API key never reaches the LLM** -- it's held in plugin config and injected by the HTTP client only.
- **SSE events are user-scoped** -- each user only sees their own approval events.
- **All write paths check approval policies** on the backend; the plugin cannot bypass USD thresholds or daily limits.
- **SSRF protection** on custom chain RPC URLs (private IPs and cloud metadata addresses are blocked backend-side).

---

## Web UI

TEENet Wallet includes a built-in web interface served at the root URL (e.g., [`https://test.teenet.io/instance/wallet`](https://test.teenet.io/instance/wallet)). The web UI provides:

- **Account management:** Passkey registration, login, and session management.
- **Wallet dashboard:** Create, view, rename, and delete wallets. View addresses, balances, and chain information.
- **Transfer interface:** Send native currency and tokens with a visual form.
- **Contract whitelist management:** Add, update, and remove whitelisted contracts with an interactive table.
- **Approval queue:** Review pending approval requests and approve or reject them with hardware Passkey authentication.
- **API key management:** Generate and revoke API keys for programmatic access.
- **Policy configuration:** Set and manage USD-denominated approval thresholds and daily limits.

The web UI is served with a restrictive Content Security Policy and additional security headers (`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`).

---
[Previous: Approval System](/en/approvals.md) | [Next: API Reference](/en/api-overview.md)
