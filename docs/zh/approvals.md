# 审批系统

### USD 阈值

每个钱包可设置一个 USD 计价的审批策略，覆盖所有币种：

```bash
curl -s -X PUT "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/policy" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "threshold_usd": 100,
    "daily_limit_usd": 5000,
    "enabled": true
  }'
```

- `threshold_usd`：单笔交易超过此 USD 值需要 Passkey 审批
- `daily_limit_usd`（可选）：每 UTC 日累计 USD 支出上限，超出则硬性拒绝
- `enabled`：策略启用/禁用开关

价格换算规则：
- ETH/SOL：通过 CoinGecko API 获取实时价格（10 秒缓存）
- 稳定币（USDC、USDT、DAI 等）：固定按 $1 计价
- 可通过 `GET /api/prices` 查看当前使用的价格

### 日限额

日限额以 UTC 日历日为周期运作：

- 每笔转账发起时，系统预扣相应 USD 额度
- 若签名或广播失败，预扣额度自动回退（auth/capture 模式）
- 超过日限额的交易被硬性拒绝，无审批路径可绕过
- UTC 午夜自动重置

### 审批流程

当交易触发审批时（金额超阈值、高危方法调用、非自动审批合约等），流程如下：

1. **API 返回待审批状态**：

```json
{
  "status": "pending_approval",
  "approval_id": 42,
  "approval_url": "http://localhost:8080/#/approve/42"
}
```

2. **钱包所有者在 Web UI 中审批**：访问审批链接，使用 Passkey 硬件认证进行审批或拒绝。每次审批都需要即时的硬件验证。

3. **轮询审批结果**：

```bash
# 每 15 秒轮询一次
curl -s "${TEE_WALLET_URL}/api/approvals/${APPROVAL_ID}" \
  -H "Authorization: Bearer ${API_KEY}"
```

4. **处理结果**：
- `status: "approved"` + `tx_hash`：交易已完成并广播
- `status: "rejected"`：审批被拒绝，未执行任何操作
- `status: "expired"`：审批超时（默认 24 小时），需重新发起

**查看所有待审批请求：**

```bash
curl -s "${TEE_WALLET_URL}/api/approvals/pending" \
  -H "Authorization: Bearer ${API_KEY}"
```

**审批/拒绝（仅 Passkey）：**

```bash
# 审批
curl -s -X POST "${TEE_WALLET_URL}/api/approvals/${APPROVAL_ID}/approve" \
  -H "Authorization: Bearer ps_session_token" \
  -H "X-CSRF-Token: csrf_token_value"

# 拒绝
curl -s -X POST "${TEE_WALLET_URL}/api/approvals/${APPROVAL_ID}/reject" \
  -H "Authorization: Bearer ps_session_token" \
  -H "X-CSRF-Token: csrf_token_value"
```

### 高危方法

以下 EVM 合约方法被标记为高危，**始终**需要 Passkey 审批，即使合约已开启 `auto_approve`：

| 方法 | 风险说明 |
|------|---------|
| `approve` | 授权第三方无限额支出代币 |
| `increaseAllowance` | 增加第三方支出授权额度 |
| `setApprovalForAll` | 授权第三方操作所有 NFT |
| `transferFrom` | 从他人账户转移代币 |
| `safeTransferFrom` | 从他人账户安全转移代币/NFT |

---
[上一页: 智能合约交互](smart-contracts.md) | [下一页: AI Agent 集成](agent-integration.md)
