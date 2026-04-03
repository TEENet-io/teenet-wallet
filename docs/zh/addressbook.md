# 地址簿

地址簿用于保存常用的收款地址，并为其分配人类可读的昵称。保存后，转账时可以直接使用昵称代替原始地址。

### 查看条目

```bash
curl -s "${TEE_WALLET_URL}/api/addressbook" \
  -H "Authorization: Bearer ${API_KEY}"
```

支持按昵称或链筛选：

```bash
curl -s "${TEE_WALLET_URL}/api/addressbook?nickname=alice&chain=ethereum" \
  -H "Authorization: Bearer ${API_KEY}"
```

### 添加条目

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/addressbook" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "nickname": "alice",
    "chain": "ethereum",
    "address": "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
    "memo": "Alice 主钱包"
  }'
```

**参数说明：**
- `nickname`（必填）：小写字母、数字、连字符和下划线，最长 100 字符。格式：`^[a-z0-9][a-z0-9_-]*$`。
- `chain`（必填）：链名称，来自 `GET /api/chains`。
- `address`（必填）：有效的链上地址（EVM 为 `0x...`，Solana 为 base58）。
- `memo`（可选）：备注文本，最长 256 字符。

昵称在每个用户、每条链上唯一 -- 同一用户可以在 `ethereum` 和 `solana` 各有一个 `alice`，但不能在同一条链上有两个 `alice`。

**双重认证行为：**
- **API Key：** 创建待审批请求，需要 Passkey 所有者审批后才能保存。
- **Passkey 会话：** 需要即时的硬件认证，通过后立即生效。

### 更新条目

```bash
curl -s -X PUT "${TEE_WALLET_URL}/api/addressbook/ENTRY_ID" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "address": "0xNewAddress...",
    "memo": "更新后的备注"
  }'
```

可更新 `nickname`、`address` 和/或 `memo`，仅修改提供的字段。同样遵循双重认证行为。

### 删除条目

删除需要 Passkey 会话：

```bash
curl -s -X DELETE "${TEE_WALLET_URL}/api/addressbook/ENTRY_ID" \
  -H "Authorization: Bearer ps_session_token" \
  -H "X-CSRF-Token: csrf_token_value"
```

### 在转账中使用昵称

地址簿条目保存后，转账请求的 `to` 字段可以直接使用昵称代替原始地址：

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/transfer" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "alice",
    "amount": "0.1"
  }'
```

钱包会自动识别 `alice` 是昵称（非原始地址），并解析为该钱包所在链上对应的已保存地址。如果未找到匹配条目，请求将返回错误。

---
[上一页: 转账](/zh/transfers.md) | [下一页: 智能合约交互](/zh/smart-contracts.md)
