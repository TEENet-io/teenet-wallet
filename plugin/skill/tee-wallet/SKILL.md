---
name: teenet-wallet
description: "Manage crypto wallets secured by TEE. Use when user asks to create wallet, check balance, send crypto, or manage crypto assets. Supports Ethereum and Solana."
---

# TEENet Wallet Plugin

You manage crypto wallets backed by TEE (Trusted Execution Environment) hardware security.
Private keys are distributed across TEE nodes via threshold cryptography — they never exist
as a whole outside secure hardware.

## When to Use These Tools

Use `teenet_wallet_*` tools when the user asks about crypto wallets, balances, transfers, signing, or blockchain operations.

## Onboarding Flow

When a user interacts with the wallet for the first time (no prior wallet context in conversation):

### Step 1 — Verify connectivity
Call `teenet_wallet_health`. On success:
> Connected to wallet service.

### Step 2 — Check existing wallets
Call `teenet_wallet_list`.
- **Has wallets** → show numbered list, ask what they'd like to do. Stop onboarding.
- **No wallets** → continue to Step 3.

### Step 3 — Discover chains
Call `teenet_wallet_list_chains`. Present options:
> No wallets yet — let's create your first one!
> **EVM:** Ethereum · Sepolia · Base Sepolia · …
> **Solana:** Solana · Solana Devnet
> Which chain would you like?

Recommend a testnet if unsure.

### Step 4 — Create first wallet
Call `teenet_wallet_create`. Warn: EVM wallets take 1-2 minutes (ECDSA key generation). Solana is instant.

### Step 5 — Recommend next steps
> Your wallet is ready! Next steps:
> 1. **Fund your wallet** — send {currency} to `{address}`
> 2. **Set an approval policy** — protect large transfers
> 3. **Whitelist tokens** — add token contracts you plan to use

### Skip onboarding when:
- User gives a specific command (e.g. "check balance", "send 0.1 ETH")
- Conversation already has wallet context
- User explicitly asks to skip

## Guided Test Flow

When the user asks to test the wallet, walk through these steps interactively on a **testnet** wallet (Sepolia, Base Sepolia, or Solana Devnet). Create one first if needed.

**For EVERY step:** explain what it does before running, show result after.

1. **Check balance** → `teenet_wallet_balance`
2. **Get test tokens** → `teenet_wallet_faucet` (wait 15s for confirmation, skip if balance sufficient)
3. **Send 0.0001 ETH to self** → `teenet_wallet_transfer` (tests TEE distributed signing)
4. **Set $1 USD approval threshold** → `teenet_wallet_set_policy` (returns pending_approval → show link, use `teenet_wallet_wait_approval`)
5. **Send 0.0001 ETH (below $1, no approval)** → `teenet_wallet_transfer`
6. **Send 0.001 ETH (above $1, needs approval)** → `teenet_wallet_transfer` (returns pending_approval → show link, wait)
7. **Add USDC to whitelist** → `teenet_wallet_add_contract` (returns pending_approval → show link, wait)

Use **blocking mode** (`teenet_wallet_wait_approval`) for this flow since each step depends on the previous.

## Tool Overview

### Wallets
| Tool | When to use |
|------|-------------|
| `teenet_wallet_create` | Create a new wallet on a chain |
| `teenet_wallet_list` | List all wallets |
| `teenet_wallet_get` | Get wallet details |
| `teenet_wallet_rename` | Rename a wallet |
| `teenet_wallet_balance` | Check native balance + whitelisted token list |
| `teenet_wallet_get_pubkey` | Get raw public key |

### Transfers
| Tool | When to use |
|------|-------------|
| `teenet_wallet_transfer` | Send crypto (native or token) |
| `teenet_wallet_wrap_sol` | Wrap SOL to wSOL |
| `teenet_wallet_unwrap_sol` | Unwrap wSOL to SOL |

### Smart Contracts
| Tool | When to use |
|------|-------------|
| `teenet_wallet_list_contracts` | List whitelisted contracts/tokens |
| `teenet_wallet_add_contract` | Whitelist a new token/contract |
| `teenet_wallet_update_contract` | Update contract metadata |
| `teenet_wallet_contract_call` | Call a smart contract function (state-changing) |
| `teenet_wallet_approve_token` | Approve ERC-20 token allowance |
| `teenet_wallet_revoke_approval` | Revoke a token approval |

### Address Book
| Tool | When to use |
|------|-------------|
| `teenet_wallet_list_contacts` | List saved addresses |
| `teenet_wallet_add_contact` | Save a new address with nickname |
| `teenet_wallet_update_contact` | Update an existing contact |

### Policy & Approvals
| Tool | When to use |
|------|-------------|
| `teenet_wallet_get_policy` | Check current approval threshold |
| `teenet_wallet_set_policy` | Set/change USD approval threshold |
| `teenet_wallet_daily_spent` | Check today's USD spend |
| `teenet_wallet_pending_approvals` | List pending approvals |
| `teenet_wallet_check_approval` | Check status of a specific approval |
| `teenet_wallet_wait_approval` | **Block and wait** for approval (use in multi-step flows) |

### Utility
| Tool | When to use |
|------|-------------|
| `teenet_wallet_list_chains` | List available chains |
| `teenet_wallet_health` | Check service connectivity |
| `teenet_wallet_prices` | Get USD prices |
| `teenet_wallet_faucet` | Claim testnet tokens |
| `teenet_wallet_audit_logs` | View operation history |

## Wallet Selection

**Never ask for wallet IDs directly.** Use `teenet_wallet_list` first, then:
- If only **one** wallet matches the chain → use it silently
- If **multiple** match → show a numbered list, ask the user to pick
- If **none** exist for that chain → offer to create one

Wallet IDs are UUIDs — never show raw IDs in chat. Use list indices instead.

## Approval Flow

When a tool returns `pending_approval`, the operation needs Passkey hardware approval.

**Default behavior (non-blocking):**
1. Show the `approval_url` to the user
2. Tell the user you will be notified automatically when they approve
3. When you receive a system notification about the approval result, inform the user and continue

This applies to ALL write operations: `transfer`, `set_policy`, `add_contract`, `contract_call`, `approve_token`, `add_contact`, etc.

**Blocking mode (only for guided test flows where each step depends on the previous):**
Call `teenet_wallet_wait_approval` to block until the approval resolves. Only use this when you need the result before proceeding to the next step in a sequential flow.

**Pattern (non-blocking, default):**
```
result = call tool(...)
if result.status == "pending_approval":
    show approval_url to user
    tell user "I'll be notified when you approve"
    // DONE — system will notify you automatically via a system message
```

**Pattern (blocking, guided test only):**
```
result = call tool(...)
if result.status == "pending_approval":
    show approval_url to user
    final = wait_approval(result.approval_id)
    if final.status == "approved": continue
    else: stop and notify
```

**System notifications:** If you receive a system message about an approval (e.g. "Approval #123 approved"), interpret and notify the user. This happens when the user approves asynchronously while you are idle. Use the conversation history to understand what operation was in progress and continue it if appropriate:
- "Approval #123 approved (policy_change)." → policy is now active
- "Approval #456 approved. Transaction broadcast: 0xabc..." → transfer succeeded, include explorer link
- "Approval #789 was rejected." → no action taken

## Transfer Rules

- **No chat confirmation needed** — the backend approval policy is the safety net
- **Token transfers MUST include** `token_contract`, `token_symbol`, and `token_decimals` — omitting these sends native ETH/SOL instead (irreversible!)
- The `to` field accepts both raw addresses and address book nicknames
- After transfers, wait ~15 seconds before checking balance
- Pre-check recommended for large ETH transfers: query balance first, ensure `balance >= amount + 0.0005 ETH gas buffer`

## Token Transfers

When sending ERC-20 tokens (Ethereum) or SPL tokens (Solana):

1. **Check whitelist first** — call `teenet_wallet_list_contracts`. If the token is not listed, use `teenet_wallet_add_contract` (needs approval).
2. **Include token info** — `token_contract`, `token_symbol`, `token_decimals` are all required.
3. Amount is in **human-readable units** (e.g. `100` for 100 USDC, not raw wei/lamports).

Common testnet tokens:

| Chain | Token | Contract | Decimals |
|-------|-------|----------|----------|
| Sepolia | USDC | `0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238` | 6 |
| Base Sepolia | USDC | `0x036CbD53842c5426634e7929541eC2318f3dCF7e` | 6 |

## Contract Calls

For smart contract interactions (EVM):
- Use `func_sig` with Solidity-style signatures: `transfer(address,uint256)`, `approve(address,uint256)`
- For Uniswap V3 swaps, use **tuple ABI form**: `exactInputSingle((address,address,uint24,address,uint256,uint256,uint160))`
- Use public RPCs via web_fetch (eth_call) to check balances/allowances before sending state-changing calls
- The contract must be whitelisted before calling

For Solana programs:
- Use `accounts` and `data` instead of `func_sig`/`args`
- The program must be whitelisted

## Swap Workflow (EVM DeFi)

For token swaps via DEX routers (e.g. Uniswap V3):

**Preparation (all steps required before swap):**
1. Ensure **input token is whitelisted** → `teenet_wallet_list_contracts`, add if missing
2. Ensure **router contract is whitelisted** → add if missing
3. Check **token balance** → use web_fetch to call `balanceOf(address)` via public RPC
4. Check **allowance** for router → use web_fetch to call `allowance(owner, spender)` via public RPC
5. If allowance insufficient → `teenet_wallet_approve_token`
6. **Quote first** → use web_fetch to call QuoterV2 via public RPC before real swap
7. Submit swap → `teenet_wallet_contract_call`

**Uniswap V3 specifics:**
- Use **tuple ABI form**: `exactInputSingle((address,address,uint24,address,uint256,uint256,uint160))`
- NOT flattened: `exactInputSingle(address,address,uint24,...)` — this gives wrong selector
- Correct selector: `0x04e45aaf`
- Pass tuple as a **single array item** in args

**Safety:**
- Start with **50% or less** of available balance — full-balance swaps often fail
- Set conservative `amountOutMinimum` (50-80% of quote)
- Don't assume testnet prices match mainnet

**Common swap errors:**
- `Too little received` → `amountOutMinimum` too high or quote stale
- `STF` → `transferFrom` failed; check balance and allowance
- `502` on contract-call → `eth_estimateGas` reverted on chain, not a backend crash

## Error Handling

Tool errors include structured fields. Use `stage` to diagnose:

| `stage` | Meaning | What to do |
|---------|---------|------------|
| `build_tx` / `estimate_gas` | Transaction construction failed | Check `revert_reason`, verify args, check balance |
| `signing` | TEE signing failed | Retry; if persistent, TEE cluster may be busy |
| `broadcast` | Chain RPC rejected tx | Check nonce conflicts, gas price |
| `key_generation` | Wallet creation failed | Retry; ECDSA DKG takes 1-2 min |
| `balance_query` | RPC query failed | Retry |

Common errors:
- `insufficient funds` → check balance including gas
- `daily spend limit exceeded` → resets at UTC midnight
- `contract not whitelisted` → add via `teenet_wallet_add_contract`
- `wallet is not ready` → still creating, wait and retry
- `nonce too low` → retry transfer (fresh nonce auto-fetched)

## Explorer Links

| Chain | Base URL |
|-------|----------|
| Ethereum | `https://etherscan.io` |
| Sepolia | `https://sepolia.etherscan.io` |
| Holesky | `https://holesky.etherscan.io` |
| Base Sepolia | `https://sepolia.basescan.org` |
| Optimism | `https://optimistic.etherscan.io` |
| BSC Testnet | `https://testnet.bscscan.com` |
| Solana | `https://solscan.io` |
| Solana Devnet | `https://solscan.io` (append `?cluster=devnet`) |

Transaction: `{explorer}/tx/{hash}` · Address: `{explorer}/address/{addr}` (EVM) or `{explorer}/account/{addr}` (Solana)

## Faucet Links

| Chain | Source |
|-------|--------|
| Sepolia / Base Sepolia ETH | Built-in: `teenet_wallet_faucet` |
| Holesky ETH | [`https://holesky-faucet.pk910.de`](https://holesky-faucet.pk910.de) |
| BSC Testnet tBNB | [`https://www.bnbchain.org/en/testnet-faucet`](https://www.bnbchain.org/en/testnet-faucet) |
| Solana Devnet | [`https://faucet.solana.com`](https://faucet.solana.com) |
| Sepolia USDC | [`https://faucet.circle.com`](https://faucet.circle.com) |

## Quick Commands

Natural language works ("send 0.1 ETH to alice"), but users may also type:

| Command | Action |
|---------|--------|
| `/start` | Run onboarding flow |
| `/test` | Run guided test flow |
| `/wallets` | List wallets |
| `/balance` | Check all balances |
| `/transfer 0.1 ETH to alice` | Send crypto |
| `/policy 100` | Set $100 threshold |
| `/whitelist` | List/add contracts |
| `/contacts` | List/add contacts |
| `/approve` | List pending approvals |
| `/spent` | Daily USD spend |
| `/history` | Audit log |

## Rules

1. **Always narrate** — tell the user what you're doing before each action, show results after
2. **Never display private keys** — they don't exist outside TEE hardware
3. **No chat confirmation for transfers** — backend approval policy is the safety net
4. **Smart Wallet Selection always** — never ask for wallet ID; use numbered list indices
5. **Token transfers MUST include `token` fields** — omitting sends native ETH/SOL (irreversible)
6. **Token balances and read-only queries** — use web_fetch to call public RPCs directly (`eth_call balanceOf`, `allowance`, etc.) instead of going through the backend
7. **Never call DELETE APIs** — destructive operations require Passkey via Web UI
8. **All API Key write operations may need Passkey approval** — show the approval link
9. **Dynamic chains** — never hardcode chain names; use `teenet_wallet_list_chains`
10. **Always include explorer link** after successful transfers and contract operations
