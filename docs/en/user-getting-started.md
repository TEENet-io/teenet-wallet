[wallet-url]: https://wallet.teenet.app

# Getting Started

## Step 1: Create Your Account

1. Open your browser and go to the [wallet registration page][wallet-url]. Use Google Chrome for the best experience.

> **Note:** TEENet Wallet is currently in testing phase with a limit of 500 users. Registration is first-come, first-served.

2. Enter your email address and click **Send code**.

<div align="center"><img src="picture/register.png" alt="Registration page" width="360" /></div>

3. Check your inbox for the 6-digit verification code, then enter it on the wallet page.

4. Click **Register with Passkey**. Your browser will ask you to set up a **Passkey**:
   - On a phone or laptop with biometrics: scan your fingerprint or use Face ID.
   - On a desktop: use a security key (USB device) or scan a QR code with your phone.

<div align="center"><img src="picture/register2.png" alt="Passkey creation prompt" width="360" /></div>

5. Done. Your account is created and you're logged in -- no password to remember.

> **What is a Passkey?** A Passkey replaces passwords with your fingerprint, Face ID, or a security key. Your biometric data never leaves your device. Passkeys can't be phished, guessed, or stolen in a data breach.

## Step 2: Generate an API Key

The API key lets your AI agent access your wallet. Without it, no agent can interact with your wallets.

1. Click the **Settings** icon (gear) in the top-right corner.

2. In the API Keys section, type a label (e.g., "my-agent") and click **Generate API Key**.

3. Authenticate with your Passkey.

4. Copy the key immediately (starts with **ocw_**). It's only shown once.

<div align="center"><img src="picture/generate_api.png" alt="API key generated" width="360" /></div>

## Step 3: Connect Your Agent

Provide your agent with the **API key** from Step 2 and the **wallet API URL** shown on your account page. How you do this depends on your agent platform.

**OpenClaw example:**

1. Open your OpenClaw chat.

2. Copy and paste this message:

   ```
   Install this skill: https://github.com/TEENet-io/teenet-wallet/blob/main/skill/teenet-wallet/SKILL.md
   ```

3. When prompted, enter:
   - **TEENET_WALLET_API_URL** -- the wallet API URL shown on your account page
   - **TEENET_WALLET_API_KEY** -- the key you copied in Step 2

<div align="center"><img src="picture/tg.png" alt="Connect OpenClaw via Telegram" width="360" /></div>

## Step 4: Test Your Wallet

Ask your agent to run a quick test. On OpenClaw, type `/test`. The agent will create a testnet wallet for you if you don't have one, then walk you through:

1. **Check balance** -- confirm the wallet is active
2. **Get test tokens** -- from a faucet (free, testnet only)
3. **Send to yourself** -- verifies TEE distributed signing works
4. **Set a $1 approval threshold** -- requires your Passkey
5. **Send a small amount** -- goes through automatically (below threshold)
6. **Send a larger amount** -- held until you approve with Passkey
7. **Whitelist a token** -- adds test USDC to your contract whitelist

Testnet faucets: [Sepolia ETH](https://faucet.google.com/ethereum/sepolia) · [Solana Devnet](https://faucet.solana.com) · [Test USDC](https://faucet.circle.com)
