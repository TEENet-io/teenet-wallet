# API 参考

> 完整 OpenAPI 规范请查看 [openapi.yaml](../api/openapi.yaml)

## 支持的链

### 内置链

| 链名称 | 显示名 | 币种 | 协议 | 曲线 | 链族 |
|--------|--------|------|------|------|------|
| `ethereum` | Ethereum Mainnet | ETH | ECDSA | secp256k1 | EVM |
| `optimism` | Optimism Mainnet | ETH | ECDSA | secp256k1 | EVM |
| `sepolia` | Sepolia Testnet | ETH | ECDSA | secp256k1 | EVM |
| `holesky` | Holesky Testnet | ETH | ECDSA | secp256k1 | EVM |
| `base-sepolia` | Base Sepolia Testnet | ETH | ECDSA | secp256k1 | EVM |
| `bsc-testnet` | BSC Testnet | tBNB | ECDSA | secp256k1 | EVM |
| `solana` | Solana Mainnet | SOL | Schnorr | ed25519 | Solana |
| `solana-devnet` | Solana Devnet | SOL | Schnorr | ed25519 | Solana |

### 自定义链

支持在运行时添加任意 EVM 兼容链（Solana 自定义链暂不支持）：

```bash
# 添加自定义链（仅 Passkey 会话）
curl -s -X POST "${TEE_WALLET_URL}/api/chains" \
  -H "Authorization: Bearer ps_session_token" \
  -H "X-CSRF-Token: csrf_token_value" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "arbitrum",
    "label": "Arbitrum One",
    "currency": "ETH",
    "rpc_url": "https://arb1.arbitrum.io/rpc",
    "chain_id": 42161
  }'

# 删除自定义链（仅 Passkey，且该链上无钱包时才可删除）
curl -s -X DELETE "${TEE_WALLET_URL}/api/chains/arbitrum" \
  -H "Authorization: Bearer ps_session_token" \
  -H "X-CSRF-Token: csrf_token_value"
```

自定义链自动使用 ECDSA/secp256k1 协议，持久化存储在数据库中，服务重启后自动加载。

---

## 错误参考

### 常见错误及解决方案

| 错误信息 | 原因 | 解决方案 |
|---------|------|---------|
| `insufficient funds` | 钱包余额不足以支付转账金额和 Gas 费 | 检查余额，确保预留足够的 Gas 费（ETH 约 0.0005 ETH） |
| `daily spend limit exceeded` | 当日 USD 累计支出已超过日限额 | 等待 UTC 午夜自动重置，或通过 Passkey 调整日限额策略 |
| `contract not whitelisted` | 目标合约/铸造地址/程序 ID 未在白名单中 | 通过 API Key 提交白名单添加请求或在 Web UI 中直接添加 |
| `method not allowed` | 调用的方法不在合约的 `allowed_methods` 列表中 | 更新白名单条目，将所需方法加入 `allowed_methods` |
| `wallet is not ready` | 钱包仍在创建中（DKG 进行中） | 等待 1-2 分钟后重试 |
| `invalid API key` | API Key 无效或已被撤销 | 检查 API Key 是否正确，或重新生成 |
| `approval has expired` | 审批请求已超时（默认 30 分钟） | 重新发起转账或操作 |
| `pending_approval` (策略变更) | 通过 API Key 设置策略需要 Passkey 审批 | 在 Web UI 中审批待处理的策略变更请求 |
| `cannot overwrite a built-in chain` | 试图添加与内置链同名的自定义链 | 使用不同的链名称 |
| `chain has existing wallets` | 试图删除仍有钱包的自定义链 | 先删除该链上的所有钱包 |
| `rate limit exceeded` | 超过 API 请求速率限制 | 降低请求频率，或调整 `API_KEY_RATE_LIMIT` 环境变量 |
| `max wallets reached` | 已达到用户钱包数量上限 | 删除不需要的钱包，或调整 `MAX_WALLETS_PER_USER` |
| `CSRF token missing` | Passkey 会话请求缺少 CSRF 令牌 | 在请求头中添加 `X-CSRF-Token` |

### HTTP 状态码说明

| 状态码 | 含义 |
|--------|------|
| `200` | 请求成功 |
| `201` | 资源创建成功（如钱包、自定义链） |
| `202` | 请求已接受，等待审批（API Key 设置策略/添加白名单时） |
| `400` | 请求参数错误 |
| `401` | 认证失败（无效的 API Key 或会话） |
| `403` | 权限不足（如 API Key 尝试执行 Passkey 专属操作） |
| `404` | 资源不存在 |
| `409` | 资源冲突（如重复的链名称、链上仍有钱包） |
| `429` | 请求频率超限 |
| `500` | 服务端内部错误 |

### 调试建议

1. **检查健康状态**：调用 `GET /api/health` 确认服务和数据库正常
2. **查看审计日志**：调用 `GET /api/audit/logs` 查看操作历史和错误详情
3. **确认链配置**：调用 `GET /api/chains` 确认目标链已加载
4. **检查 app-comm-consensus**：确保 `CONSENSUS_URL` 对应的节点正常运行且可达
5. **结构化日志**：服务以 JSON 格式输出日志（slog），可通过日志聚合工具分析

---
[上一页: AI Agent 集成](agent-integration.md)
