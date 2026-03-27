# 开发者指南

本指南面向需要编译、测试和参与 teenet-wallet 代码开发的开发者。

---

## 构建与运行

**前置条件：** Go 1.24+、SQLite3 开发头文件、一个运行中的 `app-comm-consensus` 节点（端口 8089）。

```bash
# 编译
make build

# 运行（默认端口 8080，数据目录 /data，consensus 地址 localhost:8089）
./teenet-wallet

# 或用 Docker
make docker
docker run -p 8080:8080 \
  -e CONSENSUS_URL=http://host.docker.internal:8089 \
  -v wallet-data:/data \
  teenet-wallet:latest
```

所有配置通过环境变量设置 -- 完整列表见 [配置参考](configuration.md)。

---

## 项目结构

```
teenet-wallet/
├── main.go              # 入口：路由注册、中间件、依赖注入
├── handler/             # HTTP 处理器（每个领域一个文件）
│   ├── auth.go          # Passkey 注册/登录、API Key 增删、账户删除
│   ├── wallet.go        # 钱包增删改查、转账、签名、wrap/unwrap SOL、策略、日花费
│   ├── contract.go      # 合约白名单增删改
│   ├── contract_call.go # 通用合约调用（EVM + Solana）、approve-token、revoke-approval
│   ├── call_read.go     # 只读合约查询（eth_call）
│   ├── approval.go      # 审批列表/详情/批准/拒绝、审批后执行
│   ├── balance.go       # 链上余额查询（原生 + ERC-20 + SPL）
│   ├── audit.go         # 审计日志查询 + writeAuditCtx 辅助函数
│   ├── price.go         # 价格服务：CoinGecko + Jupiter，TTL 缓存
│   ├── middleware.go     # 认证中间件（API Key + Passkey 会话）、CORS、CSP
│   ├── ratelimit.go     # 按 Key 和按 IP 的速率限制
│   ├── idempotency.go   # Idempotency-Key 存储（24 小时 TTL，按用户）
│   ├── helpers.go       # 公共工具：isPasskeyAuth、authInfo、createPendingApproval
│   └── response.go      # JSON 错误/成功响应辅助函数
├── model/               # GORM 模型（SQLite）
│   ├── wallet.go        # Wallet、ChainConfig、CustomChain、LoadChains()
│   ├── user.go          # User、PasskeyCredential
│   ├── apikey.go        # APIKey
│   ├── policy.go        # ApprovalPolicy、ApprovalRequest
│   ├── contract.go      # AllowedContract
│   ├── audit.go         # AuditLog
│   └── idempotency.go   # IdempotencyRecord
├── chain/               # 区块链交互（无数据库、无 HTTP -- 纯链逻辑）
│   ├── rpc.go           # EVM JSON-RPC + Solana RPC 客户端
│   ├── tx_eth.go        # 构建 EIP-1559 交易，编码转账 calldata
│   ├── tx_sol.go        # 构建 Solana 交易（原生、SPL、wrap/unwrap、程序调用）
│   ├── abi.go           # Solidity ABI 编码器（支持所有类型包括元组）
│   ├── address.go       # 从公钥推导地址（EVM + Solana）
│   └── nonce.go         # EVM nonce 管理器（并发交易安全）
├── frontend/
│   └── index.html       # 单文件 SPA（原生 JS，无构建步骤）
├── skill/
│   └── tee-wallet/
│       └── SKILL.md     # OpenClaw 技能定义
├── docs/                # Docsify 文档站点
├── Makefile             # build、test、lint、docker、clean
├── Dockerfile
└── chains.json          # 默认链配置
```

### 关键设计决策

- **单体二进制** -- 没有微服务，所有逻辑在一个 Go 进程中。SQLite 做存储。
- **Handler 直接用 GORM** -- 项目足够简单，不需要 Repository 层。
- **GORM AutoMigrate** -- Schema 变更在启动时自动应用，没有迁移文件。
- **单文件前端** -- `frontend/index.html` 是完整的 SPA，没有构建工具链。通过 `gin.Static` 嵌入。
- **TEENet SDK** -- 签名通过 `github.com/TEENet-io/teenet-sdk/go` 发送到本地 `app-comm-consensus` 节点。

---

## 测试

```bash
# 运行所有测试
make test

# 只运行 handler 测试
go test ./handler/ -v

# 运行单个测试
go test ./handler/ -run TestTransfer_ERC20 -v

# 静态检查
make lint
```

测试使用内存 SQLite（`file::memory:`），不需要运行中的 consensus 节点 -- SDK 客户端在测试中为 nil，签名调用预期失败（测试验证签名前的行为）。

### 编写测试

项目中使用的标准模式：

```go
func TestSomething(t *testing.T) {
    db := setupTestDB(t)  // 内存 SQLite + AutoMigrate
    wh := handler.NewWalletHandler(db, nil, "http://localhost:8080")
    r := gin.New()
    r.Use(handler.FakeAuth(userID))  // 注入认证上下文
    r.POST("/wallets/:id/transfer", wh.Transfer)

    body := `{"to":"0x...", "amount":"0.1"}`
    req := httptest.NewRequest("POST", "/wallets/"+walletID+"/transfer", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    // 断言响应
    assert(t, w.Code, http.StatusOK)
}
```

---

## 添加新链

1. 在 `chains.json` 中添加条目（或运行时通过 `POST /api/chains`）：

```json
{
  "name": "polygon",
  "label": "Polygon Mainnet",
  "protocol": "ecdsa",
  "curve": "secp256k1",
  "currency": "POL",
  "family": "evm",
  "rpc_url": "https://polygon-rpc.com",
  "chain_id": 137
}
```

2. 如果需要 CoinGecko 价格，在 `handler/price.go` 的 `coinGeckoIDs` 中添加币种映射。

3. 如果是 CoinGecko 支持的 EVM 平台（用于代币定价），在 `handler/price.go` 的 `coinGeckoPlatformIDs` 中添加。

标准 EVM 链不需要改代码。Solana 系列链需要修改 `chain/tx_sol.go`。

---

## 添加新 API 端点

1. **Handler** -- 在 `handler/` 的相应 handler struct 上添加方法。遵循模式：验证输入 → 检查权限 → 业务逻辑 → 写审计日志 → 响应。

2. **路由** -- 在 `main.go` 中注册到合适的路由组（`pub` 公开、`passkeyOnly` 仅 Passkey、`auth` 双认证）。

3. **模型**（如需要）-- 在 `model/` 中添加 GORM 模型。AutoMigrate 会在下次启动时自动建表。

4. **测试** -- 在 `handler/*_test.go` 中添加测试，使用内存 SQLite。

5. **文档** -- 更新 `docs/en/` 和 `docs/zh/` 的相关页面，以及 `docs/api/openapi.yaml`。

### main.go 中的路由组

```go
pub := r.Group("/api")           // 无需认证
passkeyOnly := auth.Group("")    // 仅 Passkey 会话
auth := r.Group("/api")          // API Key 或 Passkey 会话
auth.Use(handler.AuthMiddleware) // 双认证中间件
```

---

## 数据库

SQLite WAL 模式，启动时自动迁移：

| 表 | 模型 | 用途 |
|----|------|------|
| `users` | `User` | 注册用户 |
| `passkey_credentials` | `PasskeyCredential` | WebAuthn 凭证 |
| `api_keys` | `APIKey` | API 密钥（哈希存储，`ocw_` 前缀） |
| `wallets` | `Wallet` | 钱包：链、地址、公钥 |
| `approval_policies` | `ApprovalPolicy` | USD 阈值和日限额 |
| `approval_requests` | `ApprovalRequest` | 待审批/已审批/已拒绝请求 |
| `allowed_contracts` | `AllowedContract` | 每个钱包的合约白名单 |
| `audit_logs` | `AuditLog` | 完整操作审计记录 |
| `idempotency_records` | `IdempotencyRecord` | Idempotency-Key 缓存（24 小时 TTL） |
| `custom_chains` | `CustomChain` | 用户添加的 EVM 链 |

---

## 签名流程（内部）

```
handler (wallet.go / contract_call.go)
  → 构建交易 (chain/tx_eth.go 或 chain/tx_sol.go)
  → 检查审批策略（阈值、日限额）
  → 需要审批：创建 ApprovalRequest，返回 202
  → 直接执行：sdk.Sign() → 广播 → 返回 tx_hash

handler (approval.go) 审批通过时：
  → 验证即时 Passkey 断言
  → 重新构建交易（刷新 nonce 和 gas）
  → sdk.Sign() → 广播 → 更新 ApprovalRequest 的 tx_hash
```

审批路径在审批时（而非请求时）重新构建交易，确保使用最新的 nonce 和 gas 估算。

---
[上一页: API 概览](/zh/api-overview.md) | [下一页: 架构与安全](/zh/whitepaper.md)
