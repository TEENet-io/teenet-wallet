# 开发者指南

本指南面向需要编译、测试和参与 teenet-wallet 代码开发的开发者。

---

## 构建与运行

**前置条件：** Go 1.24+、SQLite3 开发头文件、一个运行中的 TEENet 服务节点（端口 8089）。

```bash
# 编译
make build

# 运行（默认端口 8080，数据目录 /data，consensus 地址 localhost:8089）
./teenet-wallet

# 或用 Docker
make docker
docker run -p 8080:8080 \
  -e SERVICE_URL=http://host.docker.internal:8089 \
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
│   ├── sse.go           # SSE 事件流端点
│   ├── sse_hub.go       # 按用户分发的 SSE 发布/订阅中心
│   ├── approval.go      # 审批列表/详情/批准/拒绝、审批后执行
│   ├── balance.go       # 链上余额查询（原生 + ERC-20 + SPL）
│   ├── addressbook.go   # 地址簿增删改查、昵称解析
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
│   ├── addressbook.go   # AddressBookEntry
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
│       └── SKILL.md     # OpenClaw 技能定义（REST 方式）
├── plugin/              # OpenClaw 插件（TypeScript，原生工具集成）
│   ├── index.ts         # 插件入口：注册工具 + SSE 审批监听
│   ├── openclaw.plugin.json  # 插件清单（id、配置 schema、skill 列表）
│   ├── src/
│   │   ├── api-client.ts       # 钱包后端 HTTP 客户端
│   │   ├── approval-watcher.ts # SSE 订阅 + subagent.run() 通知
│   │   ├── tools/              # 工具定义（wallet、transfer、contract、policy 等）
│   │   └── __tests__/          # 单元 + E2E 测试（node --test）
│   └── skill/tee-wallet/       # 随插件分发的 Agent 指令
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
- **TEENet SDK** -- 签名通过 `github.com/TEENet-io/teenet-sdk/go` 发送到本地 TEENet 服务节点。

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

测试使用内存 SQLite（`file::memory:`），不需要运行中的 TEENet 服务节点 -- SDK 客户端在测试中为 nil，签名调用预期失败（测试验证签名前的行为）。

### Mock TEENet Service（模拟 TEENet 服务）

如果本地没有真实的 TEENet 服务，可以使用 [teenet-sdk](https://github.com/TEENet-io/teenet-sdk) 自带的 mock server（位于 [`mock-server/`](https://github.com/TEENet-io/teenet-sdk/tree/main/mock-server)）。它实现了完整的 TEENet HTTP API 并提供真实的密码学签名，只需将 `SERVICE_URL` 指向它，钱包就能像对接生产环境一样工作。

```bash
git clone https://github.com/TEENet-io/teenet-sdk.git
cd teenet-sdk/mock-server
go build && ./mock-server                                  # 127.0.0.1:8089
# 如需自定义端口/绑定：MOCK_SERVER_PORT=xxxx MOCK_SERVER_BIND=0.0.0.0 ./mock-server
# 注意：改了端口后，下面的 SERVICE_URL 也要同步更新
```

然后运行钱包：

```bash
SERVICE_URL=http://127.0.0.1:8089 ./teenet-wallet
```

**提供的能力（共 34 个端点）：**

- **核心签名 & 密钥** -- `/api/health`、`/api/publickeys/:app_instance_id`、`/api/submit-request`、`/api/generate-key`、`/api/apikey/:name`、`/api/apikey/:name/sign`
- **投票缓存** -- `/api/cache/:hash`、`/api/cache/status`、`/api/config/:app_instance_id`
- **审批桥接** -- `/api/auth/passkey/options|verify|verify-as`、`/api/approvals/request/init`、`/api/approvals/request/:id/challenge|confirm`、`/api/approvals/:taskId/challenge|action`、`/api/approvals/pending`、`/api/requests/mine`、`/api/signature/by-tx/:txId`
- **管理桥接** -- Passkey 用户邀请/列表/删除、审计记录、权限策略 CRUD、公钥/API Key 管理、Passkey 注册

**签名模式**（由 `app_instance_id` 决定）：

| 模式 | 示例 app | 行为 |
|------|---------|------|
| Direct（直签） | `test-ecdsa-secp256k1`、`ethereum-wallet-app` | 立即签名，返回 `{status: "signed", signature}` |
| Voting（投票） | `test-voting-2of3` | 首次返回 `pending`；需要 2 个不同实例投票后完成 |
| Approval（审批） | `test-approval-required` | 返回 `pending_approval` + `tx_id`；需通过 `/api/approvals/:taskId/action` 完成 Passkey 审批 |

**预置的测试 App：**

| App Instance ID | 协议 | 曲线 | 模式 |
|-----------------|------|------|------|
| test-schnorr-ed25519 | schnorr | ed25519 | Direct |
| test-schnorr-secp256k1 | schnorr | secp256k1 | Direct |
| test-ecdsa-secp256k1 | ecdsa | secp256k1 | Direct |
| test-ecdsa-secp256r1 | ecdsa | secp256r1 | Direct |
| ethereum-wallet-app | ecdsa | secp256k1 | Direct |
| secure-messaging-app | schnorr | ed25519 | Direct |
| test-voting-2of3 | ecdsa | secp256k1 | Voting (2-of-N) |
| test-approval-required | ecdsa | secp256k1 | Approval |

预置 Passkey 用户：**Alice**（ID=1）、**Bob**（ID=2），均绑定到 `test-approval-required`。

> **关于 `app_instance_id`。** 上表中"协议/曲线"列描述的只是每个测试 app **初始绑定**的那把密钥。`app_instance_id` 并不锁定到某条链或某把密钥 —— 你可以用 `POST /api/generate-key` 在任何 id 下追加新的密钥(任意协议/曲线组合),然后用 `POST /api/submit-request` 指定密钥名签名。选 id 时只看模式(Direct / Voting / Approval)即可;只有当你用默认初始密钥签名时,初始密钥的曲线才有意义。

**哈希职责**（与 TEE-DAO 后端一致）：

| 协议 | 曲线 | 谁来哈希？ |
|------|------|-----------|
| ECDSA | secp256k1 / secp256r1 | **调用方** -- 必须传入 32 字节哈希（以太坊用 Keccak-256，其他用 SHA-256） |
| Schnorr | secp256k1 | Mock server 内部（SHA-256） |
| Schnorr | ed25519 | EdDSA 协议内部（SHA-512） |
| HMAC | -- | HMAC 内部（SHA-256） |

**限制：** 全部数据保存在内存中（重启即清空），使用确定性私钥以便签名可复现，审批 Token 使用随机 HMAC 密钥，TTL 为 30 分钟。**请勿用于生产环境。**

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
| `address_book_entries` | `AddressBookEntry` | 地址簿（按用户/链唯一昵称） |
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
