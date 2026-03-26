[wallet-url]: https://test.teenet.io/instance/f8e649535e1d2838ae2817992f946d6a

# Security & FAQ

---

## Security

TEENet Wallet is built with multiple layers of protection:

- **Your private keys are split across TEE hardware nodes.** They are generated and stored using threshold cryptography. No single node, server, or person ever holds your complete private key. Multiple secure hardware nodes must cooperate to sign a transaction.

- **OpenClaw never sees your private keys.** OpenClaw sends transaction requests to the wallet service, which coordinates signing across the TEE nodes. The keys never leave the secure hardware.

- **Large transactions need your fingerprint or Face ID.** Any transaction above your spending threshold requires Passkey approval. There is no way for OpenClaw, or anyone with your API key, to bypass this.

- **You control the spending limit.** You decide the threshold. Lower thresholds give you more control. Higher thresholds give OpenClaw more autonomy. Adjust it to your comfort level.

- **Daily limits provide a hard cap.** Even if every individual transaction is below your threshold, the daily limit prevents runaway spending.

- **You can revoke OpenClaw's access at any time.** Go to the Account tab, click Revoke next to the API key, and OpenClaw immediately loses access to your wallets.

- **Everything is logged.** Every action OpenClaw takes is recorded in the History tab. Review it anytime to confirm OpenClaw is doing what you expect.

---

## FAQ

### How do I install the wallet skill on OpenClaw?

Open your OpenClaw chat and send **"Install this skill:"** followed by:

```
https://github.com/TEENet-io/teenet-wallet/blob/master/skill/tee-wallet/SKILL.md
```

OpenClaw will ask you for two settings: TEE_WALLET_API_URL (enter your wallet URL) and TEE_WALLET_API_KEY (paste your API key starting with `ocw_`). Once both are provided, the skill is installed and ready.

### What can OpenClaw do without my approval?

OpenClaw can freely perform the following without requiring your Passkey:

- Send transactions that are below your USD approval threshold.
- Check wallet balances and addresses.
- View transaction history.
- Read contract data (checking balances, viewing prices) without sending transactions.

Everything else requires your approval.

### What always needs my approval?

The following actions always require your Passkey, regardless of spending limits:

- Transactions above your USD approval threshold.
- Transactions that would exceed your daily spending limit.
- All smart contract interactions (swaps, token approvals, DeFi operations).
- Adding a new contract or token to the whitelist.
- Changing or disabling an approval policy.
- Deleting a wallet.
- Generating or revoking API keys.

### Can I use multiple OpenClaw bots?

Yes. Generate a separate API key for each one in the Account tab. Each bot operates independently but shares your wallets and is subject to the same approval policies. If one misbehaves, revoke its specific key without affecting the others.

### What if OpenClaw tries to overspend?

If OpenClaw submits a transaction above your threshold, it is held in a pending state and nothing is sent until you approve it. If the transaction would also exceed your daily limit, it is blocked entirely -- even you cannot approve it until the next day. The daily limit is a hard cap enforced at the infrastructure level.

### How do I stop OpenClaw from using my wallet?

Go to [TEENet Wallet][wallet-url], click the **Account** tab, find the API key OpenClaw is using, and click **Revoke**. Authenticate with your Passkey. OpenClaw immediately loses all access to your wallets. If you want to reconnect later, generate a new API key and provide it to OpenClaw.

### What is a Passkey?

A Passkey is a modern replacement for passwords. Instead of typing a password, you prove your identity using your fingerprint, Face ID, or a physical security key. Your biometric data never leaves your device -- the wallet only receives a cryptographic proof that you authenticated successfully. Passkeys cannot be phished, guessed, or stolen in a data breach. Think of it like unlocking your phone, but for your wallet.

### What if I lose my device?

Your wallets and funds are safe. They remain in the TEE infrastructure regardless of what happens to your device. You just need to regain access to your account.

Many Passkey systems (like Apple's iCloud Keychain or Google's Password Manager) sync your Passkeys across devices. If you lose your phone but have a laptop signed into the same account, you can still log in. Some platforms also let you scan a QR code on a new device to authenticate using a nearby device.

If you have completely lost access to all devices with your Passkey, contact the wallet administrator for account recovery options.

**Best practice:** Make sure your Passkeys are backed up through your device ecosystem (Apple, Google, or a hardware security key stored in a safe place).

### What chains are supported?

**Mainnets:**

| Chain | Currency | Type |
|-------|----------|------|
| Ethereum | ETH | EVM |
| Optimism | ETH | EVM Layer 2 |
| Solana | SOL | Solana |

**Testnets (free test tokens):**

| Chain | Currency | Type |
|-------|----------|------|
| Sepolia | ETH | EVM |
| Holesky | ETH | EVM |
| Base Sepolia | ETH | EVM Layer 2 |
| BSC Testnet | tBNB | EVM |
| Solana Devnet | SOL | Solana |

Any EVM-compatible chain (Polygon, Arbitrum, Avalanche, etc.) can be added at runtime. Use `/chains` to see the current list, or ask your wallet administrator to add one.

---

## Need Help?

If you run into any issues not covered here, contact your wallet administrator. When reporting a problem, it helps to include:

- What you were trying to do.
- What you expected to happen.
- What actually happened (including any error messages).
- Which browser and device you are using.

---
[Previous: Approvals & Web UI](user-approvals.md)
