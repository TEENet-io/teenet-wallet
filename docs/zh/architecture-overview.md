# 架构概览

本页提供 TEENet Wallet 的整体结构、核心术语以及各组件之间协作方式的概览。

---

## 核心术语

| 术语 | 定义 |
|------|------|
| **Wallet** | 与特定链绑定的账户，由 TEE 管理的密钥支撑。每个 Wallet 拥有一个地址、一对密钥，并归属于一个用户。 |
| **Chain** | 一条区块链网络（如 `ethereum`、`solana`、`avalanche-c`）。可在 `chains.json` 中预配置，也可在运行时动态添加。 |
| **Approval policy** | 控制交易何时需要 Passkey 审批的规则。审批策略包括一个 USD 阈值（超过该阈值的代币转账需审批）、一个可选的每日消费限额（UTC 午夜重置），以及一个合约白名单（仅列出的合约地址允许被调用）。 |
| **API key** | 以 `ocw_` 为前缀的 Bearer Token，供 Agent 和自动化流程使用。可执行大多数操作，但敏感操作（超阈值转账、合约白名单变更）会创建待审批请求，需通过 Passkey 确认。 |
| **TEENet service** | 钱包所依托的底层平台。通过 SDK 提供密钥生成、门限签名和 Passkey 管理功能。钱包仅通过 SDK 与之交互 -- 从不直接访问 TEE 节点。本地开发时，可使用 Mock 服务（`teenet-sdk/mock-server`）模拟相同的 SDK 接口，执行真实的密码学运算但使用确定性密钥。 |
| **TEE node** | 运行在可信执行环境（如 Intel SGX、AMD SEV）中的服务器。TEE 节点持有密钥分片，并参与门限签名。 |

---

## 系统组件

TEENet Wallet 是一个单一 Go 二进制文件，内部有清晰的分层结构：

```
┌─────────────────────────────────────────────────────┐
│  main.go                                            │
│  Route registration, middleware wiring, DI setup    │
└──────────────┬──────────────────────────────────────┘
               │
┌──────────────▼──────────────────────────────────────┐
│  handler/                                           │
│  HTTP handlers — one file per domain                │
│  (wallet, approval, auth, contract, balance, ...)   │
└──────────────┬──────────────────────────────────────┘
               │
       ┌───────┴────────┐
       │                │
┌──────▼──────┐  ┌──────▼──────────────────────┐
│  chain/     │  │  TEENet SDK                  │
│  Tx build,  │  │  (github.com/TEENet-io/      │
│  RPC calls, │  │   teenet-sdk/go)             │
│  address    │  │  Key gen, signing, passkeys  │
│  derivation │  └──────────┬──────────────────┘
└─────────────┘             │
                    ┌───────▼───────┐
                    │  TEENet       │
                    │  Service      │
                    │  (or mock)    │
                    └───────────────┘
```

**存储：** SQLite WAL 模式，通过 GORM 管理。所有表在启动时自动迁移。

**前端：** 一个单文件 SPA（`frontend/index.html`），由同一个 Go 二进制文件提供服务。用于 Passkey 注册、登录和审批确认。

---

## 签名流程

```
 AI Agent / 应用                        用户（浏览器）
     |  API Key (ocw_*)                    |  Passkey
     v                                     v
+--------------------------------------------------------------+
|  TEENet Wallet  (:8080)                                      |
|  REST API · approval policies · contract whitelist            |
+--------------------------------------------------------------+
     |  TEENet SDK
     v
+--------------------------------------------------------------+
|  TEENet Service                                              |
|  Threshold signing · keys never leave TEE hardware            |
+--------------------------------------------------------------+
```

1. 客户端发送请求（API Key 或 Passkey）。
2. 钱包检查白名单、阈值和每日限额。
3. 如需审批，请求进入待审批状态，等待所有者用 Passkey 确认。
4. 钱包将签名请求路由到 TEE 集群。
5. TEE 节点生成门限签名 -- 完整私钥从未被还原。
6. 钱包广播已签名交易并返回交易哈希。

---

## 核心工作流

### 创建钱包

```
POST /api/wallets {chain}
        │
        ▼
  Save wallet record ──► status: "creating"
        │
        ▼
  SDK GenerateKey(scheme, curve)
        │
        ▼
  Derive chain address from public key
        │
        ▼
  Update wallet record ──► status: "ready"
```

### 发送转账

```
POST /api/wallets/:id/transfer {to, amount}
        │
        ▼
  Build unsigned transaction
        │
        ▼
  Check approval policy ─── exceeds threshold ──► Save as pending
        │                                          approval,
        │                                          return HTTP 202
        ├── exceeds daily limit ──► Reject request (HTTP 400)
        ▼                                         + approval_url
  SDK Sign(tx, keyName)
        │
        ▼
  Broadcast to blockchain
        │
        ▼
  Return tx hash
```

### 审批待处理请求

```
Pending approval appears in Web UI (via SSE)
        │
        ▼
  User confirms with Passkey (tap / biometric)
        │
        ▼
  Verify Passkey assertion via TEENet service
        │
        ▼
  Rebuild transaction (fresh nonce + gas estimate)
        │
        ▼
  SDK Sign(tx, keyName) ──► Broadcast ──► Return tx hash
```

### 设置审批策略

```
PUT /api/wallets/:id/policy {threshold_usd, daily_limit_usd, enabled}
        │
        ▼
  Called via API key? ─── yes ──► Save as pending approval
        │                         (policy changes require
        │                          Passkey confirmation)
        ▼
  Called via Passkey ──► Apply policy immediately
```

未设置策略时，所有转账立即签名。设置后，超过阈值的转账需要 Passkey 审批，而超出每日限额的转账会被直接拒绝。

---

## 为什么 Agent 应选择 TEENet Wallet？

| 方案 | 取舍 | TEENet Wallet |
|---|---|---|
| **将私钥共享给 Agent** | Agent 拥有完全访问权限 -- 一个 Bug 或提示词注入即可清空钱包 | Agent 在可配置的消费限额内操作；高价值操作需 Passkey 审批 |
| **多签钱包** | 每次审批消耗 Gas，需多个签名者同时在线，体验差 | 审批只需一次 Passkey 点击 -- 无 Gas 消耗、无需协调、亚秒级完成 |
| **托管 API 服务** | 使用方便，但需信任服务商保管密钥 | 密钥分片分布在 TEE 节点 -- 任何单方（包括运营者）都无法持有完整密钥 |
| **每条链单独集成** | 每条链需要独立的钱包部署和密钥管理 | 以太坊、Solana 及所有主要 EVM 链通过一个 API 统一管理 |

---

## 后续阅读

- [TEENet SDK 使用指南](sdk-usage.md) -- 钱包如何在代码中使用 SDK
- [签名与 TEE 信任模型](signing-tee.md) -- TEENet service 内部的运作机制
- [数据模型](data-model.md) -- 数据库表及其关系
