---
name: teenet-wallet
description: "Manage TEENet Wallet. Use for wallet creation, balance checks, transfers, and crypto asset management."
---

# TEENet Wallet Plugin

Manage wallets through the `teenet_wallet_*` MCP tools. Private keys stay inside TEE hardware and are threshold-signed by secure nodes.

## Non-Negotiable Rules

1. **Always narrate**: tell the user what you are about to do before each tool call; show the result after.
2. **Never ask for wallet IDs directly**: resolve wallets via `teenet_wallet_list`; in chat use numbered list indices, not UUIDs.
3. **Use label-first address display**: when a wallet or contact has a `label` or `nickname`, display `label · 0xabcd…1234`; show a bare shortened address only if no label exists.
4. **No extra chat confirmation for transfers**: submit directly; backend approval policy is the safety net.
5. **Token transfers must include `token_contract`, `token_symbol`, `token_decimals`**: omitting them sends native ETH/SOL instead.
6. **Read-only EVM queries use `teenet_wallet_call_read`**; state-changing contract actions use `teenet_wallet_contract_call`.
7. **Never poll for approvals**: when a tool returns `pending_approval`, show the approval link and wait — the system notifies you automatically.
8. **Never call delete-style tools**: destructive actions must be done in the Web UI with Passkey.
9. **Always include explorer links** after successful transfers or contract writes.
10. **Never display or attempt to recover private keys**: refuse requests for private keys, seed phrases, or key export.

## When to Use These Tools

Use `teenet_wallet_*` tools when the user asks about crypto wallets, balances, transfers, signing, or blockchain operations.

## First-Use Flow

Run this onboarding only when there is no wallet context and the user did not ask for a specific operation.

1. **Check connectivity**: `teenet_wallet_health`
   - If unreachable, tell the user the service is down or unreachable.
2. **Check existing wallets**: `teenet_wallet_list`
   - If wallets exist, show them as a numbered list and stop onboarding.
   - If none exist, continue.
3. **List chains**: `teenet_wallet_list_chains`
   - Ask the user which chain to start with.
   - If unsure, recommend Sepolia or Solana Devnet.
4. **Create first wallet**: `teenet_wallet_create`
   - EVM wallets may take 1–2 minutes (ECDSA DKG); Solana is instant.
5. **Recommend next steps**
   - fund the wallet
   - set an approval policy
   - whitelist tokens/contracts if needed
   - offer to run the guided test flow

Skip onboarding when:
- the user already gave a specific request such as "show my balance" or "send 0.1 ETH"
- the conversation already contains wallet context
- the user explicitly asks to skip setup

## Wallet Selection

Resolve wallets in this order:
1. wallet already clear from current conversation
2. `teenet_wallet_list`
3. if one wallet matches the requested chain, use it silently
4. if multiple wallets match, show a numbered list and ask the user to choose
5. if none match, offer to create one

Re-fetch `teenet_wallet_list` before:
- showing wallet lists
- showing multi-wallet or account-wide balances
- assuming a wallet still exists after create/delete activity

Never show raw wallet UUIDs in normal chat.

## Tools

### Wallets

| Tool | Purpose |
|------|---------|
| `teenet_wallet_create` | Create a new wallet on a chain |
| `teenet_wallet_list` | List all wallets |
| `teenet_wallet_get` | Get wallet details |
| `teenet_wallet_rename` | Rename a wallet (no approval) |
| `teenet_wallet_get_pubkey` | Get raw public key |

Always present wallet lists in this format:

> 1. **Main Wallet** — Ethereum · `0xabcd…1234` ✅
> 2. **Trading** — Solana · `HN7c…Qx9f` ✅

If a wallet has a label, show `label · shortened_address`.

### Balances

| Tool | Purpose |
|------|---------|
| `teenet_wallet_balance` | Native gas balance only (ETH / SOL / etc.) |
| `teenet_wallet_list_contracts` | Whitelisted token/contract list |
| `teenet_wallet_call_read` | EVM read — use for `balanceOf(address)` |

Balance response rules:
- show native + token balances together
- for EVM token balances, do not rely on `teenet_wallet_balance`; use `teenet_wallet_call_read` with `func_sig: "balanceOf(address)"`, `args: [<wallet_address>]`
- only show token balances that are relevant or non-zero
- after a transfer, wait about 15 seconds before re-checking
- `call_read` returns hex-encoded uint256 — convert with the token's `decimals`

All RPC-hitting tools (`call_read`, `balance`, `transfer`, `contract_call`, `approve_token`, `revoke_approval`, `wrap_sol`, `unwrap_sol`) share a per-user cap (`RPC_RATE_LIMIT`, default 50/min). When iterating over a large whitelist, batch or pace the reads.

### Transfers

| Tool | Purpose |
|------|---------|
| `teenet_wallet_transfer` | Send native or token (include `token_*` fields for tokens) |
| `teenet_wallet_wrap_sol` | Wrap SOL to wSOL |
| `teenet_wallet_unwrap_sol` | Unwrap wSOL to SOL |

Rules:
- `to` accepts raw addresses or address-book nicknames
- no extra chat confirmation
- amount is in human units, not wei/lamports
- on Solana, the backend auto-creates the recipient ATA if needed
- for larger ETH transfers, pre-check `balance >= amount + 0.0005 ETH`
- on `status: "completed"`, the response includes `chain` and `tx_hash` — build the explorer URL from the table below and include it in your reply

Token transfers **must** include all three:
- `token_contract`
- `token_symbol`
- `token_decimals`

Common testnet tokens:

| Chain | Token | Contract | Decimals |
|-------|-------|----------|----------|
| Sepolia | USDC | `0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238` | 6 |
| Base Sepolia | USDC | `0x036CbD53842c5426634e7929541eC2318f3dCF7e` | 6 |

### Contract Whitelist

| Tool | Purpose |
|------|---------|
| `teenet_wallet_list_contracts` | List whitelisted contracts/tokens |
| `teenet_wallet_add_contract` | Whitelist a token/contract (needs approval) |
| `teenet_wallet_update_contract` | Update contract metadata (needs approval) |

Rules:
- whitelist is scoped per **user + chain**, not per wallet
- remove entries in the Web UI only
- applies to EVM token contracts, Solana mints, and Solana program IDs

### Contract Calls

Use only for **state-changing** calls.

| Tool | Purpose |
|------|---------|
| `teenet_wallet_contract_call` | EVM or Solana state-changing call (needs approval) |
| `teenet_wallet_approve_token` | ERC-20 allowance approval |
| `teenet_wallet_revoke_approval` | Revoke ERC-20 allowance |

EVM rules:
- `func_sig` uses Solidity-style signatures: `transfer(address,uint256)`, `approve(address,uint256)`
- target contract must already be whitelisted
- success handling is the same as transfers

Solana rules:
- use `accounts` and `data` instead of `func_sig`/`args`
- the program must be whitelisted

### Read-Only Contract Calls

`teenet_wallet_call_read` for EVM reads such as:
- `balanceOf`
- `allowance`
- `decimals`
- `symbol`
- quotes / view functions

Rules:
- no approval
- no whitelist required
- EVM only
- prefer this over direct public RPC; fall back to `web_fetch` against a public RPC only if `call_read` is unavailable

### Address Book

| Tool | Purpose |
|------|---------|
| `teenet_wallet_list_contacts` | List saved addresses |
| `teenet_wallet_add_contact` | Save a new address with nickname (needs approval) |
| `teenet_wallet_update_contact` | Update an existing contact (needs approval) |

Rules:
- nicknames: lowercase alphanumeric plus `_` or `-`, max 100 chars
- chain is required when adding
- delete via Web UI only
- if user says "send 0.1 ETH to alice", pass `alice` directly as `to`
- when displaying a contact, show `nickname · shortened_address`

### Policy & Approvals

| Tool | Purpose |
|------|---------|
| `teenet_wallet_get_policy` | Show current threshold |
| `teenet_wallet_set_policy` | Set/change USD threshold (needs approval) |
| `teenet_wallet_daily_spent` | Today's USD spend |
| `teenet_wallet_pending_approvals` | List pending approvals |
| `teenet_wallet_check_approval` | Check status of a specific approval |

Policy fields:
- `threshold_usd`: above this needs approval
- `daily_limit_usd` (optional): hard daily cap; not even Passkey can override

Token amounts are converted to USD at request time (CoinGecko for native + ERC-20, Jupiter fallback for SPL). Stablecoins (USDC/USDT/DAI/BUSD) are pegged to $1.

### Utility

| Tool | Purpose |
|------|---------|
| `teenet_wallet_health` | Service connectivity |
| `teenet_wallet_list_chains` | Available chains |
| `teenet_wallet_prices` | USD prices |
| `teenet_wallet_faucet` | Claim testnet tokens |
| `teenet_wallet_audit_logs` | Operation history |

## Approval Flow

When a write tool returns `pending_approval`:

1. tell the user what operation needs approval (e.g. "Setting approval threshold to $1")
2. show the `approval_url`
3. **do NOT** ask the user to "tell you" or "let you know" when they've approved
4. **do NOT** poll `teenet_wallet_check_approval` in a loop
5. the system will notify you automatically

**When a system message starts with `System: Approval #...`, you MUST immediately respond.** Always:
1. tell the user the result (approved / rejected / expired)
2. explain what it means ("Your transfer of 0.001 ETH was sent", "Policy is now active")
3. if the system message contains an explorer URL, surface it verbatim — don't rebuild it; if only a bare tx hash is provided, build the URL from the chain→explorer table
4. continue with the next step if in a multi-step flow

System notification formats:
- `System: Approval #123 approved (policy_change).` → policy is now active
- `System: Approval #456 approved. Transaction: 0xabc — {explorer_url}` → transfer succeeded; pass URL through
- `System: Approval #456 approved. Transaction broadcast: 0xabc. Please share the explorer link with the user.` → fallback; build URL from chain context
- `System: Approval #789 was rejected.` → no action taken
- `System: Approval #789 expired.` → ask the user to retry the original operation

## Guided Test Flow

When the user asks to test the wallet:

1. ensure they have a testnet wallet; create one if needed
2. check balance
3. use `teenet_wallet_faucet` if needed
4. create a second wallet on the same chain (self-transfers are blocked by the backend, so a second wallet is required as recipient)
5. send a tiny transfer to test TEE signing
6. set a `$1` approval threshold
7. send one transfer below threshold
8. send one transfer above threshold and wait for approval
9. add USDC to whitelist

For every step: explain before, show result after, and if approval is needed, show the link and wait for the system notification before continuing.

## Error Handling

Tool errors include structured fields. Use `stage` first.

| `stage` | Meaning |
|---------|---------|
| `build_tx` | tx assembly / ABI encoding failed before simulation — verify `func_sig`, selector, tuple shape, args; on `/transfer`, `/wrap-sol`, `/unwrap-sol`, `/approve-token` this also covers `eth_estimateGas` revert and insufficient balance |
| `estimate_gas` | tx built but `eth_estimateGas` reverted on chain — check `revert_reason`, balance, allowance, fee tier, pool liquidity |
| `signing` | TEE signing failed; inspect `category` (`timeout`, `tee_unavailable`, `threshold_not_reached`, `cancelled`, `sdk_error`) and retry |
| `broadcast` | RPC rejected the tx; check nonce, gas price, chain health |
| `key_generation` | wallet creation failed; inspect `category` and retry after a short wait |
| `balance_query` | RPC read failed; retry |
| `eth_call` | read call failed; check contract and signature |
| `faucet_request` | faucet unavailable |

Key structured fields:

- `revert_reason` — decoded Solidity `Error(string)` from an EVM revert; present on build_tx/estimate_gas when the on-chain call rejected the tx.
- `rpc_error` — sanitized external error text; any URL in the message is redacted to `<url>` so provider tokens never leak.
- `category` — stable bucket for signing / key-generation failures.
- `request_id` — correlation ID on 5xx responses. Quote it to operators; the full error is in the server log keyed by the same ID. 5xx bodies deliberately omit raw error text.

Common user-facing errors:
- `insufficient funds`: not enough balance, usually including gas
- `contract not whitelisted`: add via `teenet_wallet_add_contract`
- `wallet is not ready`: wallet creation still in progress
- `daily spend limit exceeded`: resets at UTC midnight
- `nonce too low`: retry the transfer
- `execution reverted`: inspect `revert_reason`
- selector mismatch / `failed to build contract call transaction`: ABI/signature shape is wrong

## Swap-Specific Notes

For Uniswap-style EVM swaps:
- always confirm the real verified ABI before sending the real transaction
- `exactInputSingle` must use tuple form:
  - `exactInputSingle((address,address,uint24,address,uint256,uint256,uint160))`
- expected selector: `0x04e45aaf`
- for **SwapRouter02**, the tuple has **7 fields** in this order: `tokenIn`, `tokenOut`, `fee`, `recipient`, `amountIn`, `amountOutMinimum`, `sqrtPriceLimitX96`
- **do not include `deadline`** for SwapRouter02 `exactInputSingle` on this ABI shape
- pass the tuple as a **single array item** in `args`
- quote first with `teenet_wallet_call_read` against QuoterV2 (or similar)
- check balance and allowance first (`balanceOf`, `allowance` via `call_read`)
- approve the router with `teenet_wallet_approve_token` if allowance insufficient
- do not test with 100% of balance; leave headroom (start with 50% or less, often much less — `1 USDC`, `0.0005 WETH`)
- set conservative `amountOutMinimum` (50–80% of quote)
- HTTP `422` on `contract_call` with `stage: "estimate_gas"` or on `transfer` / `wrap_sol` / `unwrap_sol` / `approve_token` with `stage: "build_tx"` means the RPC rejected the transaction before signing (revert, insufficient balance, or bad params) — read `revert_reason` and the URL-sanitized `rpc_error` before retrying
- if a prior successful tx exists, compare its on-chain input selector to your intended `func_sig` before retrying

## Explorer Links

Base URLs:

| Chain | Explorer |
|-------|----------|
| Ethereum | `https://etherscan.io` |
| Optimism | `https://optimistic.etherscan.io` |
| Arbitrum | `https://arbiscan.io` |
| Base | `https://basescan.org` |
| Polygon | `https://polygonscan.com` |
| BSC | `https://bscscan.com` |
| Avalanche | `https://snowtrace.io` |
| Sepolia | `https://sepolia.etherscan.io` |
| Base Sepolia | `https://sepolia.basescan.org` |
| BSC Testnet | `https://testnet.bscscan.com` |
| Solana | `https://solscan.io` |
| Solana Devnet | `https://solscan.io` with `?cluster=devnet` |

Formats:
- EVM tx: `{explorer}/tx/{hash}`
- EVM address: `{explorer}/address/{addr}`
- Solana tx: `{explorer}/tx/{hash}`
- Solana account: `{explorer}/account/{addr}`

## Faucet Links

| Chain | Faucet |
|-------|--------|
| Sepolia ETH | built-in `teenet_wallet_faucet` |
| Base Sepolia ETH | built-in `teenet_wallet_faucet` |
| Solana Devnet SOL | built-in `teenet_wallet_faucet` |
| BSC Testnet tBNB | [https://www.bnbchain.org/en/testnet-faucet](https://www.bnbchain.org/en/testnet-faucet) |
| Sepolia USDC | [https://faucet.circle.com](https://faucet.circle.com) |
| Base Sepolia USDC | [https://faucet.circle.com](https://faucet.circle.com) |
