[wallet-url]: https://test.teenet.io/instance/wallet/

# Approvals & Web UI

When OpenClaw submits a transaction that exceeds your spending threshold, the transaction is held in a pending state. Nothing is sent to the blockchain until you act.

---

## Approving Transactions

### How it works

1. OpenClaw tells you in chat that a transaction needs your approval. It includes a direct link to the approval screen.

2. Tap the link. It opens in your browser and takes you straight to that specific transaction.

3. Review the details: the recipient address, the amount, the currency, and the estimated USD value.

4. Tap **Approve** and authenticate with your fingerprint or Face ID. Or tap **Reject** if something looks wrong.

<div align="center"><img src="picture/appqueue.png" alt="Approval detail page" width="360" /></div>

5. If you approve, the transaction is signed and sent to the blockchain. OpenClaw will confirm in chat with a transaction hash and explorer link.

### Using the Web UI directly

You can also go to [TEENet Wallet][wallet-url] and click the **Approvals** tab to see all pending transactions. Each one shows:

- The wallet the transaction is from.
- The recipient address.
- The amount and currency.
- The estimated USD value.
- A countdown timer showing how much time remains before it expires.

### Expiry

Pending approvals expire after **24 hours**. If you do not approve or reject within that window, the transaction is automatically cancelled. If you still want to proceed, ask OpenClaw to submit it again.

---

## Web UI Overview

The Web UI at [TEENet Wallet][wallet-url] is your dashboard for oversight and management. Here is what you can do:

- **Wallets tab** -- View all your wallets, their addresses, and balances. Click on a wallet to see its detail page with spending policy and contract whitelist.

- **Approvals tab** -- Review and approve or reject pending transactions that exceed your spending threshold.

- **Activity tab** -- View a complete audit trail of every action: transfers, approvals, wallet creations, policy changes, whitelist updates, and more. Use the filter to narrow by action type.

- **Settings** (gear icon) -- Generate and manage API keys, manage your address book, and delete your account.

- **Approval policies** -- In a wallet's detail page, select the **Threshold** tab to set or change spending thresholds and daily limits.

- **Contract whitelist** -- In a wallet's detail page, select the **Contract** tab (or **Program** for Solana) to see whitelisted contracts. You can add or remove contracts here (requires Passkey approval).

- **Address Book** -- In Settings, manage saved addresses with nicknames for quick transfers.

Day-to-day operations like sending crypto and interacting with contracts are handled through OpenClaw. The Web UI is for oversight, approvals, and security configuration.

---
[Previous: Talking to OpenClaw](/en/user-commands.md) | [Next: Security & FAQ](/en/user-faq.md)
