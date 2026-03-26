# TEENet Wallet 智能合约调用设计

## 核心结论：无需 ABI 文件即可调用任意合约

传统 DApp 调用合约需要导入完整的 ABI JSON 文件（通常几百行），我们的方案只需一行函数签名即可调用任意已白名单合约。

---

## 一、EVM 合约调用

### 调用方式

```json
POST /api/wallets/:id/contract-call

{
  "contract": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
  "func_sig": "transfer(address,uint256)",
  "args": ["0x接收地址", "1000000"]
}
```

系统内置完整 ABI 编码器（`chain/abi.go`），自动完成：
1. 从函数签名计算 4 字节 selector（Keccak256）
2. 按 ABI 规范将参数编码为 EVM calldata

### 为什么不需要 ABI 文件

Solidity 的所有自定义类型在编译后都会被展开为基础类型：

| Solidity 源码 | ABI 层面实际类型 |
|:-------------|:---------------|
| `struct Order { address maker; uint256 amount; }` | `(address,uint256)` — tuple |
| `enum Status { Pending, Active, Cancelled }` | `uint8` |
| `type Price is uint256` | `uint256` |
| `MyLib.SomeStruct` | 字段展开为 tuple |

ABI 文件的作用只是告诉调用方"函数名是什么、参数类型是什么"，而我们的 API 要求调用方直接传入函数签名（如 `transfer(address,uint256)`），本身就已经包含了编码所需的全部信息。

### ABI 类型覆盖（100% 实用覆盖）

| ABI 基础类型 | 支持 | 典型场景 |
|:------------|:----:|:--------|
| `address` | ✅ | 钱包地址、合约地址 |
| `bool` | ✅ | 开关标志 |
| `uintN` (8~256) | ✅ | 金额、时间戳、ID |
| `intN` (8~256) | ✅ | 价格差值、偏移量 |
| `bytesN` (1~32) | ✅ | 哈希值、Merkle proof |
| `bytes` (动态) | ✅ | 任意二进制数据 |
| `string` | ✅ | 文本参数 |
| `T[]` 动态数组 | ✅ | 批量地址列表、批量金额 |
| `T[N]` 定长数组 | ✅ | 固定路径参数 |
| `(T1,T2,...)` tuple | ✅ | 结构体参数，支持任意层级嵌套 |
| `fixed` / `ufixed` | — | Solidity 编译器自身未完整实现，无实际合约使用 |

**所有实际存在的 Solidity 合约函数均可调用。** 包括复杂 DeFi 协议如 Uniswap V3 的嵌套 tuple 参数：

```json
{
  "func_sig": "exactInputSingle((address,address,uint24,address,uint256,uint256,uint160))",
  "args": [["0xTokenIn", "0xTokenOut", 3000, "0xRecipient", "1000000", "0", "0"]]
}
```

### 与传统方式对比

| | 传统方式（ethers.js / web3.js） | TEENet Wallet |
|:--|:------|:------------|
| 调用前准备 | 导入完整 ABI JSON 文件（几十到几百行） | 无需任何文件 |
| 接入新合约 | 获取并存储 ABI 文件 | 白名单地址即可，零额外配置 |
| 调用参数 | ABI 对象 + 函数名 + 参数 | 一行函数签名 + 参数 |
| 合约升级 | 需更新 ABI 文件 | 修改签名字符串即可 |
| AI Agent 友好度 | 低（需管理 JSON 文件） | 高（纯文本签名，LLM 可直接生成） |

---

## 二、Solana 程序调用

### Solana 为什么不需要 ABI

Solana 的架构与 EVM 完全不同，**链级别就不存在 ABI 这个概念**：

| | EVM (Ethereum) | Solana |
|:--|:--------------|:-------|
| 调用方式 | 函数签名 → 4字节 selector + ABI 编码参数 | 程序 ID + 账户列表 + 原始字节 |
| 接口描述标准 | ABI（统一规范） | 无统一标准 |
| 参数编码 | 严格的 32 字节对齐编码规范 | 程序自行定义，原始 bytes |
| 路由机制 | selector 决定调哪个函数 | 程序自己解析 data |

Solana 程序调用的本质就是三样东西：

```json
POST /api/wallets/:id/contract-call

{
  "contract": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
  "accounts": [
    {"pubkey": "源账户地址", "is_signer": true, "is_writable": true},
    {"pubkey": "目标账户地址", "is_signer": false, "is_writable": true},
    {"pubkey": "Owner地址", "is_signer": true, "is_writable": false}
  ],
  "data": "03a086010000000000"
}
```

- **contract** — 调用哪个程序
- **accounts** — 涉及哪些账户及其读写/签名权限
- **data** — 原始指令字节（由调用方按程序约定组装）

不是我们省略了 ABI，而是 **Solana 本身就没有 ABI 这层抽象**。

---

## 三、安全保障

### 转账：限额控制

`/transfer` 端点的转账按 USD 限额自动判断：

| 场景 | API Key 行为 |
|------|-------------|
| 转 0.01 ETH（阈值 $100，ETH=$3500 → $35） | 自动执行 |
| 转 1 ETH（阈值 $100 → $3500） | 需要 Passkey 审批 |
| 转 100 USDC（阈值 $500 → $100） | 自动执行 |
| 转 10 UNI（CoinGecko 查到 $10 → $100，阈值 $500） | 自动执行 |
| 转 SPL token（Jupiter 查到价格，低于阈值） | 自动执行 |
| 转未知 token（所有来源查不到价格） | 需要 Passkey 审批 |

定价来源（按优先级）：内置原生币（ETH/SOL/BNB/POL/AVAX）→ 稳定币（$1）→ CoinGecko Token Price API（17 条 EVM 链 + Solana）→ Jupiter Price API（Solana SPL fallback）→ 审批

### 合约操作：全部审批

所有合约操作通过 API Key 调用时需要 Passkey 审批：

| 层级 | 机制 | 说明 |
|:-----|:-----|:-----|
| **第一层：合约白名单** | 合约地址须经 Passkey 批准后加入白名单 | 只控准入，未白名单返回 403 |
| **第二层：操作审批** | `/contract-call`、`/approve-token`、`/revoke-approval` 全部需要 Passkey 审批 | 审批者看到方法和参数，自行判断 |

白名单不控制方法（无 `allowed_methods`），也不存在自动通过（无 `auto_approve`）。每次合约操作都经过人工审批，审批者能看到完整的调用上下文。

### 为什么合约操作不走限额

合约调用的金额无法可靠自动定价（ABI 只描述数据类型，不描述数据语义），详见 [审批与限额设计](approval-threshold-design.md)。

补充安全机制：
- **每日限额** — 可配置 USD 计价的每日支出上限（针对转账）
- **幂等性保护** — 通过 `Idempotency-Key` 防止重复交易

---

## 四、总结

| 维度 | 结论 |
|:-----|:-----|
| **EVM 合约** | 内置完整 ABI 编码器，100% 覆盖实用类型，函数签名即调用 |
| **Solana 程序** | 链本身无 ABI 概念，直接传原始指令数据 |
| **转账安全** | 多级定价（CoinGecko + Jupiter），查不到价格走审批 |
| **合约安全** | 白名单控准入 + 每次操作 Passkey 审批 |
| **AI Agent 适配** | 纯文本接口，无需管理 ABI 文件，LLM 可直接生成调用参数 |
