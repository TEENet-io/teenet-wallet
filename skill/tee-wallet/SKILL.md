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

- `TEE_WALLET_API_URL`: The wallet service URL (e.g. `https://wallet.example.com`)
- `TEE_WALLET_API_KEY`: Your API key (starts with `ocw_`)

RPC URLs are configured in the wallet service's `chains.json` file (or via the `CHAINS_FILE` env var on the server), not as client-side environment variables. The wallet service handles all blockchain RPC communication internally.

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

Present wallets in a clear user-facing list with:
- list index (`1`, `2`, `3`, ...)
- Label
- Chain
- Address
- Status

Do **not** show the raw wallet `id` (UUID) by default in normal chat responses. Keep it
internal and use it only for API calls or debugging.

Mark wallets with status `creating` as ⏳ and `error` as ❌.

### 3. Get Wallet Details

```bash
curl -s "${TEE_WALLET_API_URL}/api/wallets/<id>" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

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
    "label": "<optional label>",
    "allowed_methods": "<optional: comma-separated method names, e.g. transfer,approve>",
    "auto_approve": false
  }'
```

A 202 response means the request is pending approval:
```json
{ "success": true, "pending": true, "approval_id": 7, "message": "Contract whitelist request submitted for approval" }
```

After receiving a 202 response, tell the user:
> 📋 **Contract whitelist request submitted** (Approval ID: {approval_id})
> **Contract:** `{contract_address}` ({symbol})
>
> The wallet owner must approve this via the Web UI before it can be used for ERC-20 transfers.
> [**→ Approve Request**]({TEE_WALLET_API_URL}/#/approve/{approval_id})

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
    "auto_approve": false,
    "allowed_methods": "transfer,balanceOf"
  }'
```
Only include the fields you want to change. A 202 response means the update is pending approval — follow the Approval Polling Flow (Section 12, `contract_update` type).

**Why removing requires Passkey but adding/updating can be proposed by API key**: An API key can only *propose* changes — the human wallet owner with hardware security must still approve. Removal is always Passkey-only since it's a more sensitive operation (accidentally removing could block legitimate transfers).

### 7.1. Contract Whitelist Fields

| Field | Required | Description |
|-------|----------|-------------|
| `contract_address` | Yes | EVM contract address (`0x…`, lowercase) **or** Solana mint/program address (base58) |
| `symbol` | No | Token symbol (e.g. USDC) |
| `decimals` | No | Token decimals (e.g. 6 for USDC, 18 for WETH, 9 for most SPL tokens) |
| `label` | No | Human-readable label |
| `allowed_methods` | No | Comma-separated method names (EVM: e.g. `transfer,approve`; Solana programs: leave empty to allow all instructions). Empty = all methods allowed |
| `auto_approve` | No | If `true`, API key can execute calls to this contract/program without Passkey approval (except high-risk methods). Default: `false`. Also applies to Solana program calls |

**High-risk methods** (always require Passkey approval regardless of `auto_approve`):
`approve`, `increaseAllowance`, `setApprovalForAll`, `transferFrom`, `safeTransferFrom`

> Note: For Solana programs, `auto_approve: true` enables API-key-initiated program calls without Passkey approval (except when the instruction itself triggers the wallet's transfer approval policy).

### 7.2. General Contract Call

Use when the user wants to call any smart contract function (EVM) or invoke a Solana program instruction. This endpoint has **three-layer security**:
1. Contract/program must be whitelisted
2. Method must be in `allowed_methods` (if configured; Solana programs typically leave this empty)
3. High-risk EVM methods always require Passkey approval; `auto_approve` applies to Solana programs too

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

The program must be added to the whitelist before calling (same API as Section 7). `auto_approve: true` on the whitelist entry allows API-key calls without Passkey.

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

> ⚠️ `approve` is a **high-risk method** — always requires Passkey approval, even if `auto_approve` is enabled on the contract.

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

> ⚠️ Also a high-risk method — requires Passkey approval.

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

### 8. Delete Wallet

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

**Step 2 — Build the global token list** (for Ethereum wallets):

The whitelist controls *sending*, not *receiving*. Any wallet can hold tokens that aren't on its own whitelist. To avoid missing balances, collect the **union of whitelisted contracts across all wallets on the same chain**, then query every target wallet against that global list.

```bash
# 1. Fetch all wallets
curl -s "${TEE_WALLET_API_URL}/api/wallets" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"

# 2. For each wallet id, fetch its contracts (run in parallel)
curl -s "${TEE_WALLET_API_URL}/api/wallets/<id>/contracts" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

Deduplicate by `contract_address` to get the global token list. Then for each token in the list, query the target wallet's on-chain balance (see Section 9.1).

> ⚠️ **Never skip this step because a wallet's own whitelist is empty.** The whitelist only gates sending — a wallet can hold any token. Always use the global list.

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
    # Fill from the global token list (union of all wallet whitelists)
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

Each wallet can have one policy **per currency** (ETH, USDC, SOL, etc.).

```bash
curl -s -X PUT "${TEE_WALLET_API_URL}/api/wallets/<id>/policy" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "threshold_amount": "<amount>",
    "currency": "<ETH|USDC|SOL|…>",
    "enabled": true,
    "daily_limit": "<optional: max total spend per UTC day>"
  }'
```

- `threshold_amount`: single transaction above this amount requires Passkey approval
- `daily_limit` (optional): cumulative spend per UTC calendar day; if this would be exceeded the transfer is **hard-blocked** (no approval path)
- Run the command once per currency to configure each policy independently

Ask user for the threshold amount if not specified. If they also want a daily cap, ask for `daily_limit`.

**When called with an API key**, the policy change is **not applied immediately** — it creates a pending approval request (HTTP 202) that the wallet owner must approve via Passkey:

```json
{ "success": true, "pending": true, "approval_id": 42, "message": "Policy change submitted for approval" }
```

After receiving a 202 response, tell the user:
> 🔐 **Policy change submitted** (Approval ID: {approval_id})
> **Currency:** {currency} · **New threshold:** {threshold_amount} {currency}
> **Daily limit:** {daily_limit or "—"}
>
> The wallet owner must approve this change via the Web UI before it takes effect.
> [**→ Approve Policy Change**]({TEE_WALLET_API_URL}/#/approve/{approval_id})

Then poll `GET /api/approvals/{approval_id}` every 15 seconds until `status` is `approved` or `rejected`:
- `approved` → "✅ Policy applied. Transfers above {threshold} {currency} now require Passkey approval."
- `rejected` → "🚫 Policy change rejected. No changes were made."

**When called with a Passkey session** (Web UI), the policy is applied immediately and returns HTTP 200.

### 11. View Pending Approvals

```bash
curl -s "${TEE_WALLET_API_URL}/api/approvals/pending" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

Show: wallet, amount, currency, created time, expiry, approval link.

### 12. Approval Polling Flow

Use this whenever:
- A `/sign`, `/transfer`, or `/contract-call` response has `"status":"pending_approval"`, **or**
- A `/approve-token` or `/revoke-approval` response has `"status":"pending_approval"`, **or**
- A `/wrap-sol` or `/unwrap-sol` response has `"status":"pending_approval"`, **or**
- A `PUT /policy` response returns HTTP 202 (`"pending": true`), **or**
- A `POST /contracts` response returns HTTP 202 (`"pending": true`)

**1. Immediately show the summary:**

For transfer/sign:
> 🔐 **Approval required** (ID: {approval_id})
> **From:** `{from}`  →  **To:** `{to}`
> **Amount:** {amount} {currency}
> **Memo:** {memo or "—"}
> **Expires in:** 30 minutes
> [**→ Approve with Passkey**]({TEE_WALLET_API_URL}/#/approve/{approval_id})

For policy change:
> 🔐 **Policy change pending approval** (ID: {approval_id})
> **Currency:** {currency} · **New threshold:** {threshold_amount}
> **Daily limit:** {daily_limit or "—"}
> [**→ Approve with Passkey**]({TEE_WALLET_API_URL}/#/approve/{approval_id})

**2. Poll every 15 seconds** until resolved or 25 minutes elapsed:

```bash
curl -s "${TEE_WALLET_API_URL}/api/approvals/<approval_id>" \
  -H "Authorization: Bearer ${TEE_WALLET_API_KEY}"
```

After each poll, show progress:
> ⏳ Waiting for approval… (~{N} min remaining)

**3. Handle result by `approval_type`:**

Transfer / sign (`approval_type` is `"transfer"` or `"sign"`):
- `"status":"approved"` with `"tx_hash"` → show success + explorer link (same format as Section 5)
- `"status":"approved"` without `tx_hash` → show signature (sign-only requests)

Policy change (`approval_type` is `"policy_change"`):
- `"status":"approved"` → "✅ Policy applied. {currency} transfers above {threshold} now require Passkey approval."

Contract call (`approval_type` is `"contract_call"`):
- `"status":"approved"` with `"tx_hash"` → show success + explorer link

Wrap SOL (`approval_type` is `"wrap_sol"`):
- `"status":"approved"` with `"tx_hash"` → show "✅ Wrapped SOL — `{tx_hash}`" + Solscan link

Unwrap SOL (`approval_type` is `"unwrap_sol"`):
- `"status":"approved"` with `"tx_hash"` → show "✅ Unwrapped wSOL — `{tx_hash}`" + Solscan link

Contract whitelist add (`approval_type` is `"contract_add"`):
- `"status":"approved"` → "✅ Contract `{contract_address}` ({symbol}) has been added to the whitelist. ERC-20 transfers using this contract are now available."

Contract whitelist update (`approval_type` is `"contract_update"`):
- `"status":"approved"` → "✅ Contract whitelist entry updated."

All types:
- `"status":"rejected"` → "🚫 Approval rejected. No action was taken."
- `"status":"expired"` → "⏰ Approval expired. Please try again."
- After 25 min with no result → stop polling: "⚠️ Approval is taking longer than expected. Please check the Web UI."

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

## Error Handling

Map common API errors to user-friendly messages:

| Error contains | User-facing message |
|---|---|
| `insufficient funds` | ❌ Insufficient ETH balance. Check your balance (including ~0.0005 ETH for gas). |
| `daily spend limit exceeded` | ❌ Daily {currency} spend limit reached. Limit resets at UTC midnight. |
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
15. **Poll with countdown**: when waiting for approval, show remaining time on each poll update
16. **Follow Smart Wallet Selection rules at all times**: refresh `/api/wallets` before account-wide views, never report balances for deleted wallets, hide raw wallet ids in normal UX, and interpret numeric references as list indices unless the user explicitly says `id=...` (see Smart Wallet Selection section for full details).
17. **Policy changes via API key always need approval**: `PUT /policy` with an API key returns 202 and creates a pending approval — always follow the Approval Polling Flow (Section 12) and share the approval link with the wallet owner.
18. **Contract whitelist proposals via API key**: `POST /contracts` with an API key returns 202 — follow the Approval Polling Flow (Section 12, `contract_add` type) and share the approval link. The passkey owner must approve before the contract can be used.
19. **Approve/reject is hardware-protected**: each approve or reject action requires a fresh hardware Passkey assertion at that moment — a stolen session token alone cannot approve. The Web UI handles this automatically.
20. **Audit log available**: users can check their operation history via `GET /api/audit/logs` (Section 13).
21. **Global token list for balances**: when checking balances, always collect the union of whitelisted contracts across all wallets on the same chain. Apply this global list when querying any wallet — the whitelist gates sending, not holding. Never skip token queries because a specific wallet's whitelist is empty.
22. **Contract calls**: use `/contract-call` for general smart contract interactions. The contract must be whitelisted first. Use `/call-read` for read-only queries (no approval needed).
23. **High-risk methods always need Passkey**: `approve`, `increaseAllowance`, `setApprovalForAll`, `transferFrom`, `safeTransferFrom` — even if the contract has `auto_approve: true`.
24. **Use convenience endpoints**: prefer `/approve-token` and `/revoke-approval` over raw `/contract-call` for token approvals — they handle ABI encoding automatically.
25. **Method restrictions**: if a contract's `allowed_methods` is set (e.g. `transfer,approve`), only those methods can be called. Empty = all methods allowed.
26. **SPL token transfer**: use `/transfer` with the `token` field (same as ERC-20). The token mint must be whitelisted. The backend creates the recipient's ATA automatically if needed.
27. **Solana program calls**: use `/contract-call` with `accounts` (array of `{pubkey, is_signer, is_writable}`) and `data` (hex-encoded instruction data) instead of `func_sig`/`args`. The program ID must be whitelisted. `auto_approve` on the whitelist entry controls whether API-key calls require Passkey.
28. **Wrap/Unwrap SOL**: use `/wrap-sol` (with `amount`) to convert native SOL to wSOL, and `/unwrap-sol` (no body params) to close the wSOL ATA and recover all SOL. Both endpoints follow the same approval/polling flow as transfers.
29. **Solana explorer links**: use `https://solscan.io/tx/{hash}` for mainnet and `https://solscan.io/tx/{hash}?cluster=devnet` for devnet.
30. **Dynamic chain list**: never hardcode chain names. Always call `GET /api/chains` to discover available chains (including user-added custom EVM chains). Custom chains have `"custom": true` in the response.
