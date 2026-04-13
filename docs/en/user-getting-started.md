[wallet-url]: https://test.teenet.io/instance/wallet/

# Getting Started

You tell OpenClaw what you want to do, it uses TEENet Wallet to do it, and big transactions need your fingerprint. Here's how to set it all up.

---

## Step 1: Create Your Account

1. Open your browser and go to [TEENet Wallet][wallet-url]. Use Google Chrome for the best experience.

2. Enter your name or email in the required field.

<div align="center"><img src="picture/register.png" alt="Registration page" width="360" /></div>

3. Click **Register with Passkey**. Your browser will ask you to set up a **Passkey**:
   - On a phone or laptop with biometrics: scan your fingerprint or use Face ID.
   - On a desktop: use a security key (USB device) or scan a QR code with your phone.

<div align="center"><img src="picture/register2.png" alt="Passkey creation prompt" width="360" /></div>

4. Done. Your account is created and you're logged in -- no password to remember.

> **What is a Passkey?** A Passkey replaces passwords with your fingerprint, Face ID, or a security key. Your biometric data never leaves your device. Passkeys can't be phished, guessed, or stolen in a data breach.

## Step 2: Generate an API Key

The API key lets OpenClaw access your wallet. Without it, OpenClaw can't do anything.

1. Click the **Settings** icon (gear) in the top-right corner.

2. In the API Keys section, type a label (e.g., "my-openclaw") and click **Generate API Key**.

3. Authenticate with your Passkey.

4. Copy the key immediately (starts with **ocw_**). It's only shown once.

<div align="center"><img src="picture/generate_api.png" alt="API key generated" width="360" /></div>

## Step 3: Connect OpenClaw

1. Open your OpenClaw chat.

2. Send **"Install this skill:"** followed by:

   ```
   https://github.com/TEENet-io/teenet-wallet/blob/master/skill/tee-wallet/SKILL.md
   ```

3. When prompted, enter:
   - **TEE_WALLET_API_URL** -- `https://test.teenet.io/instance/f8e649535e1d2838ae2817992f946d6a`
   - **TEE_WALLET_API_KEY** -- the key you copied in Step 2

<div align="center"><img src="picture/tg.png" alt="Connect OpenClaw via Telegram" width="360" /></div>

That's it. OpenClaw can now use your wallet.

## Step 4: Create a Wallet

Tell OpenClaw:

- **"Create an Ethereum wallet"**
- **"Create a Solana wallet called Trading"**

Ethereum wallets take about a minute (distributed key generation). Solana wallets are instant. Once you have the address, fund it from an exchange or another wallet.

<div align="center"><img src="picture/create.png" alt="Create wallet via OpenClaw" width="480" /></div>

## Step 5: Set a Spending Limit

Set an approval policy so large transactions require your fingerprint.

**In the Web UI:**

1. Go to [TEENet Wallet][wallet-url], click on a wallet to open its detail page, then select the **Threshold** tab.
2. Set an **Approval Threshold (USD)** (e.g., $100) -- anything above this needs your approval.
3. Optionally set a **Daily Limit (USD)** (e.g., $1,000) -- a hard cap per day.
4. Click **Save policy**.

**Or just tell OpenClaw:** "Set approval threshold to $100" / "Set daily limit to $1,000"

<div align="center"><img src="picture/threshold.png" alt="Set threshold via OpenClaw" width="480" /></div>

Policy changes always require your Passkey approval, so a compromised API key can't weaken your protections.

<div align="center"><img src="picture/appqueue.png" alt="Web approval page" width="480" /></div>

---
[Next: What You Can Do](/en/user-commands.md)
