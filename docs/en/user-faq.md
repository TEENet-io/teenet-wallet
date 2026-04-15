[wallet-url]: https://test.teenet.io/instance/wallet/

# FAQ

---

## Getting Connected

### How do I connect an AI agent?

Generate an API key in the [TEENet Wallet][wallet-url] **Account** tab and provide it to your agent along with the wallet API URL shown on your account page. The agent uses these to call the wallet's REST API on your behalf.

**OpenClaw example:** Open your OpenClaw chat and send:

```
Install this skill: https://github.com/TEENet-io/teenet-wallet/blob/master/skill/tee-wallet/SKILL.md
```

OpenClaw will ask for your API key (starts with `ocw_`) and wallet API URL. Once provided, the skill is installed and ready.

For other agent platforms, consult the platform's documentation on how to configure API credentials.

---

## Agent Permissions

### What can my agent do without approval?

Your agent can freely perform the following without requiring your Passkey:

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

### Can I use multiple agents?

Yes. Generate a separate API key for each agent in the Account tab. Each agent operates independently but shares your wallets and is subject to the same approval policies. If one misbehaves, revoke its specific key without affecting the others.

---

## Safety

### How are my private keys protected?

Your private keys are split across multiple TEE (Trusted Execution Environment) hardware nodes using threshold cryptography. No single node, server, or person ever holds your complete private key. Multiple secure hardware nodes must cooperate to sign a transaction. Your agent never sees your private keys -- it sends transaction requests to the wallet service, which coordinates signing across the TEE nodes. The keys never leave the secure hardware.

### What if my agent tries to overspend?

If your agent submits a transaction above your threshold, it is held in a pending state and nothing is sent until you approve it. If the transaction would also exceed your daily limit, it is blocked entirely -- even you cannot approve it until the next day. The daily limit is a hard cap enforced at the infrastructure level.

### Can I see what my agent has done?

Yes. Every action your agent takes is recorded in the History tab of the [TEENet Wallet][wallet-url] web UI. Review it anytime to confirm your agent is doing what you expect.

### How do I revoke an agent's access?

Go to [TEENet Wallet][wallet-url], click the **Account** tab, find the API key your agent is using, and click **Revoke**. Authenticate with your Passkey. The agent immediately loses all access to your wallets. If you want to reconnect later, generate a new API key and provide it to the agent.

---

## Account

### What is a Passkey?

A Passkey is a modern replacement for passwords. Instead of typing a password, you prove your identity using your fingerprint, Face ID, or a physical security key. Your biometric data never leaves your device -- the wallet only receives a cryptographic proof that you authenticated successfully. Passkeys cannot be phished, guessed, or stolen in a data breach. Think of it like unlocking your phone, but for your wallet.

### What if I lose my device?

Your wallets and funds are safe. They remain in the TEE infrastructure regardless of what happens to your device. You just need to regain access to your account.

Many Passkey systems (like Apple's iCloud Keychain or Google's Password Manager) sync your Passkeys across devices. If you lose your phone but have a laptop signed into the same account, you can still log in. Some platforms also let you scan a QR code on a new device to authenticate using a nearby device.

If you have completely lost access to all devices with your Passkey, contact the wallet administrator for account recovery options.

**Best practice:** Make sure your Passkeys are backed up through your device ecosystem (Apple, Google, or a hardware security key stored in a safe place).

---

## Chains

### What chains are supported?

TEENet Wallet supports all major signature schemes used by blockchain systems. Chains marked with **✓** have been tested end-to-end.

| Scheme | Blockchains |
|--------|-------------|
| ECDSA secp256k1 | Ethereum **✓**, Optimism **✓**, Base **✓**, BNB Chain **✓**, Avalanche **✓**, Arbitrum, Polygon, Bitcoin, + any EVM chain |
| Ed25519 | Solana **✓** |

Any EVM-compatible chain can be added at runtime. Ask your agent to list available chains, or check `GET /api/chains` in the API.

---

## Need Help?

If you run into any issues not covered here, contact your wallet administrator. When reporting a problem, it helps to include:

- What you were trying to do.
- What you expected to happen.
- What actually happened (including any error messages).
- Which browser and device you are using.
