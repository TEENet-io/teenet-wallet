# 钱包管理

### 创建钱包

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/wallets" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"chain": "ethereum", "label": "主钱包"}'
```

**参数说明：**
- `chain`（必填）：链名称，必须是 `GET /api/chains` 返回的有效链名
- `label`（可选）：钱包的人类可读标签

**注意事项：**
- EVM 链（ECDSA）钱包创建需要分布式密钥生成，耗时约 1-2 分钟
- Solana（EdDSA / ed25519）钱包通常即时完成
- 每个用户最多创建 10 个钱包（可通过 `MAX_WALLETS_PER_USER` 调整）
- 钱包创建受独立速率限制（默认每分钟 5 个）

### 列出钱包

```bash
curl -s "${TEE_WALLET_URL}/api/wallets" \
  -H "Authorization: Bearer ${API_KEY}"
```

### 获取钱包详情

```bash
curl -s "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}" \
  -H "Authorization: Bearer ${API_KEY}"
```

### 重命名钱包

```bash
curl -s -X PATCH "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"label": "新名称"}'
```

### 删除钱包

```bash
# 仅 Passkey 会话可执行（不可逆操作）
curl -s -X DELETE "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}" \
  -H "Authorization: Bearer ps_session_token" \
  -H "X-CSRF-Token: csrf_token_value"
```

> 警告：删除钱包是不可逆操作。删除前请确保钱包中没有剩余资产。

### 链选择

创建钱包时，`chain` 字段决定了钱包所属的区块链网络：

- EVM 链族（`family: "evm"`）：使用 ECDSA/secp256k1 协议，地址格式为 `0x...`
- Solana 链族（`family: "solana"`）：使用 EdDSA/ed25519 协议，地址格式为 base58

可通过 `GET /api/chains` 获取当前可用链列表。链定义在启动时从 `chains.json` 加载,要新增/删除链请编辑该文件并重启服务,详见 [chains.json Schema](chains-schema.md) 与 [如何添加新链](howto-add-chain.md)。

---
[上一页: 认证体系](/zh/authentication.md) | [下一页: 转账](/zh/transfers.md)
