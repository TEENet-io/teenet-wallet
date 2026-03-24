# TEENet Wallet 产品指南

## 什么是 TEENet Wallet？

TEENet Wallet 是一款基于可信执行环境（TEE）技术的多链加密货币钱包。与传统钱包将完整私钥存储在单台设备上不同，TEENet Wallet 将私钥通过门限密码学分散到多个 TEE 硬件节点中。私钥在任何时刻都不会以完整形式存在于任何单一机器上——签名操作需要多个 TEE 节点协同完成（例如 3-of-5 门限签名），从根本上消除了私钥泄露的单点风险。

TEENet Wallet 支持以太坊（及所有 EVM 兼容链）和 Solana 两大生态，内置 ERC-20 代币转账、SPL 代币转账、智能合约交互、ABI 编码等功能。钱包采用双重认证模型：API Key 供 AI Agent 和自动化程序使用，Passkey（WebAuthn 硬件认证）供人类操作者审批高危交易。所有转账金额均自动转换为 USD 计价，结合可配置的审批阈值和日限额，实现精细化的风控策略。

TEENet Wallet 的目标用户包括：需要自主管理链上资产的 **AI Agent**（通过 API Key 自动执行交易）、追求无人值守运行的 **DeFi 自动化系统**（合约白名单 + 自动审批模式）、以及对资产安全有极高要求的 **机构级托管方案**（Passkey 硬件审批 + 门限签名 + 审计日志）。

---

## 核心功能

### 多链支持

- **以太坊主网及所有 EVM 兼容链**：Ethereum、Optimism、Sepolia、Holesky、Base Sepolia、BSC Testnet，以及通过 `chains.json` 或运行时 API 添加的任意自定义 EVM 链
- **Solana 生态**：Solana Mainnet、Solana Devnet
- EVM 链使用 ECDSA (secp256k1) 签名协议，Solana 使用 Schnorr (ed25519) 签名协议
- 支持运行时动态添加/删除自定义 EVM 链（持久化到数据库，重启不丢失）

### 双重认证体系

- **API Key**（`ocw_` 前缀）：面向 AI Agent 和程序化自动操作，可配置每分钟请求速率限制
- **Passkey 会话**（`ps_` 前缀）：基于 WebAuthn 硬件认证，用于高价值交易审批、钱包删除、策略变更等敏感操作
- 每次审批操作都需要即时的硬件 Passkey 验证，即使会话令牌被盗也无法越权审批

### USD 计价审批阈值与实时价格

- 每个钱包可设置独立的 USD 审批阈值（`threshold_usd`）和日限额（`daily_limit_usd`）
- 转账金额通过 CoinGecko 实时价格自动换算为 USD（稳定币按 $1 计价）
- 单笔交易超过阈值自动进入 Passkey 审批队列；超过日限额则硬性拒绝
- 日限额采用"预扣 + 回退"模式（auth/capture），签名或广播失败时自动退回额度

### 合约白名单三层安全模型

1. **合约白名单**：仅预先批准的合约地址（EVM）、代币铸造地址（SPL）或程序 ID（Solana）可被调用
2. **方法级限制**：可选的 `allowed_methods` 列表限定允许调用的合约函数（EVM）
3. **高危方法强制审批**：`approve`、`transferFrom`、`increaseAllowance`、`setApprovalForAll`、`safeTransferFrom` 等方法始终需要 Passkey 审批，即使合约已开启 `auto_approve`
4. **自动审批模式**：白名单中标记 `auto_approve: true` 的合约允许 API Key 直接执行非高危操作

### 智能合约交互

- **EVM 合约调用**：完整的 ABI 编码支持，包括 `address`、`bool`、`uintN`、`intN`、`bytesN`、`bytes`、`string`、动态数组、定长数组（`T[N]`）和元组（tuple）
- **Solana 程序调用**：通过 `accounts` 和 `data` 字段构建任意程序指令
- **只读调用**：`call-read` 端点查询链上状态，无需签名和 Gas
- **便捷端点**：`approve-token`、`revoke-approval` 自动处理 ABI 编码
- **Wrap/Unwrap SOL**：原生 SOL 与 wSOL 之间的便捷转换

### 日限额（预扣 + 回退模式）

- 日限额以 UTC 日历日为周期，跨越 UTC 午夜自动重置
- 转账发起时预扣额度，签名或广播失败后自动回退，防止"幽灵消费"
- 超过日限额的转账被硬性拒绝，无审批路径

### 其他功能

- **幂等性转账**：通过 `Idempotency-Key` HTTP 头防止重复提交
- **EIP-1559 交易**：动态 Gas 费估算
- **Nonce 管理器**：支持 EVM 并发交易安全
- **SPL 自动 ATA 创建**：接收方无 ATA 时自动在同一交易中创建
- **审计日志**：所有钱包操作全程记录
- **内置 Web UI**：账户管理和交易审批一站式界面

---

## 系统架构

```
+------------------------------------------+      +---------------------------+
|  应用 / AI Agent                          |      |  用户 (浏览器)              |
|  认证方式: API Key (ocw_*)               |      |  认证方式: Passkey (ps_*)   |
+------------------+-----------------------+      +-------------+-------------+
                   |                                            |
                   |          HTTP REST API                     |
                   +--------------------+-----------------------+
                                        |
                                        v
          +------------------------------------------------------------+
          |  TEENet Wallet  (:8080)                                    |
          |                                                            |
          |  - 构建交易 (EIP-1559 / Solana)                            |
          |  - 执行合约白名单 + 方法门控                                 |
          |  - 管理审批策略 + 日限额 (USD 计价)                         |
          |  - 路由至审批队列或直接签名                                  |
          |  - 广播已签名交易至区块链                                    |
          +-----------------------------+------------------------------+
                                        |
                                        |  TEENet SDK (HTTP)
                                        v
          +------------------------------------------------------------+
          |  app-comm-consensus  (:8089)                                |
          |                                                            |
          |  - M-of-N 投票协调                                         |
          |  - 签名请求转发                                             |
          +-----------------------------+------------------------------+
                                        |
                                        |  gRPC + mTLS
                                        v
          +------------------------------------------------------------+
          |  TEE-DAO Key Management Cluster                            |
          |  (3-5 个 TEE 节点)                                         |
          |                                                            |
          |  - 门限签名 (FROST / GG20)                                 |
          |  - 分布式密钥生成 (DKG)                                     |
          |  - 私钥分片永远不离开 TEE 硬件                               |
          +------------------------------------------------------------+
```

### 签名流程

1. 应用或 AI Agent 向 TEENet Wallet 发送请求，使用 API Key 或 Passkey 会话认证
2. TEENet Wallet 验证请求：检查合约白名单、方法限制、审批策略
3. 若金额超过阈值、调用高危方法、或未启用自动审批，交易进入待审批状态，需通过 Web UI 进行 Passkey 硬件审批
4. TEENet Wallet 通过 TEENet SDK 向本地 app-comm-consensus 节点请求门限签名
5. TEE-DAO 集群执行分布式签名——私钥永远不会在任何单一机器上被重建
6. TEENet Wallet 将已签名交易广播至区块链

---

## 快速开始

### 环境要求

| 依赖项 | 版本/说明 |
|--------|----------|
| Go | 1.24 或更高版本 |
| SQLite3 开发头文件 | Debian/Ubuntu: `apt-get install libsqlite3-dev` |
| TEENet Mesh 节点 | 需要运行中的 app-comm-consensus（端口 8089） |
| Docker（可选） | 用于容器化部署 |

### 安装部署

**方式一：源码编译**

```bash
# 克隆仓库
git clone <repository-url>
cd teenet-wallet

# 编译
make build

# 启动（默认监听 0.0.0.0:8080）
./teenet-wallet
```

**方式二：Docker 部署**

```bash
# 构建镜像
make docker

# 运行容器
docker run -p 8080:8080 \
  -e CONSENSUS_URL=http://host.docker.internal:8089 \
  -v wallet-data:/data \
  teenet-wallet:latest
```

服务启动后，访问 `http://localhost:8080` 即可看到内置 Web UI。

### 创建第一个钱包

**第 1 步：注册账户并获取 API Key**

通过 Web UI（`http://localhost:8080`）完成 Passkey 注册，然后在界面中生成 API Key。

或者通过 API 注册流程：

```bash
# 开始注册（获取 WebAuthn 挑战）
curl -s -X POST http://localhost:8080/api/auth/passkey/register/begin \
  -H "Content-Type: application/json" \
  -d '{"display_name": "my-account"}'

# 完成注册后，登录并生成 API Key（需要 Passkey 会话）
# 建议通过 Web UI 完成此步骤
```

**第 2 步：查看支持的链**

```bash
export TEE_WALLET_URL=http://localhost:8080
export API_KEY=ocw_your_api_key_here

curl -s "${TEE_WALLET_URL}/api/chains"
```

返回示例：

```json
{
  "success": true,
  "chains": [
    {"name": "ethereum", "label": "Ethereum Mainnet", "currency": "ETH", "family": "evm"},
    {"name": "solana", "label": "Solana Mainnet", "currency": "SOL", "family": "solana"},
    {"name": "sepolia", "label": "Sepolia Testnet", "currency": "ETH", "family": "evm"}
  ]
}
```

**第 3 步：创建钱包**

```bash
# 创建 Sepolia 测试网钱包
curl -s -X POST "${TEE_WALLET_URL}/api/wallets" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"chain": "sepolia", "label": "My Test Wallet"}'
```

返回示例：

```json
{
  "success": true,
  "wallet": {
    "id": "8a2fbc16-faf4-451a-be34-9fc5c49cde00",
    "chain": "sepolia",
    "address": "0x1234...abcd",
    "label": "My Test Wallet",
    "status": "ready"
  }
}
```

> 注意：以太坊（ECDSA）钱包创建涉及分布式密钥生成（DKG），可能需要 1-2 分钟。Solana 钱包通常即时完成。

### 发送第一笔交易

```bash
WALLET_ID=8a2fbc16-faf4-451a-be34-9fc5c49cde00

# 查询余额
curl -s "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/balance" \
  -H "Authorization: Bearer ${API_KEY}"

# 发送 ETH（确保钱包有足够余额）
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/transfer" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "0.01"
  }'
```

成功返回：

```json
{
  "status": "completed",
  "tx_hash": "0xabc123...",
  "chain": "sepolia",
  "amount": "0.01",
  "currency": "ETH"
}
```

若金额超过审批阈值，返回 `"status": "pending_approval"`，需在 Web UI 中通过 Passkey 审批。

### 设置审批策略

```bash
# 设置 USD 审批阈值和日限额
curl -s -X PUT "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/policy" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "threshold_usd": 100,
    "daily_limit_usd": 5000,
    "enabled": true
  }'
```

> 通过 API Key 设置策略会创建一个待审批请求（HTTP 202），钱包所有者需在 Web UI 中通过 Passkey 确认后策略才会生效。通过 Passkey 会话设置则立即生效。

---

## 配置参考

所有配置通过环境变量完成，无需配置文件：

| 环境变量 | 默认值 | 说明 |
|---------|-------|------|
| `CONSENSUS_URL` | `http://localhost:8089` | 本地 app-comm-consensus 节点地址 |
| `HOST` | `0.0.0.0` | 服务绑定地址 |
| `PORT` | `8080` | HTTP 监听端口 |
| `DATA_DIR` | `/data` | SQLite 数据库存储目录 |
| `BASE_URL` | `http://localhost:<PORT>` | 公网访问地址（用于审批链接生成） |
| `FRONTEND_URL` | （空） | 允许的 CORS 来源地址；为空则不发送 CORS 头 |
| `CHAINS_FILE` | `./chains.json` | 链配置文件路径 |
| `APP_INSTANCE_ID` | （来自 TEENet） | TEENet 应用实例标识符 |
| `API_KEY_RATE_LIMIT` | `200` | 每个 API Key 每分钟最大请求数 |
| `WALLET_CREATE_RATE_LIMIT` | `5` | 每个 Key 每分钟最大钱包创建数（DKG 资源密集） |
| `REGISTRATION_RATE_LIMIT` | `10` | 每个 IP 每分钟最大注册尝试次数 |
| `APPROVAL_EXPIRY_MINUTES` | `30` | 待审批请求的过期时间（分钟） |
| `MAX_WALLETS_PER_USER` | `20` | 每个用户可创建的最大钱包数 |

区块链 RPC URL 在 `chains.json` 文件中定义，不作为独立环境变量。可通过 `CHAINS_FILE` 环境变量指定自定义路径。也可在运行时通过 `POST /api/chains` 动态添加自定义 EVM 链。

---

## 认证体系

TEENet Wallet 采用双层认证模型，分别服务于程序化访问和人类操作两种场景。

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
- 不可执行：删除钱包、删除审批策略、审批/拒绝请求、撤销 API Key（仅 Passkey）

**速率限制：** 默认每分钟 200 次请求，钱包创建每分钟 5 次（可通过环境变量调整）。

**管理操作：**

```bash
# 列出所有 API Key
curl -s "${TEE_WALLET_URL}/api/auth/apikey/list" \
  -H "Authorization: Bearer ps_session_token"

# 撤销 API Key（仅 Passkey 会话）
curl -s -X DELETE "${TEE_WALLET_URL}/api/auth/apikey" \
  -H "Authorization: Bearer ps_session_token" \
  -H "X-CSRF-Token: csrf_token_value" \
  -H "Content-Type: application/json" \
  -d '{"key_prefix": "ocw_xxxx"}'
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
- 邀请新用户、删除账户

### CSRF 保护

所有通过 Passkey 会话发起的状态变更请求（POST、PUT、DELETE）都需要携带 CSRF 令牌：

```
X-CSRF-Token: <csrf_token_value>
```

CSRF 令牌在登录时返回。API Key 请求不受 CSRF 保护约束。

---

## 钱包管理

### 创建钱包

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/wallets" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"chain": "ethereum", "label": "主钱包"}'
```

**参数说明：**
- `chain`（必填）：链名称，必须是 `GET /api/chains` 返回的有效链名
- `label`（可选）：钱包的人类可读标签

**注意事项：**
- EVM 链（ECDSA）钱包创建需要分布式密钥生成，耗时约 1-2 分钟
- Solana（Schnorr/Ed25519）钱包通常即时完成
- 每个用户最多创建 20 个钱包（可通过 `MAX_WALLETS_PER_USER` 调整）
- 钱包创建受独立速率限制（默认每分钟 5 个）

### 列出钱包

```bash
curl -s "${TEE_WALLET_URL}/api/wallets" \
  -H "Authorization: Bearer ${API_KEY}"
```

### 获取钱包详情

```bash
curl -s "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}" \
  -H "Authorization: Bearer ${API_KEY}"
```

### 重命名钱包

```bash
curl -s -X PATCH "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"label": "新名称"}'
```

### 删除钱包

```bash
# 仅 Passkey 会话可执行（不可逆操作）
curl -s -X DELETE "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}" \
  -H "Authorization: Bearer ps_session_token" \
  -H "X-CSRF-Token: csrf_token_value"
```

> 警告：删除钱包是不可逆操作。删除前请确保钱包中没有剩余资产。

### 链选择

创建钱包时，`chain` 字段决定了钱包所属的区块链网络：

- EVM 链族（`family: "evm"`）：使用 ECDSA/secp256k1 协议，地址格式为 `0x...`
- Solana 链族（`family: "solana"`）：使用 Schnorr/ed25519 协议，地址格式为 base58

可通过 `GET /api/chains` 动态获取当前可用链列表，包括内置链和运行时添加的自定义链。

---

## 转账

### 原生代币（ETH / SOL）

发送原生代币（ETH、SOL、tBNB 等）到指定地址：

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/transfer" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "0.5",
    "memo": "付款备注（可选）"
  }'
```

**参数说明：**
- `to`（必填）：接收方地址（EVM 为 `0x...` 格式，Solana 为 base58 格式）
- `amount`（必填）：转账金额，人类可读单位（如 `"0.5"` 表示 0.5 ETH）
- `memo`（可选）：交易备注

**返回结果：**

```json
{
  "status": "completed",
  "tx_hash": "0xabc123...",
  "chain": "ethereum",
  "amount": "0.5",
  "currency": "ETH"
}
```

若需要审批，返回：

```json
{
  "status": "pending_approval",
  "approval_id": 42,
  "approval_url": "http://localhost:8080/#/approve/42"
}
```

### ERC-20 代币转账

发送 ERC-20 代币时，**必须**在请求中包含 `token` 字段：

```bash
# 前提：代币合约已在白名单中
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/transfer" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "100",
    "token": {
      "contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
      "symbol": "USDC",
      "decimals": 6
    }
  }'
```

> **警告**：遗漏 `token` 字段将发送原生 ETH 而非代币——这是完全不同的交易，无法撤回。请务必确认请求体中包含 `"token": {...}`。

**`amount` 为人类可读单位**：`"100"` 表示 100 USDC，后端自动根据 `decimals` 转换为链上原始单位。

**常见 ERC-20 代币参数：**

| 网络 | 代币 | 合约地址 | 精度 |
|------|------|---------|------|
| Ethereum 主网 | USDC | `0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48` | 6 |
| Ethereum 主网 | USDT | `0xdac17f958d2ee523a2206206994597c13d831ec7` | 6 |
| Ethereum 主网 | WETH | `0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2` | 18 |
| Ethereum 主网 | DAI | `0x6b175474e89094c44da98b954eedeac495271d0f` | 18 |
| Sepolia 测试网 | USDC | `0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238` | 6 |
| Sepolia 测试网 | WETH | `0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14` | 18 |
| Base Sepolia | USDC | `0x036CbD53842c5426634e7929541eC2318f3dCF7e` | 6 |

### SPL 代币转账

Solana 上的 SPL 代币转账同样使用 `/transfer` 端点，通过 `token` 字段指定代币铸造地址：

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/transfer" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "RecipientBase58Address...",
    "amount": "50",
    "token": {
      "contract": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
      "symbol": "USDC",
      "decimals": 6
    }
  }'
```

**Solana 特有行为：** 如果接收方没有该代币的关联代币账户（ATA），后端会在同一交易中自动创建。无需额外操作。

代币铸造地址必须先加入白名单（与 EVM 合约白名单使用同一接口）。

### Wrap / Unwrap SOL

将原生 SOL 转换为 wSOL（Wrapped SOL SPL 代币），供 DeFi 协议使用：

```bash
# Wrap SOL（原生 SOL -> wSOL）
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/wrap-sol" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"amount": "0.1"}'

# Unwrap SOL（wSOL -> 原生 SOL，关闭 wSOL ATA，全部余额）
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/unwrap-sol" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{}'
```

Wrap 操作会自动创建 wSOL ATA（如果不存在）。Unwrap 操作关闭整个 wSOL ATA，将全部 wSOL 余额转回原生 SOL。

### 幂等性

通过 `Idempotency-Key` HTTP 头实现转账请求的幂等性，防止因网络重试导致的重复交易：

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/transfer" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: unique-request-id-12345" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "0.01"
  }'
```

相同的 `Idempotency-Key` 在有效期内会返回首次请求的结果，不会重复执行交易。

---

## 智能合约交互

### 合约白名单

合约白名单是安全门控：所有合约调用（包括 ERC-20/SPL 代币转账）都必须先将目标合约/铸造地址/程序 ID 加入白名单。

**列出白名单合约：**

```bash
curl -s "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/contracts" \
  -H "Authorization: Bearer ${API_KEY}"
```

**添加合约到白名单：**

```bash
# 通过 API Key 添加（创建待审批请求，返回 HTTP 202）
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/contracts" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract_address": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
    "symbol": "USDC",
    "decimals": 6,
    "label": "USDC Stablecoin",
    "allowed_methods": "transfer,balanceOf",
    "auto_approve": false
  }'
```

通过 API Key 添加返回 202，表示需要 Passkey 所有者审批：

```json
{
  "success": true,
  "pending": true,
  "approval_id": 7,
  "message": "Contract whitelist request submitted for approval"
}
```

通过 Passkey 会话添加则立即生效。

**更新白名单条目：**

```bash
curl -s -X PUT "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/contracts/${CONTRACT_ID}" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "auto_approve": true,
    "allowed_methods": "transfer,balanceOf,approve"
  }'
```

**删除白名单条目（仅 Passkey）：**

```bash
curl -s -X DELETE "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/contracts/${CONTRACT_ID}" \
  -H "Authorization: Bearer ps_session_token" \
  -H "X-CSRF-Token: csrf_token_value"
```

**白名单字段说明：**

| 字段 | 必填 | 说明 |
|-----|------|------|
| `contract_address` | 是 | EVM 合约地址（`0x...`）或 Solana 铸造/程序地址（base58） |
| `symbol` | 否 | 代币符号（如 USDC） |
| `decimals` | 否 | 代币精度（如 USDC 为 6，WETH 为 18，大多数 SPL 为 9） |
| `label` | 否 | 人类可读标签 |
| `allowed_methods` | 否 | 逗号分隔的允许方法名（EVM）；为空表示允许所有方法 |
| `auto_approve` | 否 | 设为 `true` 则 API Key 可直接调用（高危方法除外）。默认 `false` |

### 合约调用（EVM）

调用已白名单的 EVM 智能合约函数：

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/contract-call" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xContractAddress...",
    "func_sig": "transfer(address,uint256)",
    "args": ["0xRecipient...", "1000000"],
    "value": "0",
    "amount_usd": "150.00",
    "memo": "DeFi 操作"
  }'
```

**参数说明：**
- `contract`（必填）：目标合约地址（必须在白名单中）
- `func_sig`（必填）：Solidity 风格函数签名，如 `transfer(address,uint256)`
- `args`（必填）：参数数组，按函数签名顺序排列
- `value`（可选）：附带发送的 ETH 数量
- `amount_usd`（可选）：此调用涉及的 USD 价值，用于阈值和日限额判断
- `memo`（可选）：备注

**支持的参数类型：** `address`、`uint256`（及其他 `uintN`）、`int256`（及其他 `intN`）、`bool`、`bytes32`（及其他 `bytesN`）、`bytes`、`string`、动态数组、定长数组（`T[N]`）、元组（tuple）。

支持复杂 DeFi 调用，例如 Uniswap V3 的 `exactInputSingle` 含元组参数和定长数组。

### 程序调用（Solana）

调用已白名单的 Solana 程序：

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/contract-call" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "ProgramIdBase58...",
    "accounts": [
      {"pubkey": "Account1Base58...", "is_signer": false, "is_writable": true},
      {"pubkey": "Account2Base58...", "is_signer": false, "is_writable": false}
    ],
    "data": "hex_encoded_instruction_data",
    "amount_usd": "50.00",
    "memo": "Solana 程序调用"
  }'
```

**参数说明：**
- `contract`（必填）：Solana 程序 ID（base58，必须在白名单中）
- `accounts`（必填）：账户元数据数组，按指令顺序排列；钱包地址作为签名者自动添加
- `data`（必填）：十六进制编码的指令数据（鉴别器 + 编码参数）
- `amount_usd`（可选）：USD 价值申报

### 只读调用

查询链上合约状态，无需签名、无 Gas 消耗、无需审批（仅 EVM）：

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/call-read" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xContractAddress...",
    "func_sig": "balanceOf(address)",
    "args": ["0xWalletAddress..."]
  }'
```

返回示例：

```json
{
  "success": true,
  "result": "0x0000000000000000000000000000000000000000000000000000000005f5e100",
  "contract": "0x...",
  "method": "balanceOf"
}
```

常见用途：
- 查询代币余额：`balanceOf(address)`
- 查询授权额度：`allowance(address,address)`
- 读取合约状态：`totalSupply()`、`name()`、`symbol()`、`decimals()`

### amount_usd 阈值申报

当合约调用涉及价值转移（如 DeFi 交换、代币转账）时，应在请求中包含 `amount_usd` 字段申报近似 USD 价值，以便钱包执行阈值和日限额策略：

```json
{
  "contract": "0x...",
  "func_sig": "swap(address,uint256)",
  "args": ["0x...", "1000000"],
  "amount_usd": "1500.00"
}
```

**规则：**
- 若同时存在 `value`（原生 ETH）和 `amount_usd`，钱包取两者中较大的 USD 值
- 若省略 `amount_usd` 且未附带 `value`，该调用不受阈值/日限额检查
- 可通过 `GET /api/prices` 获取当前 ETH/SOL 价格用于计算

**便捷端点：**

```bash
# 授权 ERC-20 代币支出（始终需要 Passkey 审批）
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/approve-token" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xTokenContract...",
    "spender": "0xSpenderAddress...",
    "amount": "1000",
    "decimals": 6
  }'

# 撤销 ERC-20 代币授权（始终需要 Passkey 审批）
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/${WALLET_ID}/revoke-approval" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contract": "0xTokenContract...",
    "spender": "0xSpenderAddress..."
  }'
```

---

## 审批系统

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
- `status: "expired"`：审批超时（默认 30 分钟），需重新发起

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

## Web UI

TEENet Wallet 内置了一个完整的 Web 前端界面，开箱即用，无需单独部署。

**访问方式：** 启动服务后直接访问 `http://<HOST>:<PORT>`（默认 `http://localhost:8080`）。

**主要功能：**
- **账户管理**：Passkey 注册/登录、API Key 生成与管理
- **钱包管理**：创建/查看/重命名/删除钱包，查看余额
- **交易审批**：查看待审批列表、使用 Passkey 硬件认证审批或拒绝
- **合约白名单**：添加/编辑/删除白名单条目
- **审批策略**：设置 USD 阈值和日限额
- **审计日志**：查看操作历史记录

**安全头部：** Web UI 页面附带严格的安全头部配置：
- `Content-Security-Policy`：限制资源加载来源
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`（防止点击劫持）
- `Referrer-Policy: strict-origin-when-cross-origin`

---

## AI Agent 集成

TEENet Wallet 原生支持 AI Agent 集成，提供了 OpenClaw Skill 定义，使 AI 代理能够通过自然语言管理钱包和执行交易。

### OpenClaw Skill

Skill 定义位于 `skill/tee-wallet/` 目录，兼容 [OpenClaw](https://openclaw.io) AI 助手平台。

**配置 AI Agent 所需的环境变量：**

| 变量 | 说明 |
|------|------|
| `TEE_WALLET_API_URL` | 钱包服务地址（如 `https://wallet.example.com`） |
| `TEE_WALLET_API_KEY` | API Key（`ocw_` 前缀） |

**Agent 可执行的操作：**
- 创建钱包、查看钱包列表
- 查询原生代币和 ERC-20/SPL 代币余额
- 发送原生代币和代币转账
- 智能合约调用（EVM ABI 编码 / Solana 程序指令）
- 管理合约白名单（提交待审批请求）
- 设置审批策略（提交待审批请求）
- 查看审计日志

### Agent 集成最佳实践

1. **智能钱包选择**：不要让用户手动提供钱包 ID。先调用 `GET /api/wallets` 获取列表，根据链自动匹配或让用户从列表中选择。

2. **无需聊天确认**：不要在聊天中二次确认转账——后端审批策略是安全网。小额交易直接执行，大额交易自动触发 Passkey 硬件审批。

3. **预检余额**：发起大额 ETH 转账前建议先查询余额，确保 `balance >= amount + gas_buffer`（ETH 建议预留 0.0005 ETH 的 Gas 缓冲）。

4. **始终包含 `token` 字段**：发送代币时务必包含 `token` 对象，遗漏会发送原生代币。

5. **全局代币列表**：查询代币余额时，收集同一链上所有钱包白名单合约的并集，用这个全局列表查询每个钱包——白名单只限制发送，不限制持有。

6. **审批轮询**：收到 `pending_approval` 状态后，每 15 秒轮询一次 `GET /api/approvals/{id}`，向用户展示剩余等待时间。25 分钟无结果则停止轮询。

7. **动态链列表**：不要硬编码链名称，始终通过 `GET /api/chains` 获取可用链（包括用户添加的自定义链）。

8. **附带 explorer 链接**：交易成功后始终附带区块浏览器链接：
   - Ethereum 主网：`https://etherscan.io/tx/{hash}`
   - Sepolia：`https://sepolia.etherscan.io/tx/{hash}`
   - Base Sepolia：`https://sepolia.basescan.org/tx/{hash}`
   - Solana 主网：`https://solscan.io/tx/{hash}`
   - Solana Devnet：`https://solscan.io/tx/{hash}?cluster=devnet`

9. **申报 amount_usd**：调用 `/contract-call` 涉及价值转移时，始终包含 `amount_usd` 字段以触发阈值和日限额检查。

---

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
