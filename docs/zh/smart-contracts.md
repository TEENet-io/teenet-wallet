# 智能合约交互

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
    "label": "USDC Stablecoin"
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
    "label": "USDC v2",
    "symbol": "USDC",
    "decimals": 6
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
[上一页: 转账](transfers.md) | [下一页: 审批系统](approvals.md)
