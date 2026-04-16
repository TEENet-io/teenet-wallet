# Smart Contracts

### Contract Whitelist

Before calling any smart contract, the contract address (EVM), token mint (SPL), or program ID (Solana) must be added to the contract whitelist.

> **Scope:** the whitelist is **per user + chain**, not per wallet. All wallets you own on the same chain share a single whitelist, and deleting a wallet does **not** remove its whitelist entries. The wallet ID in the URL is only used to derive the chain.
>
> **Role:** the whitelist is admission control only. It decides whether a contract or program can be called at all. It does not auto-approve methods or bypass Passkey approval for API-key-initiated contract operations.

**List whitelisted contracts:**

```bash
curl -s ${TEE_WALLET_URL}/api/wallets/WALLET_ID/contracts \
  -H "Authorization: Bearer ${API_KEY}"
```

**Add a contract via API key** (creates a pending approval):

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets/WALLET_ID/contracts \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract_address": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
    "symbol": "USDC",
    "decimals": 6,
    "label": "USD Coin"
  }'
```

A `202` response means the request needs Passkey approval:

```json
{
  "success": true,
  "pending": true,
  "approval_id": 4839271056,
  "message": "Contract whitelist request submitted for approval"
}
```

**Add a contract via Passkey session** (applied immediately):

The same endpoint returns `201` when called with a Passkey session, and the contract is whitelisted immediately.

**Update a contract configuration:**

```bash
curl -s -X PUT ${TEE_WALLET_URL}/api/wallets/WALLET_ID/contracts/CONTRACT_ID \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"label": "USDC v2", "symbol": "USDC", "decimals": 6}'
```

**Remove a contract** (Passkey only):

```bash
curl -s -X DELETE ${TEE_WALLET_URL}/api/wallets/WALLET_ID/contracts/CONTRACT_ID \
  -H "Authorization: Bearer ps_${SESSION_TOKEN}" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}"
```

**Whitelist fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `contract_address` | Yes | EVM address (`0x...`) or Solana mint/program address (base58) |
| `symbol` | No | Token symbol (e.g., USDC) |
| `decimals` | No | Token decimals (6 for USDC, 18 for WETH, 9 for most SPL tokens) |
| `label` | No | Human-readable label |

### Why No ABI File Is Required (EVM)

The wallet does not require a full ABI JSON file for contract calls. Instead, callers provide:

- `func_sig`, such as `transfer(address,uint256)`
- `args`, as a positional array matching that signature

This works because the function signature already contains the type information needed to encode calldata. The wallet's backend ABI encoder uses `func_sig` and `args` to build the EVM call data directly.

### Contract Calls (EVM)

Call any whitelisted smart contract function using the `/contract-call` endpoint with `func_sig` and `args`:

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets/WALLET_ID/contract-call \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xContractAddress...",
    "func_sig": "swap(address,uint256,uint256,address)",
    "args": [
      "0xTokenAddress...",
      "1000000",
      "990000",
      "0xRecipientAddress..."
    ],
    "value": "0.1",
    "memo": "DEX swap"
  }'
```

**Function signature format:** Use Solidity-style signatures such as `transfer(address,uint256)`, `approve(address,uint256)`, or `exactInputSingle((address,address,uint24,address,uint256,uint256,uint160))` for tuple parameters.

**Supported argument types:** `address`, `bool`, `uintN`, `intN`, `bytesN`, `bytes`, `string`, dynamic arrays, fixed-size arrays (`T[N]`), and tuples.

### Program Calls (Solana)

For Solana programs, use `accounts` and `data` instead of `func_sig`/`args`:

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets/WALLET_ID/contract-call \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "ProgramIdBase58...",
    "accounts": [
      {"pubkey": "Account1Base58...", "is_signer": false, "is_writable": true},
      {"pubkey": "Account2Base58...", "is_signer": false, "is_writable": false}
    ],
    "data": "a1b2c3d4e5f6...",
    "memo": "program interaction"
  }'
```

The program ID must be whitelisted. The wallet's own address is added as a signer automatically if required. The `data` field contains hex-encoded instruction data.

Solana does not use a chain-level ABI standard like EVM. The caller provides the program ID, the ordered account metadata, and the raw instruction bytes expected by that program.

### Read-Only Queries

For read-only contract queries (e.g., `balanceOf`, `allowance`, `totalSupply`), use `eth_call` against public RPC endpoints directly. These calls do not require signing or gas, so there is no need to route them through the wallet service.

### ERC-20 Allowance Helpers

These helper endpoints follow the same approval rule as `/contract-call`: API key requests require Passkey approval, while Passkey-session requests execute directly. See [Approval System](/en/approvals.md) for the full decision model.

**Approve ERC-20 token spending:**

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets/WALLET_ID/approve-token \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xTokenAddress...",
    "spender": "0xSpenderAddress...",
    "amount": "1000",
    "decimals": 6
  }'
```

**Revoke ERC-20 token approval:**

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets/WALLET_ID/revoke-approval \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xTokenAddress...",
    "spender": "0xSpenderAddress..."
  }'
```
