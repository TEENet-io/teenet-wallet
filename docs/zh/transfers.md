# 转账

### 原生代币（ETH / SOL）

发送原生代币（ETH、SOL、tBNB 等）到指定地址：

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/transfer" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "0.5",
    "memo": "付款备注（可选）"
  }'
```

**参数说明：**
- `to`（必填）：接收方地址（EVM 为 `0x...` 格式，Solana 为 base58 格式）
- `amount`（必填）：转账金额，人类可读单位（如 `"0.5"` 表示 0.5 ETH）
- `memo`（可选）：交易备注

**返回结果：**

```json
{
  "status": "completed",
  "tx_hash": "0xabc123...",
  "chain": "ethereum",
  "amount": "0.5",
  "currency": "ETH"
}
```

若需要审批，返回：

```json
{
  "status": "pending_approval",
  "approval_id": 42,
  "approval_url": "http://localhost:8080/#/approve/42"
}
```

### ERC-20 代币转账

发送 ERC-20 代币时，**必须**在请求中包含 `token` 字段：

```bash
# 前提：代币合约已在白名单中
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/transfer" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "100",
    "token": {
      "contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
      "symbol": "USDC",
      "decimals": 6
    }
  }'
```

> **警告**：遗漏 `token` 字段将发送原生 ETH 而非代币——这是完全不同的交易，无法撤回。请务必确认请求体中包含 `"token": {...}`。

**`amount` 为人类可读单位**：`"100"` 表示 100 USDC，后端自动根据 `decimals` 转换为链上原始单位。

**常见 ERC-20 代币参数：**

| 网络 | 代币 | 合约地址 | 精度 |
|------|------|---------|------|
| Ethereum 主网 | USDC | `0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48` | 6 |
| Ethereum 主网 | USDT | `0xdac17f958d2ee523a2206206994597c13d831ec7` | 6 |
| Ethereum 主网 | WETH | `0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2` | 18 |
| Ethereum 主网 | DAI | `0x6b175474e89094c44da98b954eedeac495271d0f` | 18 |
| Sepolia 测试网 | USDC | `0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238` | 6 |
| Sepolia 测试网 | WETH | `0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14` | 18 |
| Base Sepolia | USDC | `0x036CbD53842c5426634e7929541eC2318f3dCF7e` | 6 |

### SPL 代币转账

Solana 上的 SPL 代币转账同样使用 `/transfer` 端点，通过 `token` 字段指定代币铸造地址：

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/transfer" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "RecipientBase58Address...",
    "amount": "50",
    "token": {
      "contract": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
      "symbol": "USDC",
      "decimals": 6
    }
  }'
```

**Solana 特有行为：** 如果接收方没有该代币的关联代币账户（ATA），后端会在同一交易中自动创建。无需额外操作。

代币铸造地址必须先加入白名单（与 EVM 合约白名单使用同一接口）。

### Wrap / Unwrap SOL

将原生 SOL 转换为 wSOL（Wrapped SOL SPL 代币），供 DeFi 协议使用：

```bash
# Wrap SOL（原生 SOL -> wSOL）
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/wrap-sol" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"amount": "0.1"}'

# Unwrap SOL（wSOL -> 原生 SOL，关闭 wSOL ATA，全部余额）
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/unwrap-sol" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{}'
```

Wrap 操作会自动创建 wSOL ATA（如果不存在）。Unwrap 操作关闭整个 wSOL ATA，将全部 wSOL 余额转回原生 SOL。

### 幂等性

通过 `Idempotency-Key` HTTP 头实现转账请求的幂等性，防止因网络重试导致的重复交易：

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/transfer" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: unique-request-id-12345" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "0.01"
  }'
```

相同的 `Idempotency-Key` 在有效期内会返回首次请求的缓存结果，不会重复执行交易。

- **作用域：** Per-user -- 同一用户的不同 API Key 共享幂等键命名空间。
- **有效期：** 24 小时 -- 过期后同一键可重新使用。
- **适用端点：** `/transfer`、`/contract-call`、`/wrap-sol`、`/unwrap-sol`。

---
[上一页: 钱包管理](wallets.md) | [下一页: 智能合约交互](smart-contracts.md)
