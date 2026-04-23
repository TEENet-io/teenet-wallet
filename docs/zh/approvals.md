# 审批系统

### USD 阈值

每个钱包可设置一个 USD 计价的审批策略。该策略用于控制转账阈值和每日转账限额：

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

- `threshold_usd`：单笔转账超过此 USD 值需要 Passkey 审批
- `daily_limit_usd`（可选）：每 UTC 日累计 USD 转账支出上限，超出则硬性拒绝
- `enabled`：策略启用/禁用开关

### 决策模型

| 操作 | API Key 行为 | Passkey 行为 |
|------|--------------|--------------|
| `/transfer` | 低于阈值直接执行；超过阈值进入审批 | 直接执行 |
| 无法定价的代币转账 | 进入审批（fail-closed） | 直接执行 |
| `/contract-call` | 始终需要审批 | 直接执行 |
| `/approve-token` | 始终需要审批 | 直接执行 |
| `/revoke-approval` | 始终需要审批 | 直接执行 |

通过 API Key 发起的合约白名单新增也会创建待审批请求。通过 Passkey 会话发起则立即生效。重命名（label 修改）不触发审批，任何鉴权方式都直接生效。

**转账价格换算规则：**
- 原生代币（ETH、SOL、BNB 等）：通过 CoinGecko API 获取实时价格（10 秒缓存）
- 稳定币（USDC、USDT、DAI、BUSD）：固定按 $1 计价
- ERC-20 代币：通过 CoinGecko Token Price API 按合约地址查价
- Solana SPL 代币：优先 CoinGecko，无数据时回退到 Jupiter Price API
- 代币转账价格不可用时，该笔转账需要审批（fail-closed）
- 可通过 `GET /api/prices` 查看当前使用的价格

### 日限额

日限额仅适用于转账，并以 UTC 日历日为周期运作：

- 每笔转账发起时，系统预扣相应 USD 额度
- 若签名或广播失败，预扣额度自动回退（auth/capture 模式）
- 超过日限额的交易被硬性拒绝，无审批路径可绕过
- UTC 午夜自动重置

**查看当日已花费额度：**

```bash
curl -s "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/daily-spent" \
  -H "Authorization: Bearer ${API_KEY}"
```

返回：

```json
{
  "daily_spent_usd": "235.50",
  "daily_limit_usd": "5000",
  "remaining_usd": "4764.50",
  "reset_at": "2026-03-27T00:00:00Z"
}
```

若未设置策略，所有字段返回空字符串，`daily_spent_usd` 返回 `"0"`。

### 审批流程

当交易触发审批时（金额超阈值、合约调用通过 API Key 发起等），流程如下：

1. **API 返回待审批状态**：

```json
{
  "status": "pending_approval",
  "approval_id": 42,
  "approval_url": "https://wallet.teenet.app/#/approve/42"
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

### 地址簿审批

通过 API Key 添加或修改地址簿条目时，会创建待审批请求。Passkey 所有者确认后生效。删除地址簿条目仅限 Passkey 会话。

### 合约调用审批

所有通过 API Key 发起的合约调用都需要 Passkey 审批。这是设计决策 -- 合约交互的风险高于简单转账，钱包对每一笔由 Agent 发起的合约调用强制要求人工确认。

通过 Passkey 会话发起的合约调用则立即执行（人类已在场）。

便捷端点 `approve-token` 和 `revoke-approval` 也遵循同样的规则：通过 API Key 发起时需要审批，通过 Passkey 会话发起时直接执行。

---
[上一页: 智能合约交互](/zh/smart-contracts.md) | [下一页: AI Agent 集成](/zh/agent-integration.md)
