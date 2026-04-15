# 审计日志

所有钱包操作都会记录在审计日志中。通过以下方式查询：

```bash
curl -s "${SERVICE_URL}/api/audit/logs?page=1&limit=20" \
  -H "Authorization: Bearer ${API_KEY}"
```

## 查询参数

| 参数 | 默认值 | 描述 |
|------|--------|------|
| `page` | `1` | 页码 |
| `limit` | `20` | 每页结果数（最大：100） |
| `action` | _（全部）_ | 按操作类型筛选 |
| `wallet_id` | _（全部）_ | 按钱包筛选 |

## 操作类型

| 操作 | 描述 |
|------|------|
| `login` | Passkey 登录 |
| `wallet_create` | 钱包已创建 |
| `wallet_delete` | 钱包已删除 |
| `transfer` | 转账已发送或待处理 |
| `sign` | 转账/合约操作期间的内部签名步骤 |
| `policy_update` | 审批策略已设置或待处理 |
| `approval_approve` | 审批请求已批准 |
| `approval_reject` | 审批请求已拒绝 |
| `contract_add` | 合约已添加到白名单或待处理 |
| `wrap_sol` | SOL 包装为 wSOL |
| `unwrap_sol` | wSOL 解包为 SOL |
| `contract_update` | 合约白名单条目已更新 |
| `contract_delete` | 合约已从白名单中移除 |
| `contract_call` | 合约调用已执行 |
| `addressbook_add` | 地址簿条目已添加 |
| `addressbook_update` | 地址簿条目已更新 |
| `addressbook_delete` | 地址簿条目已删除 |
| `apikey_generate` | API key 已生成 |
| `apikey_revoke` | API key 已撤销 |
| `apikey_rename` | API key 已重命名 |
