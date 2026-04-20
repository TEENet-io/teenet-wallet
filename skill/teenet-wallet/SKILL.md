---
name: teenet-wallet
description: "Manage crypto wallets secured by TEE. Use when user asks to create wallet, check balance, send crypto, or manage crypto assets. Supports Ethereum and Solana."
metadata:
  openclaw:
    emoji: "🔐"
    requires:
      env:
        - TEENET_WALLET_API_URL
        - TEENET_WALLET_API_KEY
      anyBins:
        - python3
        - curl
    primaryEnv: TEENET_WALLET_API_KEY
---

# TEE Wallet Skill

**CRITICAL: You MUST announce every action to the user BEFORE executing it. Never run commands silently. Show results and explain next steps after every operation. This rule applies at ALL times regardless of conversation length.**

You manage crypto wallets backed by TEE (Trusted Execution Environment) hardware security.
Private keys are distributed across TEE nodes via threshold cryptography — they never exist
as a whole outside secure hardware.

## Configuration

- `TEENET_WALLET_API_URL`: The wallet service URL (required — no default)
- `TEENET_WALLET_API_KEY`: Your API key (starts with `ocw_`)

RPC URLs are configured in the wallet service's `chains.json` file (or via the `CHAINS_FILE` env var on the server), not as client-side environment variables. The wallet service handles all blockchain RPC communication internally.

## Onboarding Flow

When a user interacts with the wallet skill for the first time (no prior wallet context in the conversation), run through this flow automatically. **You MUST tell the user what you are doing at every step** — announce each step before running it, show the result, and explain what happens next.

### Step 0 — Check environment variables

Before making any API calls, verify both required environment variables are set.

**If `TEENET_WALLET_API_URL` is missing**, stop and ask the user to set it — there is no default. Tell them:
> ⚙️ **`TEENET_WALLET_API_URL` is not configured.**
>
> Set it to the wallet service URL you were given (for example, the deployed instance URL from your administrator), then try again.

**If `TEENET_WALLET_API_KEY` is missing**, stop and tell the user:
> 🔑 **`TEENET_WALLET_API_KEY` is not configured.**
>
> Set it to your wallet API key (starts with `ocw_`), then try again. You should have received this key when you generated it in the wallet Web UI.

Once both variables are set, continue to Step 1.

### Step 1 — Verify connectivity

Tell the user:
> 🔗 **Checking wallet service connection...**

```bash
curl -s "${TEENET_WALLET_API_URL}/api/health"
```

On success, tell the user:
> ✅ **Connected to wallet service** at `${TEENET_WALLET_API_URL}`

If the request fails or `status` is not `ok`, tell the user:
> ❌ Cannot reach the wallet service at `${TEENET_WALLET_API_URL}`. Check that the URL is correct and the service is running.

If a subsequent authenticated call returns `invalid API key`, tell the user:
> ❌ API key rejected. Check that `TEENET_WALLET_API_KEY` is correct (should start with `ocw_`).

### Step 2 — Check existing wallets

Tell the user:
> 📋 **Checking your wallets...**

```bash
curl -s "${TEENET_WALLET_API_URL}/api/wallets" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
```

**If the user already has wallets**, tell them:
> 👋 **Welcome back!** You have {N} wallet(s):
> 1. `0xabcd…1234` — My Main Wallet (Ethereum) ✅
> 2. `HN7c…Qx9f` — Trading (Solana) ✅
>
> What would you like to do?

Then stop onboarding — the user is already set up.

**If no wallets exist**, tell the user and continue to Step 3.

### Step 3 — Discover available chains

Tell the user:
> 🔍 **Looking up available chains...**

```bash
curl -s "${TEENET_WALLET_API_URL}/api/chains"
```

Then present the results and ask the user to pick one:
> 🆕 **No wallets yet — let's create your first one!**
>
> Available chains:
> **EVM:** Ethereum (ETH) · Sepolia (ETH) · Base Sepolia (ETH) · …
> **Solana:** Solana (SOL) · Solana Devnet (SOL)
>
> Which chain would you like to start with?

If the user is unsure, recommend a testnet:
> 💡 **Tip:** Try **Sepolia** (Ethereum testnet) or **Solana Devnet** to experiment with free test tokens before using real funds.

Then **wait for the user to choose** before continuing.

### Step 4 — Create first wallet

Tell the user what you're doing:
> ⏳ **Creating your {chain} wallet...** This may take 1–2 minutes for EVM chains (ECDSA key generation across TEE nodes). Solana wallets are instant.

Create the wallet per Section 1 (Create Wallet). After success, show the result.

### Step 5 — Recommend next steps

After the wallet is created successfully, tell the user:
> 🛡️ **Your wallet is ready! Recommended next steps:**
> 1. **Fund your wallet** — send {currency} to `{address}`
> 2. **Set an approval policy** — protect large transfers with a USD threshold (e.g. `/policy 100`)
> 3. **Whitelist tokens** — add token contracts you plan to use (see Section 6)
>
> 💡 Run `/test` to get free test tokens and walk through all features step by step.

For **Solana Devnet**, also mention [`https://faucet.solana.com`](https://faucet.solana.com). For test **USDC**, mention [`https://faucet.circle.com`](https://faucet.circle.com).

### Guided Test Flow

When the user runs `/test` or asks to test the wallet, walk them through these steps **interactively**. The user must have at least one wallet on a **testnet** (Sepolia, Base Sepolia, or Solana Devnet) — if not, create one first.

**IMPORTANT: For EVERY step:**
1. **BEFORE**: explain what the step does and why
2. **AFTER**: show the result immediately
3. **When approval is needed**: show the result + approval link together, so user sees the previous step's result while waiting for approval

Never skip showing results. Every step gets output. Never leave the user wondering what happened.

---

**Steps 1-4 — Basic Tests**

> **Step 1: Check wallet balance**
> ✅ **Result:** Balance **{amount} ETH**
>
> **Step 2: Get test tokens from faucet (wait 15s for confirmation, skip if balance is enough)**
> ✅ **Result:** Received **{amount} ETH** — [**View transaction**]({explorer}/tx/{tx_hash_S2})
>
> **Step 3: Create a second wallet on the same chain**
> This wallet serves as the transfer recipient for all subsequent tests (self-transfers are blocked by the backend).
> ✅ **Result:** Second wallet created — `{address_2}`
>
> **Step 4: Send 0.0001 ETH to the second wallet to test TEE signing**
> ✅ **Result:** TEE signing successful — [**View transaction**]({explorer}/tx/{tx_hash_S4})
>
> **Step 5: Set $1 USD approval threshold**
> 🔐 **Result:** Needs approval! Approval ID: {approval_id}
> 👉 → [Approve $1 threshold policy]({TEENET_WALLET_API_URL}/#/approve/{approval_id})

After user approves:
> **Step 5: Set $1 USD approval threshold**
> ✅ **Result:** Approval policy set! Threshold: **$1 USD**
>
> **Step 6: Send 0.0001 ETH to second wallet (below $1, no approval needed)** ⚠️ Note: 0.0001 not 0.001
> ✅ **Result:** Transfer successful! Amount: **0.0001 ETH** (~$0.20) — [**View transaction**]({explorer}/tx/{tx_hash_S6})
>
> **Step 7: Send 0.001 ETH to second wallet (above $1, needs approval)** ⚠️ Note: 0.001 not 0.0001
> 🔐 **Result:** Needs approval! Approval ID: {approval_id}
> 👉 → [Approve this 0.001 ETH transfer]({TEENET_WALLET_API_URL}/#/approve/{approval_id})

After user approves Step 7:
> **Step 7: Send 0.001 ETH to second wallet (above $1, needs approval)** ⚠️ Note: 0.001 not 0.0001
> ✅ **Result:** Transfer approved! TX: {tx_hash_S7} — [**View transaction**]({explorer}/tx/{tx_hash_S7})
>
> **Step 8: Add USDC to whitelist**
> 🔐 **Result:** Needs approval! Approval ID: {approval_id}
> 👉 → [Approve adding USDC to whitelist]({TEENET_WALLET_API_URL}/#/approve/{approval_id})

After user approves Step 8:
> **Step 8: Add USDC to whitelist**
> ✅ **Result:** USDC added to whitelist! Contract: `0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238`
> 💡 Get test USDC from [Circle Faucet](https://faucet.circle.com)

---

**Completion**

> 🎉 **All tests passed! Wallet is fully functional.**
>
> 1. ✅ Balance check
> 2. ✅ Faucet tokens received
> 3. ✅ Second wallet created (transfer recipient)
> 4. ✅ TEE distributed signing
> 5. ✅ Approval policy ($1 threshold)
> 6. ✅ Small transfer (no approval)
> 7. ✅ Large transfer (Passkey approval)
> 8. ✅ Token whitelist
>
> Type `/wallets` to see wallets or `/balance` to check balances.

### When to skip onboarding

Skip the full flow and go directly to the requested operation if:
- The user gives a specific command (e.g. `/balance`, `/transfer 0.1 ETH to 0x...`)
- The conversation already has wallet context from earlier messages
- The user explicitly asks to skip setup

### If the user has no API key yet

The agent talks to the wallet via `TEENET_WALLET_API_KEY`, which the user generates in the wallet Web UI **after** registering an account. New accounts are created in the Web UI through a 3-step flow: **enter email → submit the 6-digit verification code emailed to them → register a Passkey**. If the user says they haven't created an account yet, point them to the wallet Web UI, walk them through that flow, and ask them to come back with the generated API key.

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
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
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
curl -s "${TEENET_WALLET_API_URL}/api/wallets" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
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
curl -s "${TEENET_WALLET_API_URL}/api/wallets/<id>" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
```

### 3.1 Rename Wallet

```bash
curl -s -X PATCH "${TEENET_WALLET_API_URL}/api/wallets/<id>" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"label":"<new label>"}'
```

No approval needed. Works with API Key or Passkey session.

### 4. Send Crypto (Transfer)

When user asks to send/transfer crypto, call the `/transfer` endpoint.
The **backend constructs the transaction, signs it via TEE, and broadcasts it** — no scripts needed.

**No chat confirmation needed** — the backend enforces approval policies. Just send the request directly. If the amount exceeds the threshold, the backend returns `pending_approval` and the user approves via Passkey hardware.

**Optional pre-check** (recommended for ETH transfers > 0.01 ETH): query native balance first.
If `balance < amount + estimated_gas (0.0005 ETH buffer)`, warn the user before sending.

```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets/<id>/transfer" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "<recipient_address_or_nickname>",
    "amount": "<amount>",
    "memo": "<optional memo>"
  }'
```

The `to` field accepts raw addresses or address book nicknames (backend resolves).

**If response has `"status":"completed"`**: show the user:
> ✅ **Transaction sent**
> **Hash:** `{tx_hash}`
> **Chain:** {chain} · **Amount:** {amount} {currency}
> **To:** `{to}`
> [View on Explorer]({explorer_link})

Explorer links: use `{chain_explorer}/tx/{hash}` — Etherscan for Ethereum, Solscan for Solana, Basescan for Base, etc. Append `?cluster=devnet` for Solana devnet. For contract/address links, use `/address/{addr}` (EVM) or `/account/{addr}` (Solana).

**If response has `"status":"pending_approval"`**: follow the **Approval Polling Flow** (Section 12).

### 5. ERC-20 Token Transfer (Ethereum) / SPL Token Transfer (Solana)

Use this when the user asks to send a token (ERC-20 on Ethereum, or SPL token on Solana — e.g. USDC, WETH, USDT).

> ⚠️ **CRITICAL**: When sending tokens you MUST include the `token` field in the request body.
> Omitting `token` will send **native ETH/SOL** instead — a completely different transaction that costs
> real funds and cannot be reversed. Always double-check that your curl `-d` payload contains `"token": {...}`.

**Step 1 — Ensure the contract/mint is whitelisted** (see Section 6):
```bash
curl -s "${TEENET_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
```
If the contract/mint is not in the list, you can propose adding it via API key (creates a pending approval — see Section 6):

For Ethereum ERC-20:
```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"contract_address":"<0x...>","symbol":"<SYMBOL>","decimals":<N>}'
```

For Solana SPL tokens, use the token mint address (base58):
```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"contract_address":"<mint_address_base58>","symbol":"<SYMBOL>","decimals":<N>}'
```
Then follow the approval polling flow (Section 12, `contract_add` type). Alternatively, direct the user to add it immediately via Web UI (Passkey required):
> ⚠ The contract/mint `…` is not yet whitelisted. Requesting approval to add it… (or open Web UI → Contracts tab → Add to Whitelist for instant approval)

**Step 2 — Call `/transfer` with the `token` field** (no chat confirmation needed — backend enforces approval policies):
```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets/<id>/transfer" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
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

**Response handling** is identical to native transfer (Section 4) — include explorer link on success.

**Common testnet token addresses** (for `/test` flow and user convenience):

| Chain | Token | Contract | Decimals |
|-------|-------|----------|----------|
| Sepolia | USDC | `0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238` | 6 |
| Base Sepolia | USDC | `0x036CbD53842c5426634e7929541eC2318f3dCF7e` | 6 |

### 6. Manage Contract Whitelist

The contract whitelist is a **security gate**: only pre-registered contracts/programs/mints can be called via `/transfer` or `/contract-call`. This applies equally to:
- **Ethereum**: ERC-20 contract addresses (`0x…`)
- **Solana**: SPL token mint addresses (base58) and program IDs (base58)

> **Scope:** entries are scoped per **user + chain**, not per wallet. All wallets you own on the same chain share one whitelist, and deleting a wallet does **not** remove its whitelist entries.

Removing entries requires **Passkey hardware authentication**. Adding can be done by either:
- **Passkey session** (Web UI): applied immediately
- **API key**: creates a pending approval (HTTP 202) that the Passkey owner must approve

**List whitelisted contracts** (API key works for reading):
```bash
curl -s "${TEENET_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
```

**Add a contract via API key** (creates pending approval):
```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract_address": "<0x...>",
    "symbol": "<e.g. USDC>",
    "decimals": <e.g. 6>,
    "label": "<optional label>"
  }'
```

A 202 response means pending approval — tell the user with the contract address, symbol, explorer link, and approval link `{TEENET_WALLET_API_URL}/#/approve/{approval_id}`.

Then start **background approval polling** (Section 12). Once `approved`, the contract/mint/program is whitelisted and token transfers or program calls can proceed.

**Add a contract via Passkey** (Web UI, applied immediately):
> Web UI → Wallets → select wallet → Contracts tab → Add to Whitelist.
> Fields: contract address (0x…), symbol (e.g. USDC), decimals (e.g. 6), optional label.

**Remove a contract** (Passkey session only):
> Web UI → Wallets → wallet → Contracts tab → ✕ button next to the contract.

**Update a contract via API key** (creates pending approval):
```bash
curl -s -X PUT "${TEENET_WALLET_API_URL}/api/wallets/<id>/contracts/<cid>" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "label": "Updated label",
    "symbol": "USDC",
    "decimals": 6
  }'
```
Only include the fields you want to change (`label`, `symbol`, `decimals`). A 202 response means the update is pending approval — follow the Approval Polling Flow (Section 12, `contract_update` type).

**Whitelist fields:** `contract_address` (required, `0x…` or base58), `symbol` (optional), `decimals` (optional, e.g. 6 for USDC, 18 for WETH, 9 for most SPL), `label` (optional).

### 6.2. General Contract Call

Use when the user wants to call any smart contract function (EVM) or invoke a Solana program instruction. Security model:
1. Contract/program must be whitelisted
2. All contract operations via API Key require Passkey approval; Passkey sessions execute directly

**EVM (Ethereum) — use `func_sig` and `args`:**
```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets/<id>/contract-call" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
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

**Important for Uniswap V3 / SwapRouter02 swaps:**
- `exactInputSingle` on Uniswap V3 routers must use the **tuple ABI** form, not a flattened parameter list:
  - ✅ Correct: `exactInputSingle((address,address,uint24,address,uint256,uint256,uint160))`
  - ❌ Wrong: `exactInputSingle(address,address,uint24,address,uint256,uint256,uint160)`
- The correct selector for the tuple form is **`0x04e45aaf`**.
- If the selector differs, the function signature or argument shape is probably wrong.
- Pass the tuple as a **single array item** in JSON args, e.g.:
```json
{
  "contract": "0x94cC0AaC535CCDB3C01d6787D6413C739ae12bc4",
  "func_sig": "exactInputSingle((address,address,uint24,address,uint256,uint256,uint160))",
  "args": [[
    "0x4200000000000000000000000000000000000006",
    "0x036cbd53842c5426634e7929541ec2318f3dcf7e",
    100,
    "0xYourWallet",
    500000000000000,
    200000,
    0
  ]]
}
```

**Swap workflow (EVM, recommended):**
1. Confirm the **real contract ABI, parameter order, and selector** before sending `/contract-call`.
   - Do not guess flattened vs tuple forms.
   - If the contract was used successfully before, compare the on-chain input selector with your intended `func_sig`.
   - For routers and DeFi contracts, verify the exact function signature from official source/interfaces when possible.
2. Ensure the **input token contract is whitelisted**.
3. Ensure the **router contract is whitelisted**.
4. Check **token balance** on chain.
5. Check **allowance** for the router.
6. Use `call-read` / RPC quote tools first (for example QuoterV2 on Uniswap).
7. Only then submit the real `/contract-call` swap.

**Do not test swaps with 100% of balance/allowance.**
- Leave headroom. Start with **50% or less** of the available token amount.
- A full-balance test can fail with transfer helper errors even when balance and allowance appear correct.

**Common swap failures (EVM):**
- `Too little received` → usually `amountOutMinimum` too high, quote stale, or parameters in the wrong position.
- `STF` → token `transferFrom` failed; check balance, allowance, and whether you are trying to use the full balance.
- HTTP `502` on `/contract-call` often means **`eth_estimateGas` reverted on chain**, not that the backend crashed.

**All contract-call errors return structured fields** (`stage`, `contract`, `func_sig`, `selector`, `revert_reason`, `wallet_id`, `chain`) — see Error Handling section for the full list.

**Solana — use `accounts` and `data` instead of `func_sig`/`args`:**
```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets/<id>/contract-call" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
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

The program must be added to the whitelist before calling (same API as Section 6). All contract/program operations via API Key require Passkey approval.

**Response:** Same as transfer — either `"status":"completed"` with `tx_hash`, or `"status":"pending_approval"` with approval link.

### 6.3. Approve Token Allowance (Convenience)

Shorthand for calling `approve(spender, amount)` on an ERC-20 contract:

```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets/<id>/approve-token" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "<token contract 0x...>",
    "spender": "<spender address 0x...>",
    "amount": "<amount in token units, e.g. 1000>",
    "decimals": 6
  }'
```

> ⚠️ All contract operations via API Key require Passkey approval. Passkey sessions execute directly.

### 6.4. Revoke Token Approval (Convenience)

Shorthand for calling `approve(spender, 0)` to revoke a previously granted allowance:

```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets/<id>/revoke-approval" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "<token contract 0x...>",
    "spender": "<spender address 0x...>"
  }'
```

> ⚠️ Requires Passkey approval when called via API Key.

### 6.5. Wrap SOL (Solana)

Wraps native SOL into wSOL (Wrapped SOL SPL token). Use when the user asks to wrap SOL or convert SOL to wSOL (required by many DeFi protocols on Solana).

The backend creates the wSOL Associated Token Account (ATA) automatically if it does not yet exist.

```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets/<id>/wrap-sol" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"amount": "<SOL amount, e.g. 0.1>"}'
```

On success, show tx hash + Solscan link. If `pending_approval`, follow Section 12.

### 6.6. Unwrap SOL (Solana)

Closes the wSOL ATA and returns all wSOL back to native SOL. Use when the user asks to unwrap wSOL or convert wSOL back to SOL. The entire wSOL balance in the ATA is unwrapped in one operation.

```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets/<id>/unwrap-sol" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{}'
```

No body parameters — closes the entire wSOL ATA. On success, show tx hash + Solscan link. If `pending_approval`, follow Section 12.

### 6.7. Read-Only Contract Call

Query contract state without signing or sending a transaction. No gas, no approval needed.

**For swap prep, prefer read-only checks before sending a real trade:**
- Token `balanceOf(wallet)`
- Token `allowance(wallet, router)`
- Pool / quote endpoints (for example Uniswap QuoterV2 on EVM)

**Practical rule for testnet swaps:**
- Quote first
- Set a conservative `amountOutMinimum` (for example 50%–80% of the quote)
- Do not assume testnet pool prices resemble mainnet

```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets/<id>/call-read" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "<0x...>",
    "func_sig": "<e.g. balanceOf(address)>",
    "args": ["<wallet_address>"]
  }'
```

Returns hex-encoded `result`. Use for querying allowances, `totalSupply()`, `name()`, `symbol()`, `decimals()`, etc. **Do NOT use for token balance checks** — use client-side RPC instead (Section 9.1).

### 7. Address Book

The address book lets users save frequently used addresses with nicknames. Users can then transfer by nickname instead of pasting raw addresses.

**Nicknames** are case-insensitive, stored lowercase. The same nickname can have different addresses on different chains (e.g. "alice" on Ethereum and "alice" on Solana).

**Security model:**
- **List / Read**: API Key or Passkey
- **Add / Update via API Key**: creates a pending approval (HTTP 202) that the Passkey owner must approve
- **Add / Update via Passkey**: applied immediately (requires fresh credential)
- **Delete**: Passkey only

**List address book entries:**
```bash
curl -s "${TEENET_WALLET_API_URL}/api/addressbook" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
```

Optional query params: `?nickname=alice`, `?chain=ethereum`

**Add an entry via API key** (creates pending approval):
```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/addressbook" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "nickname": "alice",
    "chain": "ethereum",
    "address": "0x1234567890123456789012345678901234567890",
    "memo": "Alice main wallet"
  }'
```

A 202 response means pending approval — follow Section 12.

**Update an entry via API key** (creates pending approval):
```bash
curl -s -X PUT "${TEENET_WALLET_API_URL}/api/addressbook/<id>" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"address": "0xnewaddress...", "memo": "updated memo"}'
```

Only include the fields you want to change (`nickname`, `address`, `memo`). A 202 response means the update is pending approval.

**Delete an entry** (Passkey only — via Web UI).

**Transfer by nickname:**

The `/transfer` endpoint accepts a nickname in the `to` field. If the value doesn't look like a raw address, the backend resolves it from the address book for the wallet's chain:

```bash
curl -s -X POST "${TEENET_WALLET_API_URL}/api/wallets/<id>/transfer" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"to": "alice", "amount": "0.1"}'
```

If the nickname is not found for the wallet's chain, the API returns 400 with an error message.

> 💡 **Tip:** When a user says "send 0.1 ETH to alice", use the nickname directly in the `to` field — the backend resolves it.

**Address book fields:** `nickname` (required, lowercase alphanumeric with `_`/`-`, max 100 chars), `chain` (required), `address` (required, `0x…` or base58), `memo` (optional, max 256 chars).

### 8. Delete Wallet

**Do NOT call the delete API.** Wallet deletion requires Passkey hardware authentication and is irreversible. Tell the user to do it themselves in the Web UI:

> Wallet deletion requires Passkey verification and can't be done through the API key. Please delete it in the [Web UI]({TEENET_WALLET_API_URL}) → Wallets → select wallet → Delete.

### 9. Check Balance

When the user asks for a wallet's balance, **show both native and token balances together** in one response.

**Step 1 — Native balance:**
```bash
curl -s "${TEENET_WALLET_API_URL}/api/wallets/<id>/balance" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
```

> ⚠️ `/balance` returns the wallet's **native gas token** only (ETH / SOL). Never present this as a token balance.

**Step 2 — Fetch the token whitelist** (for Ethereum wallets):

Query the contract whitelist for this wallet's chain to get its token list. The whitelist is **scoped per user + chain** — all wallets on the same chain share the same whitelist, and deleting a wallet does **not** remove these entries. Only tokens in the whitelist are checked for balances.

```bash
curl -s "${TEENET_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
```

Use the returned contracts as the token list for on-chain balance queries (see Section 9.1). If the whitelist is empty, only the native balance is shown.

**Present all balances together:**
> 💼 **Wallet** `0xabcd…1234` (Ethereum)
> ├ ETH: **0.482 ETH**
> ├ USDC: **250.00 USDC**
> └ USDT: **100.00 USDT**

**After a transfer**: the balance reflects the latest confirmed block. Wait ~15 seconds before checking:
```bash
sleep 15 && curl -s "${TEENET_WALLET_API_URL}/api/wallets/<id>/balance" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
```

### 9.1. Check ERC-20 Token Balances On-Chain (Client-Side)

Query token balances **directly via public RPCs** — do NOT use `/call-read` for balance checks, as that routes through the backend RPC and can hit rate limits.

**Do this whenever the user asks for a token balance. Do not rely on `/balance`.**

For each token in the wallet's whitelist (`GET /api/wallets/<id>/contracts`), call `balanceOf(address)` via `eth_call`:

```bash
curl -s -X POST "<rpc_url>" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0","id":1,"method":"eth_call",
    "params":[{"to":"<contract_address>","data":"0x70a08231000000000000000000000000<wallet_address_no_0x>"},"latest"]
  }'
```

The `result` is a hex-encoded uint256. Convert to human-readable amount using the token's `decimals`. Only show tokens with balance > 0.

Use free public RPCs for each chain (e.g. `publicnode.com`, `llamarpc.com`, or the chain's official RPC). Try multiple endpoints with fallback. For custom chains, check `GET /api/chains` for `rpc_url`. If all RPCs fail, report the query failed — do **not** guess.

### 10. Set Approval Policy

Each wallet has a single USD-denominated approval policy. Token amounts are converted to USD at request time using real-time prices: native coins (ETH, SOL, BNB, POL, AVAX) via CoinGecko, stablecoins (USDC/USDT/DAI/BUSD) pegged to $1, ERC-20 tokens via CoinGecko Token Price API (17 EVM chains), and Solana SPL tokens via Jupiter Price API as fallback. Check native/stablecoin prices via `GET /api/prices`.

```bash
curl -s -X PUT "${TEENET_WALLET_API_URL}/api/wallets/<id>/policy" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}" \
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

**Via API key**: returns HTTP 202 with `approval_id` — show summary with approval link and start **background approval polling** (Section 12).

**Via Passkey session**: applied immediately (HTTP 200).

### 10.1. Check Daily Spend

Query how much USD has been spent today (UTC) against the wallet's daily limit:

```bash
curl -s "${TEENET_WALLET_API_URL}/api/wallets/<id>/daily-spent" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
```

Returns `spent_usd`, `daily_limit_usd`, `remaining_usd`, and `resets_at` (next UTC midnight). Check proactively before large transfers.

### 11. View Pending Approvals

```bash
curl -s "${TEENET_WALLET_API_URL}/api/approvals/pending" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
```

Show: wallet, amount, currency, created time, expiry, approval link.

### 12. Approval Flow (No Polling)

When an API call returns `pending_approval`, **do NOT start background polling**. Instead:
1. Show the approval summary with link
2. Ask the user to approve via the link
3. Wait for the user to come back and tell you they've approved
4. Then check the result and continue

**Display the approval link with descriptive anchor text:**

For transfer/sign:
> 🔐 **Approval required** (ID: {approval_id})
> **From:** `{from}`  →  **To:** `{to}`
> **Amount:** {amount} {currency}
> **Memo:** {memo or "—"}
> **Expires in:** 30 minutes
> 👉 → [Approve this {amount} {currency} transfer]({TEENET_WALLET_API_URL}/#/approve/{approval_id})

For policy change:
> 🔐 **Approval required** (ID: {approval_id})
> **Threshold:** ${threshold_usd} USD
> 👉 → [Approve policy change]({TEENET_WALLET_API_URL}/#/approve/{approval_id})

For contract/token whitelist:
> 🔐 **Approval required** (ID: {approval_id})
> **Contract:** `{contract_address}` ({symbol})
> 👉 → [Approve adding {symbol} to whitelist]({TEENET_WALLET_API_URL}/#/approve/{approval_id})

**After showing the link, tell the user:**
> Please approve via the link above, then let me know when done.

**When the user says they've approved**, check the result:
```bash
curl -s "${TEENET_WALLET_API_URL}/api/approvals/${APPROVAL_ID}" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
```

Then act on the result:

| `status` | Action |
|----------|--------|
| `approved` + `tx_hash` | ✅ Done! Hash: `{tx_hash}` — [**View transaction**]({explorer}/tx/{tx_hash}) |
| `approved` (`policy_change`) | ✅ Policy active! Transfers above ${threshold_usd} USD now require Passkey approval |
| `approved` (`contract_add`) | ✅ Contract `{addr}` ({symbol}) whitelisted |
| `rejected` | 🚫 Approval rejected. No action was taken. |
| `expired` | ⏰ Approval expired. Please try again. |

### 13. View Operation History (Audit Log)

Users can view a history of all their past operations.

```bash
curl -s "${TEENET_WALLET_API_URL}/api/audit/logs?page=1&limit=20" \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
```

Optional query parameters:
- `page` (default: 1), `limit` (default: 20, max: 100)
- `action` — filter by action type (see below)
- `wallet_id` — filter by wallet

**Action filter values:** `login`, `wallet_create`, `wallet_delete`, `transfer`, `sign`, `policy_update`, `approval_approve`, `approval_reject`, `contract_add`, `addressbook_add`, `addressbook_update`, `addressbook_delete`, `wrap_sol`, `unwrap_sol`, `apikey_generate`, `apikey_revoke`

Show each entry with timestamp, action, status (✅/⏳/❌), auth mode, and details summary.

## Quick Commands

Users can type short slash commands for common operations. Parse the intent and route to the appropriate operation.

| Command | Section |
|---------|---------|
| `/start` | Onboarding Flow |
| `/test` | Guided Test Flow |
| `/transfer 0.1 ETH to 0xabc...` | §4/5 — amount, currency, recipient (address or nickname) |
| `/balance [chain]` | §9 — all wallets or filter by chain/label |
| `/wallets` | §2 — list with status |
| `/approve [id]` | §11 — list pending or show specific |
| `/whitelist [0xabc...]` | §6 — list or add contract |
| `/contacts [alice]` | §7 — list, lookup, or add |
| `/policy [100]` | §10 — show or set threshold |
| `/spent` | §10.1 — daily USD spend |
| `/prices` | `GET /api/prices` |
| `/chains` | `GET /api/chains` |
| `/call 0xabc... method(args)` | §6.2 — contract call |
| `/history [action]` | §13 — audit log |

Commands are case-insensitive. Natural language also works (e.g., "send 0.1 ETH to alice"). Missing info → ask the user.

## Error Handling

**All error responses include structured fields.** Beyond the `error` string, check these fields to diagnose issues:

| Field | Description |
|-------|-------------|
| `stage` | Which step failed: `build_tx`, `estimate_gas`, `signing`, `broadcast`, `key_generation`, `eth_call`, `balance_query`, `faucet_request`, `create_approval`, etc. |
| `wallet_id` | Which wallet was involved |
| `chain` | Which chain (e.g. `sepolia`, `base-sepolia`, `solana-devnet`) |
| `contract` | Contract/program address (for contract-call errors) |
| `method` / `func_sig` | Function called (for contract-call errors) |
| `selector` | 4-byte function selector (for EVM contract-call errors) |
| `revert_reason` | On-chain revert message if available (e.g. `execution reverted: STF`) |

**Use `stage` to determine what went wrong and what to try next:**

| `stage` | Meaning | What to do |
|---------|---------|------------|
| `build_tx` / `estimate_gas` | Transaction construction or gas estimation failed (often an on-chain revert) | Check `revert_reason`, verify contract args, check balance covers gas |
| `signing` | TEE distributed signing failed | Retry; if persistent, check TEE cluster health |
| `broadcast` | Signed tx rejected by chain RPC | Check nonce conflicts, gas price, or chain congestion |
| `key_generation` | Wallet key generation failed | Retry; ECDSA DKG can take 1-2 min |
| `eth_call` | Read-only call failed | Check contract address and function signature |
| `balance_query` | RPC balance query failed | Retry or try alternative RPC |
| `faucet_request` | Faucet service unreachable | Check if faucet is configured/running |
| `rebuild_tx` | Approval tx refresh failed | Original tx may be stale; create a new transfer |

**Common error strings:**

| Error contains | User-facing message |
|---|---|
| `insufficient funds` | Insufficient balance. Check your balance (including ~0.0005 ETH for gas). |
| `daily spend limit exceeded` | Daily USD spend limit reached. Limit resets at UTC midnight. |
| `contract not whitelisted` | This token contract/program/mint isn't whitelisted. Request approval via API key (`POST /contracts`) or open Web UI → Wallets → Contracts tab → Add to Whitelist. |
| `wallet is not ready` | Wallet is still being created. Wait a moment and try again. |
| `invalid API key` | Invalid API key. Check `TEENET_WALLET_API_KEY` in your environment. |
| `approval has expired` | The approval window expired (30 min). Please initiate the transfer again. |
| `pending_approval` on policy | Policy change is pending Passkey approval. Share the approval link with the wallet owner. |
| `nickname not found` | Nickname not in address book for this chain. Add via `/contacts`. |
| `execution reverted` | On-chain revert — check `revert_reason` for details (e.g. `STF` = transferFrom failed, check allowance/balance). |
| `nonce too low` | Transaction nonce conflict — retry the transfer (backend will fetch a fresh nonce). |
| `signing failed` | TEE signing error — retry; if persistent, the TEE cluster may be overloaded. |
| `broadcast failed` | Chain RPC rejected the transaction — check the error details for the specific reason. |
| any other error | Show the raw `error` message plus `stage` field, and suggest checking the API URL and key. |

## Explorer Links

| Chain | Explorer Base URL |
|-------|-------------------|
| Ethereum | `https://etherscan.io` |
| Optimism | `https://optimistic.etherscan.io` |
| Arbitrum | `https://arbiscan.io` |
| Base | `https://basescan.org` |
| Polygon | `https://polygonscan.com` |
| BSC | `https://bscscan.com` |
| Avalanche | `https://snowtrace.io` |
| Sepolia | `https://sepolia.etherscan.io` |
| Holesky | `https://holesky.etherscan.io` |
| Base Sepolia | `https://sepolia.basescan.org` |
| BSC Testnet | `https://testnet.bscscan.com` |
| Solana | `https://solscan.io` |
| Solana Devnet | `https://solscan.io` (append `?cluster=devnet`) |

- Transaction: `{explorer}/tx/{hash}`
- Address/Contract: `{explorer}/address/{addr}` (EVM) or `{explorer}/account/{addr}` (Solana)

## Faucet Links (Testnet)

| Chain | Faucet |
|-------|--------|
| Sepolia ETH | Built-in: `POST /api/faucet` with `wallet_id` |
| Base Sepolia ETH | Built-in: `POST /api/faucet` with `wallet_id` |
| Solana Devnet | [`https://faucet.solana.com`](https://faucet.solana.com) |
| Sepolia USDC | [`https://faucet.circle.com`](https://faucet.circle.com) |
| Base Sepolia USDC | [`https://faucet.circle.com`](https://faucet.circle.com) |

## Rules

These are global rules that override or supplement the per-section guidance above:

1. **Always narrate** — tell the user what you're doing before every action, show results after. When approval polling completes, immediately tell the user the result and continue to the next step — do NOT wait for the user to notify you
2. **Never display private keys** — they don't exist outside TEE hardware
3. **No chat confirmation for transfers** — backend approval policy is the safety net; don't double-confirm
4. **Smart Wallet Selection always** — never ask for wallet ID; use numbered list indices; re-fetch `/api/wallets` before account-wide views
5. **Token transfers MUST include `token` field** — omitting it sends native ETH/SOL instead (irreversible)
6. **Token balances: client-side RPC** — use public RPCs with `eth_call balanceOf`, NOT `/call-read`, to avoid backend rate limits (Section 9.1)
7. **No background polling** — when approval is needed, show the link and wait for the user to confirm they've approved. Do NOT start background polling scripts
8. **All API Key write operations need Passkey approval** — the backend returns 202; you MUST show the approval link and ask the user to confirm when done (Section 12)

9. **Approve/reject is hardware-only** — each action requires a fresh Passkey assertion via Web UI
10. **Dynamic chains** — never hardcode chain names; use `GET /api/chains`
11. **Always include explorer link** after successful transfers and contract operations
12. **Never call DELETE APIs** — all destructive operations (delete wallet, remove contract, delete address book entry, delete policy, delete account) require Passkey and cannot be done via API key. Direct the user to the [Web UI]({TEENET_WALLET_API_URL}) instead
