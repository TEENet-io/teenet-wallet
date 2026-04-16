# 认证体系

TEENet Wallet 采用双层认证模型。许多钱包接口同时接受 API Key 和 Passkey 会话，但涉及账户管理或破坏性操作的敏感接口仅允许 Passkey 会话调用。

### API Key

API Key 是面向 AI Agent 和自动化程序的认证方式。

**获取方式：** 通过 Passkey 会话在 Web UI 中生成，或调用 `POST /api/auth/apikey/generate`。

**使用方式：** 在 HTTP 请求头中携带：

```
Authorization: Bearer ocw_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

**权限范围：**
- 可执行：创建钱包、查询余额、发起转账、查看审批列表、调用合约等
- 需要审批：设置审批策略（创建待审批请求）、添加白名单合约（创建待审批请求）
- 不可执行：删除钱包、删除审批策略、审批/拒绝请求、撤销/重命名 API Key、删除地址簿条目（仅 Passkey）

**速率限制：** 默认每分钟 200 次请求，钱包创建每分钟 5 次（可通过环境变量调整）。

**管理操作：**

```bash
# 列出所有 API Key
curl -s "${TEE_WALLET_URL}/api/auth/apikey/list" \
  -H "Authorization: Bearer ps_session_token"

# 重命名 API Key（仅 Passkey 会话）
curl -s -X PATCH "${TEE_WALLET_URL}/api/auth/apikey" \
  -H "Authorization: Bearer ps_session_token" \
  -H "X-CSRF-Token: csrf_token_value" \
  -H "Content-Type: application/json" \
  -d '{"prefix": "ocw_a1b2c3d4", "label": "new-label"}'

# 撤销 API Key（仅 Passkey 会话）
curl -s -X DELETE "${TEE_WALLET_URL}/api/auth/apikey?prefix=ocw_a1b2c3d4" \
  -H "Authorization: Bearer ps_session_token" \
  -H "X-CSRF-Token: csrf_token_value"
```

### Passkey 会话

Passkey 基于 WebAuthn 标准，利用硬件安全密钥（如 YubiKey、Touch ID、Windows Hello）进行身份验证。

**登录流程：**

```bash
# 1. 获取登录挑战
curl -s "${TEE_WALLET_URL}/api/auth/passkey/options"

# 2. 使用硬件密钥签署挑战并提交验证
curl -s -X POST "${TEE_WALLET_URL}/api/auth/passkey/verify" \
  -H "Content-Type: application/json" \
  -d '{"credential": "<webauthn_assertion>"}'
```

验证成功后返回 `ps_` 前缀的会话令牌。

**权限范围：** 拥有所有 API Key 的权限，外加：
- 删除钱包、删除策略、删除合约白名单条目
- 审批/拒绝待审批请求
- 生成/撤销 API Key
- 邀请新用户（管理员场景）、删除账户

### CSRF 保护

所有通过 Passkey 会话发起的状态变更请求（POST、PUT、DELETE）都需要携带 CSRF 令牌：

```
X-CSRF-Token: <csrf_token_value>
```

CSRF 令牌在登录时返回。API Key 请求不受 CSRF 保护约束。

---
[上一页: 配置参考](/zh/configuration.md) | [下一页: 钱包管理](/zh/wallets.md)
