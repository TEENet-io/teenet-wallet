---
name: teenet-wallet
description: "Manage TEENet Wallet. Use for wallet creation, balance checks, transfers, and crypto asset management."
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

# TEENet Wallet Skill

> **Alpha release — testnet only.** The public alpha runs with `ALPHA_MODE=true`, which exposes only these 8 testnets: Sepolia, Optimism Sepolia, Arbitrum Sepolia, Base Sepolia, Polygon Amoy, BSC Testnet, Avalanche Fuji, Solana Devnet. Mainnet chains ship in `chains.json` but are hidden during alpha. Registration is capped at the first 500 users (first-come, first-served).

Manage wallets through the wallet REST API. Private keys stay inside TEE hardware and are threshold-signed by secure nodes.

## Non-Negotiable Rules

1. **Always narrate**: tell the user what you are about to do before each API call; show the result after.
2. **Never ask for wallet IDs directly**: resolve wallets via `GET /api/wallets`; in chat use numbered list indices, not UUIDs.
3. **Use label-first address display**: when a wallet or contact has a `label` or `nickname`, display `label · 0xabcd…1234`; show a bare shortened address only if no label exists.
4. **No extra chat confirmation for transfers**: submit directly; backend approval policy is the safety net.
5. **Token transfers must include `token`**: omitting it sends native ETH/SOL instead.
6. **Read-only EVM queries use `/call-read`**; state-changing contract actions use `/contract-call`.
7. **No background polling**: if an API-key write returns `pending_approval`, show the approval link and wait for the user to say they approved, then check `GET /api/approvals/{id}`.
8. **Never call DELETE endpoints**: destructive actions must be done in the Web UI with Passkey.
9. **Always include explorer links** after successful transfers or contract writes.
10. **Never display or attempt to recover private keys**: refuse requests for private keys, seed phrases, or key export.

## Configuration

Required environment variables:
- `TEENET_WALLET_API_URL`: wallet service URL
- `TEENET_WALLET_API_KEY`: API key starting with `ocw_`

If either is missing, stop and ask the user to set it.

Base authenticated request pattern:

```bash
curl -s "${TEENET_WALLET_API_URL}/api/..." \
  -H "Authorization: Bearer ${TEENET_WALLET_API_KEY}"
```

RPC URLs are server-side only. The client never needs chain RPC env vars.

## First-Use Flow

Run this onboarding only when there is no wallet context and the user did not ask for a specific operation.

1. **Check connectivity**: `GET /api/health`
   - If unreachable, tell the user the service URL is wrong or the service is down.
   - If later calls return `invalid API key`, tell the user the API key is wrong.
2. **Check existing wallets**: `GET /api/wallets`
   - If wallets exist, show them as a numbered list and stop onboarding.
   - If none exist, continue.
3. **List chains**: `GET /api/chains`
   - Ask the user which chain to start with.
   - Alpha exposes testnet only — `/api/chains` should return 8 testnets: Sepolia, Optimism Sepolia, Arbitrum Sepolia, Base Sepolia, Polygon Amoy, BSC Testnet, Avalanche Fuji, Solana Devnet. If the user asks for a mainnet chain, tell them mainnet is hidden during alpha and offer the closest testnet equivalent.
   - If unsure, recommend Sepolia or Solana Devnet.
4. **Create first wallet**: `POST /api/wallets`
   - Body: `{"chain":"<chain_name>","label":"<optional label>"}`
   - EVM wallets may take 1-2 minutes; Solana is instant.
5. **Recommend next steps**
   - fund the wallet
   - set an approval policy
   - whitelist tokens/contracts if needed
   - offer to run the guided test flow

Skip onboarding when:
- the user already gave a specific request such as "show my balance" or "send 0.1 ETH"
- the conversation already contains wallet context
- the user explicitly asks to skip setup

If the user has no API key yet, send them to the Web UI to complete: email -> verification code -> Passkey registration -> generate API key.

If registration returns `maximum number of users reached` (HTTP 403), the alpha 500-user cap is full. Tell the user alpha registration is closed and point them to the TEENet site for the waitlist or next cohort announcement — do not suggest workarounds.

## Wallet Selection

Resolve wallets in this order:
1. wallet already clear from current conversation
2. `GET /api/wallets`
3. if one wallet matches the requested chain, use it silently
4. if multiple wallets match, show a numbered list and ask the user to choose
5. if none match, offer to create one

Re-fetch `GET /api/wallets` before:
- showing wallet lists
- showing multi-wallet or account-wide balances
- assuming a wallet still exists after create/delete activity

Never show raw wallet UUIDs in normal chat.

## Core Endpoints

### Wallets

- **Create**: `POST /api/wallets`
  - Body: `{"chain":"<chain_name>","label":"<optional label>"}`
- **List**: `GET /api/wallets`
- **Get details**: `GET /api/wallets/<id>`
- **Rename**: `PATCH /api/wallets/<id>`
  - Body: `{"label":"<new label>"}`
  - No approval needed

Always present wallet lists in this format:

> 1. **Main Wallet** — Ethereum · `0xabcd…1234` ✅
> 2. **Trading** — Solana · `HN7c…Qx9f` ✅

If a wallet has a label, show `label · shortened_address`.

### Balances

- **Native balance**: `GET /api/wallets/<id>/balance`
  - Returns native gas token only: ETH / SOL / etc.
- **Whitelisted contracts**: `GET /api/chains/<chain>/contracts`
- **Read token balances on EVM**: `POST /api/wallets/<id>/call-read`
  - Body:

```json
{
  "contract": "<token_contract>",
  "func_sig": "balanceOf(address)",
  "args": ["<wallet_address>"]
}
```

Balance response rules:
- show native + token balances together
- for EVM token balances, do not rely on `/balance`; use `/call-read`
- only show token balances that are relevant or non-zero
- after a transfer, wait about 15 seconds before re-checking

### Native Transfer

`POST /api/wallets/<id>/transfer`

```json
{
  "to": "<recipient_address_or_nickname>",
  "amount": "<human-readable amount>",
  "memo": "<optional memo>"
}
```

Rules:
- `to` accepts raw addresses or address-book nicknames
- no extra chat confirmation
- for larger ETH transfers, pre-check `balance >= amount + 0.0005 ETH`

If response is `completed`, show tx hash and explorer link.
If response is `pending_approval`, follow the approval flow below.

### Token Transfer

Also uses `POST /api/wallets/<id>/transfer`, but **must** include `token`:

```json
{
  "to": "<recipient_address_or_nickname>",
  "amount": "<human-readable token amount>",
  "token": {
    "contract": "<erc20_contract_or_spl_mint>",
    "symbol": "<e.g. USDC>",
    "decimals": 6
  }
}
```

Rules:
- first check whitelist via `GET /api/chains/<chain>/contracts` (the wallet's chain)
- if not whitelisted, add it first
- amount is in human units, not wei/lamports
- on Solana, the backend auto-creates the recipient ATA if needed

Common testnet tokens:

| Chain | Token | Contract | Decimals |
|-------|-------|----------|----------|
| Sepolia | USDC | `0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238` | 6 |
| Base Sepolia | USDC | `0x036CbD53842c5426634e7929541eC2318f3dCF7e` | 6 |

### Contract Whitelist

The whitelist is keyed per **(user, chain)** — independent of any wallet —
so the chain-scoped routes are the canonical form. A user can manage the
whitelist on a chain whether or not they own a wallet there.

- **List**: `GET /api/chains/<chain>/contracts`
- **Add**: `POST /api/chains/<chain>/contracts`
- **Rename**: `PUT /api/chains/<chain>/contracts/<cid>` (label only)

`<chain>` is the chain name (e.g. `sepolia`, `base`, `solana-devnet`).

Add body:

```json
{
  "contract_address": "<0x... or base58>",
  "symbol": "<optional>",
  "decimals": "<optional>",
  "label": "<optional>"
}
```

Rename body (only `label` is editable; `symbol`/`decimals` are on-chain
metadata and are immutable via this endpoint — re-add the contract if
they need to change):

```json
{ "label": "new label" }
```

Rules:
- API-key add creates a pending approval; passkey add applies immediately
- rename (label update) applies immediately under **both** auth modes — no approval required, because the label is display-only and has no effect on transfer semantics
- remove entries in the Web UI only
- applies to EVM token contracts, Solana mints, and Solana program IDs

### Contract Calls

Use only for **state-changing** calls.

**EVM write call**: `POST /api/wallets/<id>/contract-call`

```json
{
  "contract": "<0x...>",
  "func_sig": "<e.g. transfer(address,uint256)>",
  "args": ["<arg1>", "<arg2>"],
  "value": "<optional native value>",
  "memo": "<optional>"
}
```

**Solana write call**:

```json
{
  "contract": "<program_id_base58>",
  "accounts": [
    {"pubkey":"<account>", "is_signer":false, "is_writable":true}
  ],
  "data": "<hex-encoded instruction data>",
  "memo": "<optional>"
}
```

Rules:
- target contract/program must already be whitelisted
- API-key write calls require Passkey approval
- success handling is the same as transfers

### Read-Only Contract Calls

Use `POST /api/wallets/<id>/call-read` for EVM reads such as:
- `balanceOf`
- `allowance`
- `decimals`
- `symbol`
- quotes / view functions

Example:

```json
{
  "contract": "<0x...>",
  "func_sig": "allowance(address,address)",
  "args": ["<owner>", "<spender>"]
}
```

Rules:
- no approval
- no whitelist required
- EVM only
- prefer this over direct public RPC access

### Other Endpoints

- **Approve ERC-20 allowance**: `POST /api/wallets/<id>/approve-token`
  - Body: `{"contract":"<token>","spender":"<spender>","amount":"<token units>","decimals":6}`
- **Revoke ERC-20 allowance**: `POST /api/wallets/<id>/revoke-approval`
  - Body: `{"contract":"<token>","spender":"<spender>"}`
- **Wrap SOL**: `POST /api/wallets/<id>/wrap-sol`
  - Body: `{"amount":"<SOL amount>"}`
- **Unwrap SOL**: `POST /api/wallets/<id>/unwrap-sol`
  - Body: `{}`
- **Address book list/add/update**: `GET /api/addressbook`, `POST /api/addressbook`, `PUT /api/addressbook/<id>`
  - Nicknames: lowercase alphanumeric plus `_` or `-`, max 100 chars
  - Add/update via API key creates pending approval
  - Delete via Web UI only
  - If user says "send 0.1 ETH to alice", pass `alice` directly as `to`
  - When displaying a contact, show `nickname · shortened_address`
- **Set policy**: `PUT /api/wallets/<id>/policy`
  - Body: `{"threshold_usd":"100","enabled":true,"daily_limit_usd":"5000"}`
  - `threshold_usd`: above this needs approval
  - `daily_limit_usd`: optional hard daily cap
- **Daily spend**: `GET /api/wallets/<id>/daily-spent`
- **Pending approvals**: `GET /api/approvals/pending`
- **Check approval**: `GET /api/approvals/<approval_id>`
- **Utility**: `GET /api/health`, `GET /api/chains`, `GET /api/prices`, `POST /api/faucet`, `GET /api/audit/logs?page=1&limit=20`
  - Audit log filters: `action`, `wallet_id`, `page`, `limit`

## Approval Flow

When an API-key write returns `pending_approval`:

1. explain what needs approval
2. show the approval link: `${TEENET_WALLET_API_URL}/#/approve/{approval_id}`
3. ask the user to approve it and tell you when done
4. when they return, call `GET /api/approvals/{approval_id}`
5. report the result and continue

Handle approval result like this:
- `approved` + `tx_hash`: show tx hash + explorer link
- `approved` without tx hash: tell the user the change is active
- `rejected`: say no action was taken
- `expired`: ask the user to retry the original operation

## Guided Test Flow

When the user asks to test the wallet:

1. ensure they have a testnet wallet; create one if needed
2. check balance
3. use faucet if needed
4. create a second wallet on the same chain
5. send a tiny transfer to test TEE signing
6. set a `$1` approval threshold
7. send one transfer below threshold
8. send one transfer above threshold and wait for approval
9. add USDC to whitelist

For every step: explain before, show result after, and if approval is needed, show the link and wait for the user to come back.

## Error Handling

Check structured fields such as `stage`, `chain`, `contract`, `func_sig`, `selector`, `revert_reason`, `rpc_error`, `category`, and `request_id`.

- `revert_reason` — decoded Solidity `Error(string)` from an EVM revert.
- `rpc_error` — sanitized external error text; any URL is redacted to `<url>` so provider tokens (e.g. QuickNode) don't leak.
- `category` — stable bucket for signing / key-generation failures: `timeout`, `tee_unavailable`, `threshold_not_reached`, `cancelled`, `sdk_error`.
- `request_id` — correlation ID on 5xx responses. Quote this to operators; the full error lives in the server log keyed by the same ID. 5xx bodies deliberately omit raw error text.

Use `stage` first:

| `stage` | What it usually means |
|---------|-----------------------|
| `build_tx` / `estimate_gas` | bad args, bad ABI, revert, or insufficient balance |
| `signing` | TEE signing failed; retry |
| `broadcast` | RPC rejected the tx; retry or inspect chain conditions |
| `key_generation` | wallet creation failed; retry after a short wait |
| `eth_call` | read call failed; check contract and signature |
| `balance_query` | RPC read failed; retry |
| `faucet_request` | faucet unavailable |

Common user-facing errors:
- `insufficient funds`: not enough balance, usually including gas
- `contract not whitelisted`: add the contract first
- `wallet is not ready`: wallet creation still in progress
- `invalid API key`: API key is wrong
- `approval has expired`: rerun the original action
- `nickname not found`: address-book nickname missing for that chain
- `nonce too low`: retry the transfer
- `execution reverted`: inspect `revert_reason`

## Swap-Specific Notes

For Uniswap-style EVM swaps:
- verify ABI and selector before sending the real transaction
- `exactInputSingle` must use tuple form:
  - `exactInputSingle((address,address,uint24,address,uint256,uint256,uint160))`
- expected selector: `0x04e45aaf`
- quote first with `/call-read`
- check balance and allowance first
- do not test with 100% of balance; leave headroom
- HTTP `422` on `/contract-call` with `stage: "estimate_gas"` or on `/transfer` / `/wrap-sol` / `/unwrap-sol` with `stage: "build_tx"` means the RPC rejected the transaction before signing (commonly `eth_estimateGas` revert, insufficient balance, or bad params) — read `revert_reason` (decoded Solidity `Error(string)`) and the URL-sanitized `rpc_error` before retrying

## Explorer Links

Base URLs:

| Chain | Explorer |
|-------|----------|
| Sepolia | `https://sepolia.etherscan.io` |
| Optimism Sepolia | `https://sepolia-optimism.etherscan.io` |
| Arbitrum Sepolia | `https://sepolia.arbiscan.io` |
| Base Sepolia | `https://sepolia.basescan.org` |
| Polygon Amoy | `https://amoy.polygonscan.com` |
| BSC Testnet | `https://testnet.bscscan.com` |
| Avalanche Fuji | `https://testnet.snowtrace.io` |
| Solana Devnet | `https://solscan.io` with `?cluster=devnet` |

Formats:
- EVM tx: `{explorer}/tx/{hash}`
- EVM address: `{explorer}/address/{addr}`
- Solana tx: `{explorer}/tx/{hash}`
- Solana account: `{explorer}/account/{addr}`

## Faucet Links

| Chain | Faucet |
|-------|--------|
| Sepolia ETH | built-in `/api/faucet` |
| Base Sepolia ETH | built-in `/api/faucet` |
| Solana Devnet SOL | built-in `/api/faucet` |
| Sepolia USDC | [https://faucet.circle.com](https://faucet.circle.com) |
| Base Sepolia USDC | [https://faucet.circle.com](https://faucet.circle.com) |
