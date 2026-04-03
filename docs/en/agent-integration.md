# AI Agent Integration

TEENet Wallet is designed to serve as the custody layer for AI agents. The `skill/tee-wallet/` directory contains an [OpenClaw](https://openclaw.io) skill definition that enables AI agents to manage wallets through natural language.

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
