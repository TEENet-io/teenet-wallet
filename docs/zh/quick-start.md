# 快速开始

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
[上一页: 简介](introduction.md) | [下一页: 配置参考](configuration.md)
