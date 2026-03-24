# Smart Contracts

### Contract Whitelist

Before calling any smart contract, the contract address (EVM), token mint (SPL), or program ID (Solana) must be added to the wallet's whitelist.

**List whitelisted contracts:**

```bash
curl -s http://localhost:8080/api/wallets/WALLET_ID/contracts \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

**Add a contract via API key** (creates a pending approval):

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/contracts \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contract_address": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
    "symbol": "USDC",
    "decimals": 6,
    "label": "USD Coin",
    "allowed_methods": "transfer,balanceOf",
    "auto_approve": false
  }'
```

A `202` response means the request needs Passkey approval:

```json
{
  "success": true,
  "pending": true,
  "approval_id": 7,
  "message": "Contract whitelist request submitted for approval"
}
```

**Add a contract via Passkey session** (applied immediately):

The same endpoint returns `201` when called with a Passkey session, and the contract is whitelisted immediately.

**Update a contract configuration:**

```bash
curl -s -X PUT http://localhost:8080/api/wallets/WALLET_ID/contracts/CONTRACT_ID \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"auto_approve": true, "allowed_methods": "transfer,approve,balanceOf"}'
```

**Remove a contract** (Passkey only):

```bash
curl -s -X DELETE http://localhost:8080/api/wallets/WALLET_ID/contracts/CONTRACT_ID \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck"
```

**Whitelist fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `contract_address` | Yes | EVM address (`0x...`) or Solana mint/program address (base58) |
| `symbol` | No | Token symbol (e.g., USDC) |
| `decimals` | No | Token decimals (6 for USDC, 18 for WETH, 9 for most SPL tokens) |
| `label` | No | Human-readable label |
| `allowed_methods` | No | Comma-separated method names (e.g., `transfer,approve`). Empty = all methods allowed |
| `auto_approve` | No | If `true`, API keys can execute non-high-risk calls without Passkey. Default: `false` |

### Contract Calls (EVM)

Call any whitelisted smart contract function using the `/contract-call` endpoint with `func_sig` and `args`:

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/contract-call \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
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
    "amount_usd": "250.00",
    "memo": "DEX swap"
  }'
```

**Function signature format:** Use Solidity-style signatures such as `transfer(address,uint256)`, `approve(address,uint256)`, or `exactInputSingle((address,address,uint24,address,uint256,uint256,uint160))` for tuple parameters.

**Supported argument types:** `address`, `bool`, `uintN`, `intN`, `bytesN`, `bytes`, `string`, dynamic arrays, fixed-size arrays (`T[N]`), and tuples.

### Program Calls (Solana)

For Solana programs, use `accounts` and `data` instead of `func_sig`/`args`:

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/contract-call \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "ProgramIdBase58...",
    "accounts": [
      {"pubkey": "Account1Base58...", "is_signer": false, "is_writable": true},
      {"pubkey": "Account2Base58...", "is_signer": false, "is_writable": false}
    ],
    "data": "a1b2c3d4e5f6...",
    "amount_usd": "100.00",
    "memo": "program interaction"
  }'
```

The program ID must be whitelisted. The wallet's own address is added as a signer automatically if required. The `data` field contains hex-encoded instruction data (discriminator + encoded arguments).

### Read-Only Calls

Query contract state without signing or sending a transaction. No gas fees, no approval needed:

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/call-read \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
    "func_sig": "balanceOf(address)",
    "args": ["0xYourWalletAddress..."]
  }'
```

Response:

```json
{
  "success": true,
  "result": "0x0000000000000000000000000000000000000000000000000000000005f5e100",
  "contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
  "method": "balanceOf"
}
```

Useful for checking token balances (`balanceOf`), allowances (`allowance`), and reading contract state (`totalSupply`, `name`, `symbol`, `decimals`). This endpoint is EVM-only.

### The `amount_usd` Field for Threshold Enforcement

When a contract call involves transferring value (e.g., a DeFi swap, a deposit, a token transfer via contract), include the `amount_usd` field with the approximate USD value so the wallet can enforce threshold and daily-limit policies.

```json
{
  "contract": "0x...",
  "func_sig": "deposit(uint256)",
  "args": ["1000000000"],
  "amount_usd": "1000.00"
}
```

If both `value` (native ETH sent with the call) and `amount_usd` are present, the wallet uses whichever is larger. If neither is provided, threshold and daily-limit checks are skipped for that call.

You can check current prices via `GET /api/prices` to help compute the USD value.

### Convenience Endpoints

**Approve ERC-20 token spending** (always requires Passkey -- high-risk method):

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/approve-token \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xTokenAddress...",
    "spender": "0xSpenderAddress...",
    "amount": "1000",
    "decimals": 6
  }'
```

**Revoke ERC-20 token approval** (always requires Passkey -- high-risk method):

```bash
curl -s -X POST http://localhost:8080/api/wallets/WALLET_ID/revoke-approval \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xTokenAddress...",
    "spender": "0xSpenderAddress..."
  }'
```

---
[Previous: Transfers](transfers.md) | [Next: Approval System](approvals.md)
