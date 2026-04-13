[wallet-url]: https://test.teenet.io/instance/wallet/

# Web UI & Approvals

The web UI at [TEENet Wallet][wallet-url] is your dashboard for managing wallets, reviewing approvals, and monitoring activity. Your agent handles day-to-day operations — the web UI is for oversight and security configuration.

The top navigation has three tabs: **Wallets**, **Approvals**, and **Activity**. The gear icon opens **Settings**.

---

## Wallets

The Wallets tab shows all your wallets. Each card displays the wallet name, chain, address, balance, and status.

**Creating a wallet:** Click **Create New Wallet** at the top, select a chain from the dropdown, optionally add a label, and click **Create Wallet**. EVM wallets take 1-2 minutes (distributed key generation across TEE nodes). Solana wallets are instant.

**Wallet detail page:** Click on any wallet to see its detail view:

- **Balance** — current balance with USD estimate
- **Threshold tab** — configure your spend policy:
  - **Approval threshold (USD)** — transactions above this amount require your Passkey. Use the slider or type a value.
  - **Daily limit (USD)** — hard cap on total daily spending. The progress bar shows how much you've spent today.
  - Click **Save policy** to apply (requires Passkey approval).
- **Contract tab** (or **Program** for Solana) — view and manage whitelisted contracts. Only whitelisted contracts can be called. Adding or removing contracts requires Passkey approval.
- **Delete Wallet** — permanently removes the wallet (requires Passkey).

---

## Approvals

The Approvals tab shows all pending transactions that need your action. Each pending item displays:

- The amount and currency (e.g., **0.001 ETH**)
- The recipient address
- A **PENDING** status badge
- A **countdown timer** showing time remaining before expiry

**To approve:** Click on a pending item, review the details, and tap **Approve**. Authenticate with your fingerprint or Face ID. The transaction is then signed and sent to the blockchain.

**To reject:** Tap **Reject** if something looks wrong. The transaction is cancelled.

**Expiry:** Pending approvals expire after **24 hours**. If not acted on, the transaction is automatically cancelled. Ask your agent to resubmit if you still want to proceed.

**How you get here:** Your agent will send you a direct link to the approval screen when a transaction needs your action. You can also check the Approvals tab directly at any time.

---

## Activity

The Activity tab shows a complete audit trail of every action on your account, grouped by date. Each entry shows the action type, relevant details, and timestamp. Entry types include:

- **Transfer** — with status badges: **AUTO** (went through automatically) or **PENDING** (awaiting approval)
- **Approve** — you approved a pending transaction
- **Policy Update** — spending threshold or daily limit changed
- **Contract Call** — smart contract interaction
- **Login** — account login event

Use the **All Actions** dropdown to filter by action type. Click **Refresh** to load the latest entries.

---

## Settings

Click the gear icon to open **Security & API Control**:

- **API Keys** — generate new keys (enter a label and click **Generate**), view existing keys with their creation date and status, or revoke a key by clicking the revoke icon. Each key shows a masked preview (e.g., `ocw_84b888...`).
- **Address Book** — save addresses with nicknames for quick transfers. Click **Add Address** to create a new entry.
- **Danger Zone** — **Logout Global** signs out all sessions. **Delete Account** permanently removes your account and all data.

---
[Previous: What You Can Do](/en/user-commands.md) | [Next: FAQ](/en/user-faq.md)
