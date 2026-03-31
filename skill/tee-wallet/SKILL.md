---
name: tee-wallet
description: "Manage crypto wallets secured by TEE. Use when user asks to create wallet, check balance, send crypto, sign messages, or manage crypto assets. Supports Ethereum and Solana."
metadata:
  openclaw:
    emoji: "🔐"
    requires:
      env:
        - TEE_WALLET_API_URL
        - TEE_WALLET_API_KEY
      anyBins:
        - python3
        - curl
    primaryEnv: TEE_WALLET_API_KEY
---

# TEE Wallet Skill

You manage crypto wallets backed by TEE (Trusted Execution Environment) hardware security.
Private keys are distributed across TEE nodes via threshold cryptography — they never exist
as a whole outside secure hardware.

## Configuration

- `TEE_WALLET_API_URL`: The wallet service URL (default: `https://test.teenet.io/instance/wallet`)
- `TEE_WALLET_API_KEY`: Your API key (starts with `ocw_`)

RPC URLs are configured in the wallet service's `chains.json` file (or via the `CHAINS_FILE` env var on the server), not as client-side environment variables. The wallet service handles all blockchain RPC communication internally.

## Onboarding Flow

When a user interacts with the wallet skill for the first time (no prior wallet context in the conversation), run through this flow automatically.

### Step 0 — Check environment variables

Before making any API calls, verify both required environment variables are set.

**If `TEE_WALLET_API_URL` is missing**, use the default: `https://test.teenet.io/instance/wallet`

**If `TEE_WALLET_API_KEY` is missing**, guide the user through account setup:
> 🔑 **API key not configured — let's set one up.**
>
> You need a Passkey account and an API key to use this wallet. Here's how:
>
> **1. Open the Web UI**
> Go to [`https://test.teenet.io/instance/wallet`](https://test.teenet.io/instance/wallet) in your browser (Chrome or Edge recommended for Passkey support).
>
> **2. Register an account**
> Click **Register** and follow the Passkey prompts. You'll need a device with biometric or hardware security key support (TouchID, FaceID, YubiKey, etc.). If registration is invite-only, ask an existing user for an invite link.
>
> **3. Generate an API key**
> After logging in, go to **Settings → API Keys**, click **Generate New Key**, and give it a label (e.g. "AI Agent"). **Copy the key immediately** — it starts with `ocw_` and is only shown once.
>
> **4. Configure the environment**
> Set `TEE_WALLET_API_KEY` to the key you just copied, then try again.

Once both variables are set, continue to Step 1.

### Step 1 — Verify connectivity

```bash
curl -s "${TEE_WALLET_API_URL}/api/health"
```

Expected response: `{"status":"ok","service":"teenet-wallet","db":true}`

If the request fails or `status` is not `ok`, stop and show:
> ❌ Cannot reach the wallet service at `${TEE_WALLET_API_URL}`. Check that the URL is correct and the service is running.

If a subsequent authenticated call returns `invalid API key`, show:
> ❌ API key rejected. Check that `TEE_WALLET_API_KEY` is correct (should start with `ocw_`).

### Step 2 — Check existing wallets

```bash
curl -s "${TEE_WALLET_API_URL}/api/wallets" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

**If the user already has wallets** → show a summary and proceed to their request:
> 👋 **Welcome back!** You have {N} wallet(s):
> 1. `0xabcd…1234` — My Main Wallet (Ethereum) ✅
> 2. `HN7c…Qx9f` — Trading (Solana) ✅
>
> What would you like to do?

Then stop onboarding — the user is already set up.

**If no wallets exist** → continue to Step 3.

### Step 3 — Discover available chains

```bash
curl -s "${TEE_WALLET_API_URL}/api/chains"
```

Present available chains grouped by family and ask the user to pick one:
> 🆕 **No wallets yet — let's create your first one!**
>
> Available chains:
> **EVM:** Ethereum (ETH) · Sepolia (ETH) · Base Sepolia (ETH) · …
> **Solana:** Solana (SOL) · Solana Devnet (SOL)
>
> Which chain would you like to start with?

If the user is unsure, recommend a testnet:
> 💡 **Tip:** Try **Sepolia** (Ethereum testnet) or **Solana Devnet** to experiment with free test tokens before using real funds.

### Step 4 — Create first wallet

Once the user picks a chain, create the wallet per Section 1 (Create Wallet). Remind them:
- **Ethereum / EVM wallets** take 1–2 minutes (ECDSA key generation across TEE nodes)
- **Solana wallets** are created instantly

### Step 5 — Recommend next steps

After the wallet is created successfully, suggest:
> 🛡️ **Your wallet is ready! Recommended next steps:**
> 1. **Fund your wallet** — send {currency} to `{address}`
> 2. **Set an approval policy** — protect large transfers with a USD threshold (e.g. `/policy 100`)
> 3. **Whitelist tokens** — add token contracts you plan to use (see Section 7)

For testnet wallets, include relevant faucet links:
- **Sepolia ETH:** `https://cloud.google.com/application/web3/faucet/ethereum/sepolia`
- **Solana Devnet SOL:** `https://faucet.solana.com`
- **Sepolia / Base Sepolia USDC:** `https://faucet.circle.com`

### Guided Test Flow

When the user runs `/test` or asks to test the wallet, walk them through these steps interactively. Wait for each step to succeed before moving to the next. The user must have at least one wallet on a **testnet** (Sepolia, Base Sepolia, or Solana Devnet) — if not, create one first.

**Step 1 — Check balance**

> Let's start by checking your wallet balance.

Run `/balance` for the testnet wallet. Show the result. If balance is zero, continue to Step 2. If already funded, skip to Step 3.

**Step 2 — Get test tokens**

> Your wallet is empty. Let's get some free test tokens.

Provide the appropriate faucet link based on the wallet's chain:
- **Sepolia ETH:** [`https://cloud.google.com/application/web3/faucet/ethereum/sepolia`](https://cloud.google.com/application/web3/faucet/ethereum/sepolia)
- **Solana Devnet SOL:** [`https://faucet.solana.com`](https://faucet.solana.com)
- **Base Sepolia ETH:** [`https://cloud.google.com/application/web3/faucet/ethereum/sepolia`](https://cloud.google.com/application/web3/faucet/ethereum/sepolia) (bridge from Sepolia) or use Base Sepolia faucets

> Paste your wallet address `{address}` into the faucet, then tell me when you're done.

After the user confirms, wait 15 seconds and re-check balance to verify funds arrived.

**Step 3 — Send a small transfer**

> Now let's test a transfer. I'll send a tiny amount to yourself to verify the full signing flow.

Send a transfer of `0.0001 ETH` (or `0.001 SOL`) from the wallet **to its own address**. This tests the complete TEE signing pipeline with zero risk.

Show the transaction hash and explorer link on success.

**Step 4 — Set an approval policy**

> Let's set up an approval policy to protect your wallet.

Set a low USD threshold:
```
/policy 10
```

This creates a pending approval. Guide the user to approve it via the Web UI link. Poll until approved.

> ✅ Policy active. Transfers above $10 now require Passkey approval.

**Step 5 — Trigger Passkey approval**

> Now let's test the approval flow. I'll send a transfer that exceeds your $10 threshold.

Send a transfer of `0.01 ETH` (or `0.1 SOL`) to the wallet's own address. This should trigger `pending_approval`.

Guide the user to the approval link and poll until resolved. On success:
> ✅ Approval flow works! You approved the transaction with your Passkey hardware.

**Step 6 — Whitelist a test token**

> Last step — let's add a test token to your whitelist.

For Sepolia wallets, propose adding USDC:
```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"contract_address":"0x1c7d4b196cb0c7b01d743fbc6116a902379c7238","symbol":"USDC","decimals":6}'
```

Guide the user to approve the whitelist request. Once approved:
> ✅ USDC is now whitelisted. You can send and receive USDC on this wallet.
>
> Get free test USDC from the [Circle faucet](https://faucet.circle.com).

**Completion:**
> 🎉 **All tests passed!** Your wallet is fully functional:
> - ✅ Balance queries
> - ✅ TEE threshold signing
> - ✅ Approval policy enforcement
> - ✅ Passkey hardware approval
> - ✅ Token whitelist management
>
> You're ready to use the wallet for real operations. Type `/wallets` to see your wallets or `/balance` to check balances.

### When to skip onboarding

Skip the full flow and go directly to the requested operation if:
- The user gives a specific command (e.g. `/balance`, `/transfer 0.1 ETH to 0x...`)
- The conversation already has wallet context from earlier messages
- The user explicitly asks to skip setup

## Smart Wallet Selection

**Never ask the user to provide a wallet ID directly.** Always resolve the wallet automatically:

1. If the user already mentioned a wallet in this conversation (by label, address, or chain) — use that one.
2. Otherwise call `GET /api/wallets` and:
   - If only **one** wallet matches the required chain → use it silently.
   - If **multiple** wallets match → show a compact list and ask the user to pick:
     > Which wallet do you want to use?
     > 1. `0xabcd…1234` — My Main Wallet (ETH)
     > 2. `0x5678…9abc` — DeFi Wallet (ETH)
   - If **no** wallet exists for that chain → offer to create one.

You may cache wallet details briefly within the conversation for convenience, **but `/api/wallets` is the source of truth**.
Always re-fetch `/api/wallets` before:
- showing the wallet list
- showing “all balances” / totals / account-wide balances
- assuming a wallet still exists after prior create/delete activity
- checking balances for multiple wallets after any create/delete activity

Do not build an “all balances” response from a stale wallet list remembered from earlier in the chat.
Do not query chain balances for wallets that are no longer present in the latest `/api/wallets` response.

**Wallet IDs are UUIDs** (e.g. `8a2fbc16-faf4-451a-be34-9fc5c49cde00`), not sequential numbers. Never expose raw wallet IDs in normal chat — use list indices instead.

When the user refers to wallets as `1`, `2`, `3`, etc., interpret those numbers as the **current displayed list index**, not the raw wallet `id`.
Only interpret a UUID as the real wallet `id` if the user explicitly provides one.

## Available Operations

### 1. Create Wallet

When user asks to create a new wallet:

```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"chain":"<chain_name>","label":"<user description>"}'
```

- The `chain` value must be one of the names returned by `GET /api/chains` (e.g. `ethereum`, `solana`, `sepolia`, `solana-devnet`, or any user-added custom chain)
- If user doesn't specify a chain, first call `GET /api/chains` to list available options and ask them to choose
- Ethereum wallets may take 1-2 minutes to create (ECDSA key generation)
- Solana wallets are created instantly
- After success, show:
  > ✅ **Wallet created**
  > **Address:** `{address}`
  > **Chain:** {chain}
  >
  > Next steps: fund this address to get started, or set an approval policy (Section 10) to protect large transfers.

### 2. List Wallets

```bash
curl -s "${TEE_WALLET_API_URL}/api/wallets" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

**Always** present wallets as a numbered list so the user can refer to wallets by number. Use this exact format:

> **Your Wallets**
>
> 1. **My Main Wallet** — Ethereum · `0xabcd…1234` ✅
> 2. **DeFi Wallet** — Ethereum · `0x5678…9abc` ✅
> 3. **Test Wallet** — Sepolia · `0xdef0…5678` ⏳ creating…

Each line must include: **numbered index**, **label** (bold), **chain**, **abbreviated address**, and **status icon** (✅ ready, ⏳ creating, ❌ error).

Do **not** show the raw wallet `id` (UUID) in normal chat responses. Keep it internal for API calls only.

### 3. Get Wallet Details

```bash
curl -s "${TEE_WALLET_API_URL}/api/wallets/<id>" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

### 3.1 Rename Wallet

```bash
curl -s -X PATCH "${TEE_WALLET_API_URL}/api/wallets/<id>" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"label":"<new label>"}'
```

No approval needed. Works with API Key or Passkey session.

### 4. Sign Message / Send Transaction

When user asks to sign or send a transaction:

**Small amounts** (below the wallet's approval threshold, or no policy set): proceed directly without asking for confirmation — the backend approval policy is the safety net.

**Large amounts** (above threshold): the backend will return `pending_approval` and require Passkey — no need to confirm in chat either, the hardware approval is the confirmation.

```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/sign" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "message":"<hex-encoded message>",
    "encoding":"hex",
    "tx_context":{
      "type":"transfer",
      "from":"<sender address>",
      "to":"<recipient address>",
      "amount":"<amount>",
      "currency":"<ETH|SOL>",
      "memo":"<optional memo>"
    }
  }'
```

Always include `tx_context` with full transaction details — this is shown to the user during approval.

**If response has `"status":"signed"`**: show the signature to the user.

**If response has `"status":"pending_approval"`**: follow the **Approval Polling Flow** (Section 12).

### 5. Send Crypto (Transfer)

When user asks to send/transfer crypto, call the `/transfer` endpoint.
The **backend constructs the transaction, signs it via TEE, and broadcasts it** — no scripts needed.

**No chat confirmation needed** — the backend enforces approval policies. Just send the request directly. If the amount exceeds the threshold, the backend returns `pending_approval` and the user approves via Passkey hardware.

**Optional pre-check** (recommended for ETH transfers > 0.01 ETH): query native balance first.
If `balance < amount + estimated_gas (0.0005 ETH buffer)`, warn the user before sending.

```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/transfer" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "<recipient_address>",
    "amount": "<amount>",
    "memo": "<optional memo>"
  }'
```

**If response has `"status":"completed"`**: show the user:
> ✅ **Transaction sent**
> **Hash:** `{tx_hash}`
> **Chain:** {chain} · **Amount:** {amount} {currency}
> **To:** `{to}`
> 🔗 {explorer_link}

Explorer links by chain:
- Ethereum mainnet: `https://etherscan.io/tx/{hash}`
- Sepolia: `https://sepolia.etherscan.io/tx/{hash}`
- Base / Base Sepolia: `https://sepolia.basescan.org/tx/{hash}`
- Solana mainnet: `https://solscan.io/tx/{hash}`
- Solana devnet: `https://solscan.io/tx/{hash}?cluster=devnet`

**If response has `"status":"pending_approval"`**: follow the **Approval Polling Flow** (Section 12).

### 6. ERC-20 Token Transfer (Ethereum) / SPL Token Transfer (Solana)

Use this when the user asks to send a token (ERC-20 on Ethereum, or SPL token on Solana — e.g. USDC, WETH, USDT).

> ⚠️ **CRITICAL**: When sending tokens you MUST include the `token` field in the request body.
> Omitting `token` will send **native ETH/SOL** instead — a completely different transaction that costs
> real funds and cannot be reversed. Always double-check that your curl `-d` payload contains `"token": {...}`.

**Step 1 — Ensure the contract/mint is whitelisted** (see Section 7):
```bash
curl -s "${TEE_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```
If the contract/mint is not in the list, you can propose adding it via API key (creates a pending approval — see Section 7):

For Ethereum ERC-20:
```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"contract_address":"<0x...>","symbol":"<SYMBOL>","decimals":<N>}'
```

For Solana SPL tokens, use the token mint address (base58):
```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"contract_address":"<mint_address_base58>","symbol":"<SYMBOL>","decimals":<N>}'
```
Then follow the approval polling flow (Section 12, `contract_add` type). Alternatively, direct the user to add it immediately via Web UI (Passkey required):
> ⚠ The contract/mint `…` is not yet whitelisted. Requesting approval to add it… (or open Web UI → Contracts tab → Add to Whitelist for instant approval)

**Step 2 — Call `/transfer` with the `token` field** (no chat confirmation needed — backend enforces approval policies):
```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/transfer" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "<recipient_address>",
    "amount": "<human-readable amount, e.g. 100>",
    "token": {
      "contract": "<contract_address_lowercase_or_mint_base58>",
      "symbol": "<e.g. USDC>",
      "decimals": <e.g. 6>
    }
  }'
```

The amount is **in token units** (e.g. `100` for 100 USDC — the backend converts to raw units).

**Solana-specific behaviour**: if the recipient does not yet have an Associated Token Account (ATA) for the token mint, the backend creates it automatically in the same transaction. No extra steps are needed.

**Response handling** is identical to native transfer (Section 5) — include explorer link on success.

**Common ERC-20 token parameters:**

Ethereum Mainnet:
| Token | Contract | Decimals |
|-------|----------|----------|
| USDC  | `0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48` | 6 |
| USDT  | `0xdac17f958d2ee523a2206206994597c13d831ec7` | 6 |
| WETH  | `0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2` | 18 |
| DAI   | `0x6b175474e89094c44da98b954eedeac495271d0f` | 18 |

Sepolia Testnet:
| Token | Contract | Decimals | Faucet |
|-------|----------|----------|--------|
| USDC  | `0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238` | 6 | https://faucet.circle.com |
| WETH  | `0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14` | 18 | swap ETH→WETH on Uniswap |
| LINK  | `0x779877A7B0D9E8603169DdbD7836e478b4624789` | 18 | https://faucets.chain.link/sepolia |

Base Sepolia Testnet:
| Token | Contract | Decimals | Faucet |
|-------|----------|----------|--------|
| USDC  | `0x036CbD53842c5426634e7929541eC2318f3dCF7e` | 6 | https://faucet.circle.com |
| WETH  | `0x4200000000000000000000000000000000000006` | 18 | swap ETH→WETH on Uniswap |

### 7. Manage Contract Whitelist

The contract whitelist is a **security gate**: only pre-registered contracts/programs/mints can be called via `/transfer` or `/contract-call`. This applies equally to:
- **Ethereum**: ERC-20 contract addresses (`0x…`)
- **Solana**: SPL token mint addresses (base58) and program IDs (base58)

Removing entries requires **Passkey hardware authentication**. Adding can be done by either:
- **Passkey session** (Web UI): applied immediately
- **API key**: creates a pending approval (HTTP 202) that the Passkey owner must approve

**List whitelisted contracts** (API key works for reading):
```bash
curl -s "${TEE_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

**Add a contract via API key** (creates pending approval):
```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract_address": "<0x...>",
    "symbol": "<e.g. USDC>",
    "decimals": <e.g. 6>,
    "label": "<optional label>"
  }'
```

A 202 response means the request is pending approval:
```json
{ "success": true, "pending": true, "approval_id": 7, "message": "Contract whitelist request submitted for approval" }
```

After receiving a 202 response, tell the user and **include a block explorer link** so the approver can verify the contract:
> 📋 **Contract whitelist request submitted** (Approval ID: {approval_id})
> **Contract:** `{contract_address}` ({symbol})
> **Verify on explorer:** {explorer_link}
>
> The wallet owner must approve this via the Web UI before it can be used.
> [**→ Approve Request**]({TEE_WALLET_API_URL}/#/approve/{approval_id})

Use the appropriate explorer for the chain:
- **Ethereum:** `https://etherscan.io/address/{contract_address}`
- **Optimism:** `https://optimistic.etherscan.io/address/{contract_address}`
- **Arbitrum:** `https://arbiscan.io/address/{contract_address}`
- **Base:** `https://basescan.org/address/{contract_address}`
- **Polygon:** `https://polygonscan.com/address/{contract_address}`
- **BSC:** `https://bscscan.com/address/{contract_address}`
- **Avalanche:** `https://snowtrace.io/address/{contract_address}`
- **Solana:** `https://solscan.io/account/{mint_address}`

Then poll `GET /api/approvals/{approval_id}` every 15 seconds until `status` is `approved` or `rejected` (same as Section 12). Once `approved`, the contract/mint/program is whitelisted and token transfers or program calls can proceed.

**Add a contract via Passkey** (Web UI, applied immediately):
> Web UI → Wallets → select wallet → Contracts tab → Add to Whitelist.
> Fields: contract address (0x…), symbol (e.g. USDC), decimals (e.g. 6), optional label.

**Remove a contract** (Passkey session only):
> Web UI → Wallets → wallet → Contracts tab → ✕ button next to the contract.

**Update a contract via API key** (creates pending approval):
```bash
curl -s -X PUT "${TEE_WALLET_API_URL}/api/wallets/<id>/contracts/<cid>" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "label": "Updated label",
    "symbol": "USDC",
    "decimals": 6
  }'
```
Only include the fields you want to change (`label`, `symbol`, `decimals`). A 202 response means the update is pending approval — follow the Approval Polling Flow (Section 12, `contract_update` type).

**Why removing requires Passkey but adding/updating can be proposed by API key**: An API key can only *propose* changes — the human wallet owner with hardware security must still approve. Removal is always Passkey-only since it's a more sensitive operation (accidentally removing could block legitimate transfers).

### 7.1. Contract Whitelist Fields

| Field | Required | Description |
|-------|----------|-------------|
| `contract_address` | Yes | EVM contract address (`0x…`, lowercase) **or** Solana mint/program address (base58) |
| `symbol` | No | Token symbol (e.g. USDC) |
| `decimals` | No | Token decimals (e.g. 6 for USDC, 18 for WETH, 9 for most SPL tokens) |
| `label` | No | Human-readable label |

All contract operations initiated via API Key require Passkey approval. Passkey sessions execute directly.

### 7.2. General Contract Call

Use when the user wants to call any smart contract function (EVM) or invoke a Solana program instruction. Security model:
1. Contract/program must be whitelisted
2. All contract operations via API Key require Passkey approval; Passkey sessions execute directly

**EVM (Ethereum) — use `func_sig` and `args`:**
```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/contract-call" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "<0x...>",
    "func_sig": "<function signature, e.g. transfer(address,uint256)>",
    "args": ["<arg1>", "<arg2>"],
    "value": "<optional: ETH to send, e.g. 0.1>",
    "memo": "<optional>"
  }'
```

**Function signature format:** Use Solidity-style signatures like `transfer(address,uint256)`, `approve(address,uint256)`, `balanceOf(address)`.

**Supported argument types:** `address`, `uint256`, `int256`, `bool`, `bytes32`

**Solana — use `accounts` and `data` instead of `func_sig`/`args`:**
```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/contract-call" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "<program_id_base58>",
    "accounts": [
      {"pubkey": "<account1_base58>", "is_signer": false, "is_writable": true},
      {"pubkey": "<account2_base58>", "is_signer": false, "is_writable": false}
    ],
    "data": "<hex-encoded instruction data>",
    "memo": "<optional>"
  }'
```

- `contract`: the Solana program ID (base58) — must be whitelisted
- `accounts`: array of account metas in instruction order; the wallet's own address is added automatically as a signer if required
- `data`: hex-encoded instruction data (discriminator + encoded arguments)

The program must be added to the whitelist before calling (same API as Section 7). All contract/program operations via API Key require Passkey approval.

**Response:** Same as transfer — either `"status":"completed"` with `tx_hash`, or `"status":"pending_approval"` with approval link.

### 7.3. Approve Token Allowance (Convenience)

Shorthand for calling `approve(spender, amount)` on an ERC-20 contract:

```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/approve-token" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "<token contract 0x...>",
    "spender": "<spender address 0x...>",
    "amount": "<amount in token units, e.g. 1000>",
    "decimals": 6
  }'
```

> ⚠️ All contract operations via API Key require Passkey approval. Passkey sessions execute directly.

### 7.4. Revoke Token Approval (Convenience)

Shorthand for calling `approve(spender, 0)` to revoke a previously granted allowance:

```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/revoke-approval" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "<token contract 0x...>",
    "spender": "<spender address 0x...>"
  }'
```

> ⚠️ Requires Passkey approval when called via API Key.

### 7.5. Wrap SOL (Solana)

Wraps native SOL into wSOL (Wrapped SOL SPL token). Use when the user asks to wrap SOL or convert SOL to wSOL (required by many DeFi protocols on Solana).

The backend creates the wSOL Associated Token Account (ATA) automatically if it does not yet exist.

```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/wrap-sol" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"amount": "<SOL amount, e.g. 0.1>"}'
```

- `amount`: human-readable SOL amount to wrap (e.g. `"0.1"` wraps 0.1 SOL into 0.1 wSOL)

**On success:**
```json
{"status": "completed", "tx_hash": "...", "action": "wrap"}
```

Show the user:
> ✅ **Wrapped SOL**
> **Amount:** {amount} SOL → wSOL
> **Hash:** `{tx_hash}`
> 🔗 https://solscan.io/tx/{tx_hash}[?cluster=devnet]

**If response has `"status":"pending_approval"`**: follow the **Approval Polling Flow** (Section 12).

### 7.6. Unwrap SOL (Solana)

Closes the wSOL ATA and returns all wSOL back to native SOL. Use when the user asks to unwrap wSOL or convert wSOL back to SOL. The entire wSOL balance in the ATA is unwrapped in one operation.

```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/unwrap-sol" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{}'
```

No body parameters are required — the backend closes the entire wSOL ATA.

**On success:**
```json
{"status": "completed", "tx_hash": "...", "action": "unwrap"}
```

Show the user:
> ✅ **Unwrapped wSOL**
> All wSOL has been converted back to native SOL.
> **Hash:** `{tx_hash}`
> 🔗 https://solscan.io/tx/{tx_hash}[?cluster=devnet]

**If response has `"status":"pending_approval"`**: follow the **Approval Polling Flow** (Section 12).

### 7.7. Read-Only Contract Call

Query contract state without signing or sending a transaction. No gas, no approval needed.

```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/call-read" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "<0x...>",
    "func_sig": "<e.g. balanceOf(address)>",
    "args": ["<wallet_address>"]
  }'
```

Response:
```json
{ "success": true, "result": "0x0000000000000000000000000000000000000000000000000000000005f5e100", "contract": "0x...", "method": "balanceOf" }
```

Use this for:
- Checking token balances (`balanceOf(address)`)
- Querying allowances (`allowance(address,address)`)
- Reading contract state (e.g. `totalSupply()`, `name()`, `symbol()`, `decimals()`)

### 8. Address Book

The address book lets users save frequently used addresses with nicknames. Users can then transfer by nickname instead of pasting raw addresses.

**Nicknames** are case-insensitive, stored lowercase. The same nickname can have different addresses on different chains (e.g. "alice" on Ethereum and "alice" on Solana).

**Security model:**
- **List / Read**: API Key or Passkey
- **Add / Update via API Key**: creates a pending approval (HTTP 202) that the Passkey owner must approve
- **Add / Update via Passkey**: applied immediately (requires fresh credential)
- **Delete**: Passkey only

**List address book entries:**
```bash
curl -s "${TEE_WALLET_API_URL}/api/addressbook" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

Optional query params: `?nickname=alice`, `?chain=ethereum`

**Add an entry via API key** (creates pending approval):
```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/addressbook" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "nickname": "alice",
    "chain": "ethereum",
    "address": "0x1234567890123456789012345678901234567890",
    "memo": "Alice main wallet"
  }'
```

A 202 response means the request is pending approval:
```json
{ "success": true, "pending": true, "approval_id": 12345678, "message": "Address book entry submitted for approval" }
```

Follow the **Approval Polling Flow** (Section 12, `addressbook_add` type).

**Update an entry via API key** (creates pending approval):
```bash
curl -s -X PUT "${TEE_WALLET_API_URL}/api/addressbook/<id>" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"address": "0xnewaddress...", "memo": "updated memo"}'
```

Only include the fields you want to change (`nickname`, `address`, `memo`). A 202 response means the update is pending approval.

**Delete an entry** (Passkey only — via Web UI).

**Transfer by nickname:**

The `/transfer` endpoint accepts a nickname in the `to` field. If the value doesn't look like a raw address, the backend resolves it from the address book for the wallet's chain:

```bash
curl -s -X POST "${TEE_WALLET_API_URL}/api/wallets/<id>/transfer" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"to": "alice", "amount": "0.1"}'
```

If the nickname is not found for the wallet's chain, the API returns 400 with an error message.

> 💡 **Tip:** When a user says "send 0.1 ETH to alice", check the address book first. If "alice" exists for the wallet's chain, use the nickname directly in the `to` field — no need to resolve the address yourself.

### 8.1. Address Book Quick Reference

| Field | Required | Description |
|-------|----------|-------------|
| `nickname` | Yes | Contact name (lowercase, alphanumeric with `_` or `-`, max 100 chars) |
| `chain` | Yes | Chain name (e.g. `ethereum`, `solana`) |
| `address` | Yes | On-chain address (EVM `0x…` or Solana base58) |
| `memo` | No | Optional note (max 256 chars) |

### 9. Delete Wallet  

```bash
curl -s -X DELETE "${TEE_WALLET_API_URL}/api/wallets/<id>" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

Always confirm with user before deleting.

### 9. Check Balance

When the user asks for a wallet's balance, **show both native and token balances together** in one response.

**Step 1 — Native balance:**
```bash
curl -s "${TEE_WALLET_API_URL}/api/wallets/<id>/balance" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

> ⚠️ `/balance` returns the wallet's **native gas token** only (ETH / SOL). Never present this as a token balance.

**Step 2 — Fetch the wallet's token whitelist** (for Ethereum wallets):

Query the current wallet's contract whitelist to get its token list. Only tokens whitelisted on **this wallet** are checked for balances.

```bash
curl -s "${TEE_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

Use the returned contracts as the token list for on-chain balance queries (see Section 9.1). If the whitelist is empty, only the native balance is shown.

**Present all balances together:**
> 💼 **Wallet** `0xabcd…1234` (Ethereum)
> ├ ETH: **0.482 ETH**
> ├ USDC: **250.00 USDC**
> └ USDT: **100.00 USDT**

**After a transfer**: the balance reflects the latest confirmed block. Wait ~15 seconds before checking:
```bash
sleep 15 && curl -s "${TEE_WALLET_API_URL}/api/wallets/<id>/balance" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

### 9.1. Check ERC-20 Token Balances On-Chain (Batch)

For ERC-20 balances, query each token contract with `balanceOf(address)` via JSON-RPC `eth_call`.
Use the **batch script** below to query all tokens for a wallet in one go.

**Do this whenever the user asks for a token balance. Do not rely on `/balance`.**

```python
python3 - <<'PY'
import json, urllib.request, os

# --- Configure these ---
wallet = "<wallet_address>"
tokens = [
    # (contract_address_lowercase, symbol, decimals)
    # Fill from the wallet's own whitelist (GET /api/wallets/<id>/contracts)
    ("0x1c7d4b196cb0c7b01d743fbc6116a902379c7238", "USDC", 6),
    # add more as needed...
]
rpcs = [
    os.environ.get("ETH_RPC_URL", ""),
    "https://ethereum-sepolia-rpc.publicnode.com",
    "https://rpc.sepolia.org",
    "https://sepolia.gateway.tenderly.co",
]
rpcs = [r for r in rpcs if r]  # remove empty

def call_rpc(rpc, contract, wallet_addr):
    data = {
        "jsonrpc": "2.0", "id": 1, "method": "eth_call",
        "params": [{"to": contract,
                    "data": "0x70a08231000000000000000000000000" + wallet_addr[2:].lower()},
                   "latest"]
    }
    req = urllib.request.Request(
        rpc, data=json.dumps(data).encode(),
        headers={"Content-Type": "application/json", "User-Agent": "Mozilla/5.0"})
    with urllib.request.urlopen(req, timeout=15) as r:
        return json.load(r)

for contract, symbol, decimals in tokens:
    raw = None
    for rpc in rpcs:
        try:
            resp = call_rpc(rpc, contract, wallet)
            raw = int(resp["result"], 16)
            break
        except Exception:
            continue
    if raw is None:
        print(f"{symbol}: RPC_ERROR")
    elif raw == 0:
        pass  # skip zero balances
    else:
        print(f"{symbol}: {raw / 10**decimals:.6f}".rstrip('0').rstrip('.'))
PY
```

Only print tokens with balance > 0 to keep output clean.

Fallback RPC strategy:
1. Try `ETH_RPC_URL` if configured
2. Retry against public endpoints for the same chain
3. Only report a balance once an RPC call succeeds
4. If all RPCs fail, say the chain query failed — do **not** guess

Public Sepolia fallbacks: `https://ethereum-sepolia-rpc.publicnode.com`, `https://rpc.sepolia.org`, `https://sepolia.gateway.tenderly.co`

### 10. Set Approval Policy

Each wallet has a single USD-denominated approval policy. Token amounts are converted to USD at request time using real-time prices: native coins (ETH, SOL, BNB, POL, AVAX) via CoinGecko, stablecoins (USDC/USDT/DAI/BUSD) pegged to $1, ERC-20 tokens via CoinGecko Token Price API (17 EVM chains), and Solana SPL tokens via Jupiter Price API as fallback. Check native/stablecoin prices via `GET /api/prices`.

```bash
curl -s -X PUT "${TEE_WALLET_API_URL}/api/wallets/<id>/policy" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "threshold_usd": "<USD amount, e.g. 100>",
    "enabled": true,
    "daily_limit_usd": "<optional: max USD spend per UTC day, e.g. 5000>"
  }'
```

- `threshold_usd`: single transaction above this USD value requires Passkey approval
- `daily_limit_usd` (optional): cumulative USD spend per UTC calendar day; if exceeded the transfer is **hard-blocked** (no approval path)
- One policy per wallet — covers all currencies (ETH, SOL, tokens)

Ask user for the threshold amount if not specified. If they also want a daily cap, ask for `daily_limit_usd`.

**When called with an API key**, the policy change is **not applied immediately** — it creates a pending approval request (HTTP 202) that the wallet owner must approve via Passkey:

```json
{ "success": true, "pending": true, "approval_id": 42, "message": "Policy change submitted for approval" }
```

After receiving a 202 response, tell the user:
> 🔐 **Policy change submitted** (Approval ID: {approval_id})
> **Threshold:** ${threshold_usd} USD
> **Daily limit:** ${daily_limit_usd or "—"} USD
>
> The wallet owner must approve this change via the Web UI before it takes effect.
> [**→ Approve Policy Change**]({TEE_WALLET_API_URL}/#/approve/{approval_id})

Then start **background approval polling** (Section 12) and tell the user they can continue working.

**When called with a Passkey session** (Web UI), the policy is applied immediately and returns HTTP 200.

### 10.1. Check Daily Spend

Query how much USD has been spent today (UTC) against the wallet's daily limit:

```bash
curl -s "${TEE_WALLET_API_URL}/api/wallets/<id>/daily-spent" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

Response:
```json
{
  "success": true,
  "wallet_id": "<uuid>",
  "spent_usd": 150.00,
  "daily_limit_usd": 5000.00,
  "remaining_usd": 4850.00,
  "resets_at": "2026-03-26T00:00:00Z"
}
```

- `spent_usd`: total USD spent today (UTC)
- `daily_limit_usd`: the configured daily limit (0 if no policy set)
- `remaining_usd`: how much budget remains today
- `resets_at`: when the daily counter resets (next UTC midnight)

Use this to proactively check remaining budget before large transfers.

### 11. View Pending Approvals

```bash
curl -s "${TEE_WALLET_API_URL}/api/approvals/pending" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

Show: wallet, amount, currency, created time, expiry, approval link.

### 12. Approval Polling Flow (Background)

Use this whenever:
- A `/sign`, `/transfer`, or `/contract-call` response has `"status":"pending_approval"`, **or**
- A `/approve-token` or `/revoke-approval` response has `"status":"pending_approval"`, **or**
- A `/wrap-sol` or `/unwrap-sol` response has `"status":"pending_approval"`, **or**
- A `PUT /policy` response returns HTTP 202 (`"pending": true`), **or**
- A `POST /contracts` response returns HTTP 202 (`"pending": true`), **or**
- A `POST /addressbook` or `PUT /addressbook/:id` response returns HTTP 202 (`"pending": true`)

**Step 1 — Immediately show the summary:**

For transfer/sign:
> 🔐 **Approval required** (ID: {approval_id})
> **From:** `{from}`  →  **To:** `{to}`
> **Amount:** {amount} {currency}
> **Memo:** {memo or "—"}
> **Expires in:** 30 minutes
> [**→ Approve with Passkey**]({TEE_WALLET_API_URL}/#/approve/{approval_id})

For policy change:
> 🔐 **Policy change pending approval** (ID: {approval_id})
> **Threshold:** ${threshold_usd} USD
> **Daily limit:** ${daily_limit_usd or "—"} USD
> [**→ Approve with Passkey**]({TEE_WALLET_API_URL}/#/approve/{approval_id})

**Step 2 — Launch background polling script** using Bash with `run_in_background: true`:

```bash
APPROVAL_ID=<approval_id>
for i in $(seq 1 100); do
  sleep 15
  RESULT=$(curl -s "${TEE_WALLET_API_URL}/api/approvals/${APPROVAL_ID}" \
    -H "Authorization: Bearer ${TEE_WALLET_API_KEY}")
  STATUS=$(echo "$RESULT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status',''))" 2>/dev/null)
  if [ "$STATUS" = "approved" ] || [ "$STATUS" = "rejected" ] || [ "$STATUS" = "expired" ]; then
    echo "$RESULT"
    exit 0
  fi
done
echo '{"status":"timeout","message":"Polling timed out after 25 minutes"}'
```

**Step 3 — Tell the user polling is running in the background:**

> I'm monitoring this approval in the background. You can continue working — I'll notify you as soon as the approval is resolved.

Do NOT block the conversation. The user can ask other questions, run other commands, etc. while polling runs.

**Step 4 — When the background task completes**, you will be automatically notified. Parse the result and show the appropriate message based on `approval_type`:

Transfer / sign (`approval_type` is `"transfer"` or `"sign"`):
- `"status":"approved"` with `"tx_hash"` → show success + explorer link (same format as Section 5)
- `"status":"approved"` without `tx_hash` → show signature (sign-only requests)

Policy change (`approval_type` is `"policy_change"`):
- `"status":"approved"` → "✅ Policy applied. Transactions above ${threshold_usd} USD now require Passkey approval."

Contract call (`approval_type` is `"contract_call"`):
- `"status":"approved"` with `"tx_hash"` → show success + explorer link

Wrap SOL (`approval_type` is `"wrap_sol"`):
- `"status":"approved"` with `"tx_hash"` → show "✅ Wrapped SOL — `{tx_hash}`" + Solscan link

Unwrap SOL (`approval_type` is `"unwrap_sol"`):
- `"status":"approved"` with `"tx_hash"` → show "✅ Unwrapped wSOL — `{tx_hash}`" + Solscan link

Contract whitelist add (`approval_type` is `"contract_add"`):
- `"status":"approved"` → "✅ Contract `{contract_address}` ({symbol}) has been added to the whitelist. Transfers using this contract are now available."

Contract whitelist update (`approval_type` is `"contract_update"`):
- `"status":"approved"` → "✅ Contract whitelist entry updated."

Address book add (`approval_type` is `"addressbook_add"`):
- `"status":"approved"` → "✅ Address book entry added: {nickname} on {chain}."

Address book update (`approval_type` is `"addressbook_update"`):
- `"status":"approved"` → "✅ Address book entry updated."

All types:
- `"status":"rejected"` → "🚫 Approval rejected. No action was taken."
- `"status":"expired"` → "⏰ Approval expired. Please try again."
- `"status":"timeout"` → "⚠️ Approval is taking longer than expected. Please check the Web UI."

### 13. View Operation History (Audit Log)

Users can view a history of all their past operations.

```bash
curl -s "${TEE_WALLET_API_URL}/api/audit/logs?page=1&limit=20" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

Optional query parameters:
- `page` (default: 1), `limit` (default: 20, max: 100)
- `action` — filter by action type (see below)
- `wallet_id` — filter by wallet

**Action types:**

| Action | Description |
|--------|-------------|
| `login` | Passkey login |
| `wallet_create` | Wallet created |
| `wallet_delete` | Wallet deleted |
| `transfer` | Transfer sent or pending approval |
| `sign` | Message signed or pending approval |
| `policy_update` | Approval policy set or pending |
| `approval_approve` | Approval request approved |
| `approval_reject` | Approval request rejected |
| `contract_add` | Contract/mint/program added to whitelist or pending approval |
| `addressbook_add` | Address book entry added or pending approval |
| `addressbook_update` | Address book entry updated or pending approval |
| `addressbook_delete` | Address book entry deleted |
| `wrap_sol` | SOL wrapped into wSOL |
| `unwrap_sol` | wSOL unwrapped back to SOL |
| `apikey_generate` | API key generated |
| `apikey_revoke` | API key revoked |

**Present as a list:**
> 📋 **Operation History**
>
> • `{time}` — {action label} ({status}) · {auth_mode} · {details summary}
> • …

Show status as ✅ for `success`, ⏳ for `pending`, ❌ for `failed`.

## Quick Commands

Users can type short slash commands for common operations. Parse the intent and route to the appropriate operation.

| Command | Action | Details |
|---------|--------|---------|
| `/start` | Onboarding | Run the full onboarding flow: check config → register account → create first wallet → set up security. See Onboarding Flow. |
| `/test` | Guided test | Walk through all core features: balance → faucet → transfer → policy → approval → whitelist. See Guided Test Flow. |
| `/transfer 0.1 ETH to 0xabc...` | Transfer | Parse amount, currency, recipient (address or nickname). Auto-select wallet by chain. See Section 5/6/8. |
| `/balance` | Show all balances | List all wallets with native + token balances. See Section 9. |
| `/balance eth` | Show one wallet | Filter by chain or label. |
| `/wallets` | List wallets | Show all wallets with address, chain, status. See Section 2. |
| `/approve` | List pending approvals | Show all pending approval requests. See Section 12. |
| `/approve 123456` | Process one approval | Show details for a specific approval ID. Direct user to approve/reject. |
| `/whitelist` | List whitelisted contracts | Show contracts for all wallets. See Section 7. |
| `/whitelist 0xabc...` | Add contract to whitelist | Submit whitelist request with contract address. Ask for symbol/decimals if not provided. Include explorer link. |
| `/contacts` | List address book | Show all saved contacts. See Section 8. |
| `/contacts alice` | Lookup contact | Show address book entries for "alice". |
| `/contacts add alice 0xabc... ethereum` | Add contact | Save address with nickname, chain auto-detected or specified. |
| `/policy` | Show current policy | Display threshold, daily limit, and today's spend. See Section 10. |
| `/policy 100` | Set threshold to $100 | Set single-tx threshold. Ask about daily limit if not specified. |
| `/spent` | Today's spend | Show daily USD spend, remaining budget, reset time. See Section 10.1. |
| `/prices` | Current prices | Show ETH, SOL, BNB, POL, AVAX, and stablecoin prices. |
| `/chains` | Available chains | List all supported chains (built-in + custom). |
| `/call 0xabc... method(args)` | Contract call | Submit contract call. Contract must be whitelisted. API Key requires approval. See Section 7.2. |

**Parsing rules:**
- Commands are case-insensitive (`/Transfer` = `/transfer`)
- Arguments are positional and flexible — natural language also works (e.g., "send 0.1 ETH to 0xabc" = `/transfer 0.1 ETH to 0xabc`)
- If required info is missing, ask the user (e.g., `/transfer 100` → "Which currency and to what address?")
- If multiple wallets match, ask user to pick one

## Error Handling

Map common API errors to user-friendly messages:

| Error contains | User-facing message |
|---|---|
| `insufficient funds` | ❌ Insufficient ETH balance. Check your balance (including ~0.0005 ETH for gas). |
| `daily spend limit exceeded` | ❌ Daily USD spend limit reached. Limit resets at UTC midnight. |
| `contract not whitelisted` | ❌ This token contract/program/mint isn't whitelisted. Request approval via API key (`POST /contracts`) or open Web UI → Wallets → Contracts tab → Add to Whitelist. |
| `wallet is not ready` | ⏳ Wallet is still being created. Wait a moment and try again. |
| `invalid API key` | ❌ Invalid API key. Check `TEE_WALLET_API_KEY` in your environment. |
| `approval has expired` | ⏰ The approval window expired (30 min). Please initiate the transfer again. |
| `pending_approval` on policy | 🔐 Policy change is pending Passkey approval. Share the approval link with the wallet owner. |
| any other error | Show the raw error message and suggest checking the API URL and key. |

## Rules

1. Never display or ask for private keys — they don't exist outside TEE hardware
2. **No chat confirmation needed** for transfers — the backend approval policy is the safety net. Small amounts go through directly; large amounts trigger Passkey hardware approval automatically
3. When creating ETH wallets, tell the user it may take 1-2 minutes
4. Present addresses in their native format (0x... for ETH, base58 for Solana)
5. **Always use Smart Wallet Selection** — never ask for wallet ID directly
6. When showing balances, include the currency symbol and label (e.g. `ETH balance`, `USDC balance`)
7. Approval (approve/reject actions) can ONLY be done through the Web UI — each approval requires fresh hardware Passkey authentication at the moment of approval (not just a session token)
8. If an API call fails, map to a user-friendly error (see Error Handling above)
9. **ERC-20**: always verify the contract is whitelisted before sending; if not, propose adding it via API key (`POST /contracts`) or direct user to Web UI for instant approval
10. **ERC-20 amounts**: the `amount` field is in human-readable token units (e.g. `100` for 100 USDC), NOT raw wei
11. **ERC-20 `token` field is mandatory**: sending without `token` sends native ETH — always include `"token":{"contract":"...","symbol":"...","decimals":...}`
12. **Never confuse native balance with token balance**: `/balance` is for ETH/SOL only; use `eth_call balanceOf` for ERC-20
13. **Use RPC fallback for token checks**: retry multiple RPC endpoints before reporting unavailable
14. **Always include explorer link** after a successful transfer
15. **Background approval polling**: always use `run_in_background` for approval polling so the user can continue working. Never block the conversation waiting for approval
16. **Follow Smart Wallet Selection rules at all times**: refresh `/api/wallets` before account-wide views, never report balances for deleted wallets, hide raw wallet ids in normal UX, and interpret numeric references as list indices unless the user explicitly says `id=...` (see Smart Wallet Selection section for full details).
17. **Policy changes via API key always need approval**: `PUT /policy` with an API key returns 202 and creates a pending approval — always follow the Approval Polling Flow (Section 12) and share the approval link with the wallet owner. Policies are USD-denominated — one per wallet, covering all currencies.
18. **Contract whitelist proposals via API key**: `POST /contracts` with an API key returns 202 — follow the Approval Polling Flow (Section 12, `contract_add` type) and share the approval link. The passkey owner must approve before the contract can be used.
19. **Approve/reject is hardware-protected**: each approve or reject action requires a fresh hardware Passkey assertion at that moment — a stolen session token alone cannot approve. The Web UI handles this automatically.
20. **Audit log available**: users can check their operation history via `GET /api/audit/logs` (Section 13).
21. **Token list for balances**: when checking balances, only query tokens from the current wallet's own whitelist. If the whitelist is empty, only show the native balance.
22. **Contract calls**: use `/contract-call` for general smart contract interactions. The contract must be whitelisted first. Use `/call-read` for read-only queries (no approval needed).
23. **All contract operations via API Key require Passkey approval**: contract calls, token approvals, and revocations initiated via API Key always require Passkey confirmation. Passkey sessions execute directly.
24. **Use convenience endpoints**: prefer `/approve-token` and `/revoke-approval` over raw `/contract-call` for token approvals — they handle ABI encoding automatically.
25. **Contract whitelist is address-only**: the whitelist controls which contract addresses can be called. All callable methods on a whitelisted contract are available.
26. **SPL token transfer**: use `/transfer` with the `token` field (same as ERC-20). The token mint must be whitelisted. The backend creates the recipient's ATA automatically if needed.
27. **Solana program calls**: use `/contract-call` with `accounts` (array of `{pubkey, is_signer, is_writable}`) and `data` (hex-encoded instruction data) instead of `func_sig`/`args`. The program ID must be whitelisted. All program calls via API Key require Passkey approval.
28. **Wrap/Unwrap SOL**: use `/wrap-sol` (with `amount`) to convert native SOL to wSOL, and `/unwrap-sol` (no body params) to close the wSOL ATA and recover all SOL. Both endpoints follow the same approval/polling flow as transfers.
29. **Solana explorer links**: use `https://solscan.io/tx/{hash}` for mainnet and `https://solscan.io/tx/{hash}?cluster=devnet` for devnet.
30. **Address book nickname transfers**: when the user says "send to alice", pass the nickname directly in the `to` field of `/transfer` — the backend resolves it. If the nickname isn't found for that chain, the API returns 400.
31. **Address book proposals via API key**: `POST /addressbook` and `PUT /addressbook/:id` with an API key return 202 — follow the Approval Polling Flow (Section 12, `addressbook_add` / `addressbook_update` type).
30. **Dynamic chain list**: never hardcode chain names. Always call `GET /api/chains` to discover available chains (including user-added custom EVM chains). Custom chains have `"custom": true` in the response.
31. **All contract ops need Passkey via API Key**: contract calls, token approvals, and revocations via API Key always require Passkey approval — there is no auto-approve bypass. Use `GET /api/wallets/:id/daily-spent` to check remaining daily budget before large transfers.
32. **USD prices**: call `GET /api/prices` to get current native coin (ETH/SOL/BNB/POL/AVAX) and stablecoin prices. For ERC-20 token transfers, the wallet automatically looks up prices via CoinGecko Token Price API (17 EVM chains) by contract address. For Solana SPL tokens, it falls back to Jupiter Price API. Unknown tokens (no price from any source) require Passkey approval for transfers (fail-closed).
