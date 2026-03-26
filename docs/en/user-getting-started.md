[wallet-url]: https://test.teenet.io/instance/f8e649535e1d2838ae2817992f946d6a

# Getting Started

You tell OpenClaw what you want to do, it uses TEENet Wallet to do it, and big transactions need your fingerprint. Here's how to set it all up.

---

## Step 1: Create Your Account

1. Open your browser and go to [TEENet Wallet][wallet-url]. Use Google Chrome for the best experience.

2. Click **Register** at the top of the screen.

3. Choose a display name -- your name, a nickname, anything that helps you recognize your account.

4. Click **Register Device**. Your browser will ask you to set up a **Passkey**:
   - On a phone or laptop with biometrics: scan your fingerprint or use Face ID.
   - On a desktop: use a security key (USB device) or scan a QR code with your phone.

5. Done. Your account is created and you're logged in -- no password to remember.

> **What is a Passkey?** A Passkey replaces passwords with your fingerprint, Face ID, or a security key. Your biometric data never leaves your device. Passkeys can't be phished, guessed, or stolen in a data breach.

## Step 2: Generate an API Key

The API key lets OpenClaw access your wallet. Without it, OpenClaw can't do anything.

1. Click the **Account** tab.

2. Type a label (e.g., "my-openclaw") and click **Generate API Key**.

3. Authenticate with your Passkey.

4. Copy the key immediately (starts with **ocw_**). It's only shown once.

## Step 3: Connect OpenClaw

1. Open your OpenClaw chat.

2. Send **"Install this skill:"** followed by:

   ```
   https://github.com/TEENet-io/teenet-wallet/blob/master/skill/tee-wallet/SKILL.md
   ```

3. When prompted, enter:
   - **TEE_WALLET_API_URL** -- `https://test.teenet.io/instance/f8e649535e1d2838ae2817992f946d6a`
   - **TEE_WALLET_API_KEY** -- the key you copied in Step 2

That's it. OpenClaw can now use your wallet.

## Step 4: Create a Wallet

Tell OpenClaw:

- **"Create an Ethereum wallet"**
- **"Create a Solana wallet called Trading"**

Ethereum wallets take about a minute (distributed key generation). Solana wallets are instant. Once you have the address, fund it from an exchange or another wallet.

## Step 5: Set a Spending Limit

Set an approval policy so large transactions require your fingerprint.

**In the Web UI:**

1. Go to [TEENet Wallet][wallet-url], expand your wallet, click the **Policy** tab.
2. Set a **Threshold (USD)** (e.g., $100) -- anything above this needs your approval.
3. Optionally set a **Daily Limit (USD)** (e.g., $1,000) -- a hard cap per day.
4. Click **Save Policy**.

**Or just tell OpenClaw:** "Set approval threshold to $100" / "Set daily limit to $1,000"

Policy changes always require your Passkey approval, so a compromised API key can't weaken your protections.

---
[Next: Talking to OpenClaw](user-commands.md)
