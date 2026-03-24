# User Guide

Welcome to the TEENet Wallet user guide. This covers everything you need to get up and running with your wallet and OpenClaw on Telegram.

---

## What is TEENet Wallet?

TEENet Wallet is a secure cryptocurrency wallet designed to work with OpenClaw, an AI assistant you interact with through Telegram. Your private keys are never stored in one place -- they are split across multiple pieces of specialized hardware called Trusted Execution Environments (TEEs) using threshold cryptography. No single server, person, or program ever has access to your full private key. OpenClaw connects to your wallet and handles the day-to-day work: sending crypto, checking balances, swapping tokens, and interacting with smart contracts -- all through simple conversation in Telegram.

You stay in control through a system of spending limits and approvals. Small, routine transactions go through instantly when you ask OpenClaw. Larger transactions -- anything above a dollar threshold you set -- require you to approve with your fingerprint or Face ID before they are executed. Think of it as giving a trusted assistant a spending card with a limit you decide. OpenClaw does the work, and you sign off on the big decisions.

---

## How It Works

You chat with OpenClaw on Telegram, just like you would message a friend. When you want to do something with crypto -- send money, check a balance, swap tokens -- you tell OpenClaw in plain language. OpenClaw uses the TEENet Wallet skill to carry out your request.

Here is how the flow works in practice:

- **Small amounts:** You tell OpenClaw to send 0.01 ETH to a friend. OpenClaw handles it instantly, confirms the transaction, and sends you the receipt right in Telegram. No extra steps needed.
- **Large amounts:** You tell OpenClaw to send 5 ETH. Because that exceeds your spending threshold, OpenClaw submits the transaction and sends you a notification with an approval link. You tap the link, review the details in your browser, and approve with your fingerprint or Face ID. Then the transaction goes through.
- **Balances and info:** You ask OpenClaw "What's my balance?" and it replies immediately. No approval needed for read-only operations.
- **DeFi and contracts:** You tell OpenClaw to swap ETH for USDC on Uniswap. OpenClaw builds and submits the transaction. If it is above your threshold, you approve it. If not, it goes through automatically.

Like having a crypto-savvy assistant with a spending limit you control.

The Web UI at https://wallet.example.com is your dashboard for oversight. You use it to view wallets, set spending policies, approve or reject pending transactions, manage API keys, and review a full history of everything OpenClaw has done on your behalf.

---

## Getting Started (Step by Step)

### Step 1: Create Your Account

1. Open your browser and go to https://wallet.example.com.

2. You will see a screen with two options at the top: **Login** and **Register**. Click **Register**.

3. Choose a display name. This can be your name, a nickname, or an email address -- whatever helps you recognize your account.

4. Click **Register Device**. Your browser will ask you to set up a **Passkey**.

5. Follow the prompt on your device:
   - On a phone or laptop with biometrics, you will scan your fingerprint or use Face ID.
   - On a desktop, you may use a security key (a small USB device) or scan a QR code with your phone.
   - Some systems will offer to use your device's screen lock (PIN or pattern).

6. Once your Passkey is confirmed, your account is created. You are automatically logged in -- there is no password to remember.

A Passkey is a modern replacement for passwords. Instead of typing a password, you prove your identity with something built into your device -- your fingerprint, your face, or a physical security key. Your biometric data never leaves your device. The wallet only receives a cryptographic confirmation that you are you. Passkeys cannot be phished, guessed, or stolen in a data breach.

**Tip:** For the best experience, use Google Chrome. Some other browsers have limited Passkey support.

### Step 2: Generate an API Key

The API key is how OpenClaw authenticates with your wallet. Without it, OpenClaw cannot access your account.

1. Log in to https://wallet.example.com.

2. Click the **Account** tab at the top of the screen.

3. Optionally type a label for the key (for example, "my-openclaw"). This helps you identify the key later.

4. Click **Generate API Key**.

5. Authenticate with your Passkey (fingerprint or Face ID).

6. Your new API key appears. It starts with **ocw_** followed by a long string of characters.

**Copy it immediately and store it somewhere safe.** The full key is only shown once. If you lose it, you will need to generate a new one.

### Step 3: Install the Wallet Skill on OpenClaw

Now you connect OpenClaw to your wallet by installing the TEENet Wallet skill.

1. Open your OpenClaw chat on Telegram.

2. Send this message:

   **"Install this skill: https://github.com/TEENet-io/teenet-wallet/blob/master/skill/tee-wallet/SKILL.md"**

3. OpenClaw will confirm it found the skill and ask you for two settings:

   - **TEE_WALLET_API_URL** -- enter your wallet URL: **https://wallet.example.com**
   - **TEE_WALLET_API_KEY** -- paste the API key you copied in Step 2 (the one starting with ocw_)

4. OpenClaw will confirm the skill is installed and ready to use.

That is it. OpenClaw can now interact with your wallet.

### Step 4: Create a Wallet

Tell OpenClaw to create your first wallet. For example:

- **"Create an Ethereum wallet"**
- **"Create a Solana wallet called Trading"**

OpenClaw will create the wallet and show you the address. Ethereum wallets may take up to a minute (the key generation process is more involved). Solana wallets are created in seconds.

Once you have the address, fund it by sending crypto to that address from an exchange or another wallet.

### Step 5: Set a Spending Limit

Before OpenClaw starts handling real money, set an approval policy so large transactions require your fingerprint.

**Through the Web UI:**

1. Go to https://wallet.example.com and click on your wallet to expand it.

2. Click the **Policy** tab.

3. Set a **Threshold (USD)** -- for example, $100. Any transaction above this amount will require your approval.

4. Optionally set a **Daily Limit (USD)** -- for example, $1,000. This is a hard cap on total spending per day, even for transactions below the threshold.

5. Click **Save Policy**.

**Through OpenClaw:** You can also tell OpenClaw in Telegram:

- **"Set approval threshold to $100"**
- **"Set daily limit to $1,000"**

Policy changes made through OpenClaw always require your Passkey approval in the Web UI, so a compromised API key cannot weaken your protections.

---

## Talking to OpenClaw (Telegram Commands)

This is the heart of the experience. You interact with your wallet by chatting with OpenClaw in Telegram. Here are concrete examples of what you can say.

### Wallet Management

- **"Create an Ethereum wallet called Trading"** -- creates a new wallet on Ethereum with the label "Trading."
- **"Create a Solana wallet"** -- creates a new Solana wallet.
- **"Show my wallets"** -- lists all your wallets with their addresses, chains, and labels.
- **"What's my balance?"** -- shows the balance of your current wallet (or asks you to pick one if you have several).
- **"Show all my balances"** -- shows balances across all your wallets.

### Sending Crypto

- **"Send 0.1 ETH to 0xABC...123"** -- sends 0.1 ETH to the specified address. If it is below your threshold, it goes through instantly.
- **"Send 50 USDC to 0xDEF...456"** -- sends 50 USDC (an ERC-20 token transfer). OpenClaw will make sure the USDC contract is whitelisted first.
- **"Transfer 1 SOL to ABC...XYZ"** -- sends 1 SOL on Solana.

### DeFi and Contract Interaction

- **"Swap 0.5 ETH for USDC on Uniswap"** -- OpenClaw builds the swap transaction and submits it.
- **"Approve USDC spending for Uniswap router"** -- grants a token allowance (this always requires your Passkey approval, regardless of amount).
- **"Call balanceOf on USDC contract for my address"** -- reads data from a smart contract without sending a transaction.

### Whitelist Management

- **"Add USDC contract to my whitelist"** -- proposes adding the USDC contract. You will need to approve this in the Web UI.
- **"Show my whitelisted contracts"** -- lists all contracts currently on your whitelist.

Note: Adding a contract to your whitelist always requires Passkey approval. This is a safety measure to prevent unauthorized contracts from being added.

### Policy and Security

- **"Set approval threshold to $200"** -- proposes changing your spending threshold (requires Passkey approval).
- **"Show my approval policy"** -- displays your current threshold and daily limit.
- **"Show pending approvals"** -- lists any transactions waiting for your approval.

---

## Approving Transactions

When OpenClaw submits a transaction that exceeds your spending threshold, the transaction is held in a pending state. Nothing is sent to the blockchain until you act.

### How it works

1. OpenClaw tells you in Telegram that a transaction needs your approval. It includes a direct link to the approval screen.

2. Tap the link. It opens in your browser and takes you straight to that specific transaction.

3. Review the details: the recipient address, the amount, the currency, and the estimated USD value.

4. Tap **Approve** and authenticate with your fingerprint or Face ID. Or tap **Reject** if something looks wrong.

5. If you approve, the transaction is signed and sent to the blockchain. OpenClaw will confirm in Telegram with a transaction hash and explorer link.

### Using the Web UI directly

You can also go to https://wallet.example.com and click the **Approvals** tab to see all pending transactions. Each one shows:

- The wallet the transaction is from.
- The recipient address.
- The amount and currency.
- The estimated USD value.
- A countdown timer showing how much time remains before it expires.

### Expiry

Pending approvals expire after **30 minutes**. If you do not approve or reject within that window, the transaction is automatically cancelled. If you still want to proceed, ask OpenClaw to submit it again.

---

## Web UI Overview

The Web UI at https://wallet.example.com is your dashboard for oversight and management. Here is what you can do:

- **Wallets tab** -- View all your wallets, their addresses, and balances. Expand a wallet to see its full details, approval policy, and contract whitelist.

- **Approvals tab** -- Review and approve or reject pending transactions that exceed your spending threshold.

- **Account tab** -- Generate new API keys, view existing keys, and revoke keys you no longer need.

- **History tab** -- View a complete audit trail of every action: transfers, approvals, wallet creations, policy changes, whitelist updates, and more. Use the filter to narrow by action type.

- **Approval policies** -- Expand any wallet and click the Policy tab to set or change spending thresholds and daily limits.

- **Contract whitelist** -- Expand any wallet and click the Whitelist tab to see whitelisted contracts. You can delete contracts from the whitelist here. Adding new contracts is done through OpenClaw (and requires your Passkey approval).

Day-to-day operations like sending crypto and interacting with contracts are handled through OpenClaw in Telegram. The Web UI is for oversight, approvals, and security configuration.

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

Open your OpenClaw chat on Telegram and send: **"Install this skill: https://github.com/TEENet-io/teenet-wallet/blob/master/skill/tee-wallet/SKILL.md"**. OpenClaw will ask you for two settings: TEE_WALLET_API_URL (enter https://wallet.example.com) and TEE_WALLET_API_KEY (paste your API key starting with ocw_). Once both are provided, the skill is installed and ready.

### What can OpenClaw do without my approval?

OpenClaw can freely perform the following without requiring your Passkey:

- Send transactions that are below your USD approval threshold.
- Check wallet balances and addresses.
- View transaction history.
- Interact with contracts on your whitelist that have auto-approve enabled (as long as the transaction value is below your threshold).

Everything else requires your approval.

### What always needs my approval?

The following actions always require your Passkey, regardless of spending limits:

- Transactions above your USD approval threshold.
- Transactions that would exceed your daily spending limit.
- Adding a new contract or token to the whitelist.
- Changing or disabling an approval policy.
- Deleting a wallet.
- Generating or revoking API keys.
- High-risk contract operations like token approvals (granting spending permission to another contract).

### Can I use multiple OpenClaw bots?

Yes. Generate a separate API key for each one in the Account tab. Each bot operates independently but shares your wallets and is subject to the same approval policies. If one misbehaves, revoke its specific key without affecting the others.

### What if OpenClaw tries to overspend?

If OpenClaw submits a transaction above your threshold, it is held in a pending state and nothing is sent until you approve it. If the transaction would also exceed your daily limit, it is blocked entirely -- even you cannot approve it until the next day. The daily limit is a hard cap enforced at the infrastructure level.

### How do I stop OpenClaw from using my wallet?

Go to https://wallet.example.com, click the **Account** tab, find the API key OpenClaw is using, and click **Revoke**. Authenticate with your Passkey. OpenClaw immediately loses all access to your wallets. If you want to reconnect later, generate a new API key and provide it to OpenClaw.

### What is a Passkey?

A Passkey is a modern replacement for passwords. Instead of typing a password, you prove your identity using your fingerprint, Face ID, or a physical security key. Your biometric data never leaves your device -- the wallet only receives a cryptographic proof that you authenticated successfully. Passkeys cannot be phished, guessed, or stolen in a data breach. Think of it like unlocking your phone, but for your wallet.

### What if I lose my device?

Your wallets and funds are safe. They remain in the TEE infrastructure regardless of what happens to your device. You just need to regain access to your account.

Many Passkey systems (like Apple's iCloud Keychain or Google's Password Manager) sync your Passkeys across devices. If you lose your phone but have a laptop signed into the same account, you can still log in. Some platforms also let you scan a QR code on a new device to authenticate using a nearby device.

If you have completely lost access to all devices with your Passkey, contact the wallet administrator for account recovery options.

**Best practice:** Make sure your Passkeys are backed up through your device ecosystem (Apple, Google, or a hardware security key stored in a safe place).

### What chains are supported?

TEENet Wallet currently supports:

- **Ethereum Mainnet** -- for ETH and ERC-20 tokens (USDC, USDT, etc.).
- **Optimism Mainnet** -- an Ethereum Layer 2 with lower fees.
- **Solana Mainnet** -- for SOL and SPL tokens.
- **Sepolia and Holesky** -- Ethereum test networks (free test tokens).
- **Solana Devnet** -- Solana test network (free test tokens).
- **BSC Testnet** -- Binance Smart Chain test network.
- **Base Sepolia** -- Base network testnet.

Additional EVM-compatible chains can be added. Ask your wallet administrator if you need a specific chain.

---

## Need Help?

If you run into any issues not covered here, contact your wallet administrator. When reporting a problem, it helps to include:

- What you were trying to do.
- What you expected to happen.
- What actually happened (including any error messages).
- Which browser and device you are using.
