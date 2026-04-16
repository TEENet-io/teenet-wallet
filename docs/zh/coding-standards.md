# 编码规范与 CI

## 代码格式化

- 提交前运行 `go fmt ./...`。
- 运行 `go vet ./...` 检查常见错误。

## Handler 模式

每个领域（wallets、contracts、auth 等）对应一个 handler 文件。每个 handler 遵循以下流程：

1. **验证**输入
2. **权限检查**（验证调用者权限）
3. **业务逻辑**
4. **审计** -- 对任何状态变更操作调用 `writeAuditCtx()`
5. 返回 JSON **响应**

## Auth Groups

路由在 `main.go` 中按认证组进行组织：

| 组 | 认证要求 | 用途 |
|---|---------|------|
| _（裸路由）_ | 无 | 公开端点（健康检查、链列表） |
| `auth` | API key 或 Passkey | 标准操作（转账、余额查询） |
| `passkeyOnly` | Passkey 会话 | 敏感操作（删除钱包、删除策略、审批/拒绝） |
| `approveOnly` | Passkey 会话 | 审批操作（批准、拒绝待处理请求） |

## 审批门控操作

对于需要人工审批的 API key 操作，使用 `handler/helpers.go` 中的 `createPendingApproval`。该函数会创建一个 `ApprovalRequest` 并向调用者返回 HTTP 202。

## 测试

- 测试使用内存 SQLite（`file::memory:`）和 nil SDK 客户端。
- Handler 在签名步骤会优雅地失败 -- 这是设计使然。测试验证签名发生之前的所有行为。
- 对于需要真实加密签名的集成测试，请使用 [teenet-sdk/mock-server](https://github.com/TEENet-io/teenet-sdk/tree/main/mock-server) 中的 mock server。
- 所有新功能都应包含测试。参见 `handler/*_test.go` 中的示例。

## CI 流水线

GitHub Actions 在每个 pull request 上运行以下检查：

1. **Lint** -- `go vet` 和 `staticcheck`
2. **测试** -- `go test ./... -race`
3. **漏洞扫描**

所有检查必须通过后 PR 才能合并。
