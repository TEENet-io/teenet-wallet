# 错误码与状态码

## 常见错误及解决方案

| 错误信息 | 原因 | 解决方案 |
|----------|------|----------|
| `insufficient funds` | 钱包余额不足以支付转账金额和 gas 费用 | 通过 `GET /api/wallets/:id/balance` 检查余额。对于 ETH 转账，需预留约 0.0005 ETH 作为 gas 费用。 |
| `daily spend limit exceeded` | 当日累计 USD 支出已达到每日限额 | 等待 UTC 午夜限额重置，或通过 Passkey 调整策略。 |
| `contract not whitelisted` | 合约地址、代币 mint 或 program ID 不在钱包白名单中 | 通过 `POST /api/wallets/:id/contracts` 添加（API key 会创建待审批请求），或通过 Web UI 即时审批。 |
| `contract operations require passkey approval` | 通过 API key 发起的合约调用需要人工确认 | 钱包所有者需通过 Web UI 中的 Passkey 审批待处理的请求。 |
| `wallet is not ready` | 钱包仍处于 `creating` 状态（DKG 进行中） | 等待 1-2 分钟让 ECDSA 密钥生成完成，然后重试。 |
| `invalid API key` | 提供的 API key 无效或已被撤销 | 验证 `Authorization` 请求头的值。如有需要，生成新的密钥。 |
| `approval has expired` | 待审批请求在有效期内未被处理（默认：24 小时） | 重新发起操作以创建新的审批请求。 |
| `cannot overwrite a built-in chain` | 尝试创建与内置链同名的自定义链 | 为自定义链选择一个不同的名称。 |
| `chain has existing wallets; delete them first` | 尝试删除仍有钱包的自定义链 | 先删除该链上的所有钱包，再移除链。 |
| `rate limit exceeded` | 当前时间窗口内请求过多 | 等待后重试。默认限制：每个 API key 100 次请求/分钟，每用户 RPC 总调用 50 次/分钟（读写共享同一个桶），5 次钱包创建/分钟，每个 IP 10 次注册/分钟。 |
| `invalid CSRF token` | Passkey 会话请求使用了缺失、过期或错误的 `X-CSRF-Token` | 在状态变更的 Passkey 请求中，使用登录时返回的 `csrf_token` 作为 `X-CSRF-Token` 请求头。 |
| `passkey session required` | 该操作需要 Passkey 认证，但使用了 API key 调用 | 使用 Passkey 会话执行此操作（钱包删除、策略删除、合约移除、审批/拒绝）。 |
| `max wallets reached` | 用户已达到 `MAX_WALLETS_PER_USER` 限制 | 删除不使用的钱包，或在服务器配置中增加限制。 |

## HTTP 状态码

| 状态码 | 含义 |
|--------|------|
| `200` | 成功 |
| `201` | 资源已创建 |
| `202` | 请求已接受；等待 Passkey 审批 |
| `400` | 无效请求（缺少字段、格式错误） |
| `401` | 需要认证或凭证无效 |
| `403` | 禁止访问（例如，尝试删除内置链） |
| `404` | 资源未找到 |
| `409` | 冲突（例如，链名称重复、链上存在钱包） |
| `429` | 超出速率限制 |
| `500` | 服务器内部错误 |
