[wallet-url]: https://test.teenet.io/instance/f8e649535e1d2838ae2817992f946d6a

# Approvals & Web UI

When OpenClaw submits a transaction that exceeds your spending threshold, the transaction is held in a pending state. Nothing is sent to the blockchain until you act.

---

## Approving Transactions

### How it works

1. OpenClaw tells you in chat that a transaction needs your approval. It includes a direct link to the approval screen.

2. Tap the link. It opens in your browser and takes you straight to that specific transaction.

3. Review the details: the recipient address, the amount, the currency, and the estimated USD value.

4. Tap **Approve** and authenticate with your fingerprint or Face ID. Or tap **Reject** if something looks wrong.

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

- **Wallets tab** -- View all your wallets, their addresses, and balances. Expand a wallet to see its full details, approval policy, and contract whitelist.

- **Approvals tab** -- Review and approve or reject pending transactions that exceed your spending threshold.

- **Account tab** -- Generate new API keys, view existing keys, and revoke keys you no longer need.

- **History tab** -- View a complete audit trail of every action: transfers, approvals, wallet creations, policy changes, whitelist updates, and more. Use the filter to narrow by action type.

- **Approval policies** -- Expand any wallet and click the Policy tab to set or change spending thresholds and daily limits.

- **Contract whitelist** -- Expand any wallet and click the Whitelist tab to see whitelisted contracts. You can delete contracts from the whitelist here. Adding new contracts is done through OpenClaw (and requires your Passkey approval).

Day-to-day operations like sending crypto and interacting with contracts are handled through OpenClaw. The Web UI is for oversight, approvals, and security configuration.

---
[Previous: Talking to OpenClaw](user-commands.md) | [Next: Security & FAQ](user-faq.md)
