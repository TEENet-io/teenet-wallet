# TEENet Wallet — 审批与限额设计

> 限额只针对转账，合约操作全部需要审批。

## 目录

- [设计原则](#设计原则)
- [审批决策逻辑](#审批决策逻辑)
- [转账限额](#转账限额)
  - [适用范围](#适用范围)
  - [定价来源](#定价来源)
  - [查不到价格的处理](#查不到价格的处理)
- [合约操作审批](#合约操作审批)
  - [为什么合约操作不走限额](#为什么合约操作不走限额)
  - [合约白名单](#合约白名单)
- [EVM 与 Solana 差异](#evm-与-solana-差异)
- [总结](#总结)

---

## 设计原则

| 原则 | 说明 |
|------|------|
| **转账走限额** | `/transfer` 端点的原生币和 ERC-20/SPL 转账按 USD 阈值自动判断 |
| **合约走审批** | `/contract-call`、`/approve-token`、`/revoke-approval` 通过 API Key 调用时全部需要 Passkey 审批 |
| **查不到价格走审批** | 转账时系统无法定价的 token → 进入审批流程 |
| **白名单只控准入** | 合约白名单只控制"能不能调"，具体操作由审批者在审批时判断 |

---

## 审批决策逻辑

```
Passkey 操作 → 直接执行（人已在场）

API Key 操作：
  ├─ 转账（/transfer）：
  │    ├─ 系统能算出 USD：
  │    │    ├─ 低于阈值 → 直接执行
  │    │    └─ 超过阈值 → 审批
  │    └─ 系统算不出 USD（未知 token）→ 审批
  │
  └─ 合约操作（/contract-call, /approve-token, /revoke-approval）：
       └─ 全部需要审批
```

---

## 转账限额

### 适用范围

限额**仅适用于 `/transfer` 端点**：

| 操作 | 端点 | API Key 行为 |
|------|------|-------------|
| ETH/SOL 原生币转账 | `/transfer` | 按 USD 限额自动判断 |
| USDC/USDT 等 ERC-20 转账 | `/transfer` | 按 USD 限额自动判断 |
| SPL Token 转账 | `/transfer` | 按 USD 限额自动判断 |
| 合约调用 | `/contract-call` | 需要审批 |
| Token 授权 | `/approve-token` | 需要审批 |
| 撤销授权 | `/revoke-approval` | 需要审批 |

### 限额机制

每个钱包可配置一个 `ApprovalPolicy`：

| 字段 | 作用 |
|------|------|
| `threshold_usd` | 单笔阈值 — 转账金额超过此值需要 Passkey 审批 |
| `daily_limit_usd` | 每日限额 — 当天累计超过直接拒绝（硬限制） |

### 定价来源

转账定价采用多级 fallback 机制：

| 优先级 | 定价来源 | 覆盖范围 |
|-------|---------|---------|
| 1 | 内置原生币价格（CoinGecko 实时） | ETH, SOL, BNB, POL, AVAX |
| 2 | 内置稳定币 | USDC, USDT, DAI, BUSD = $1 |
| 3 | CoinGecko Token Price API（按合约地址） | 17 条 EVM 链 + Solana 上的主流 token |
| 4 | Jupiter Price API（Solana fallback） | 几乎所有有 DEX 流动性的 SPL token |
| — | 以上均无法定价 | → **进入审批流程**（fail-closed） |

**支持 token 定价的 EVM 链：** Ethereum, Optimism, Arbitrum, Base, Polygon, BSC, Avalanche, Fantom, Linea, zkSync, Scroll, Mantle, Celo, Gnosis, Cronos, Moonbeam, Blast

**Solana：** CoinGecko 查不到时自动 fallback 到 Jupiter Price API，覆盖绝大部分有流动性的 SPL token

### 查询每日花费

```
GET /api/wallets/:id/daily-spent
```

返回当天（UTC）的累计花费和限额信息：

```json
{
  "daily_spent_usd": "235.50",
  "daily_limit_usd": "1000",
  "remaining_usd": "764.50",
  "reset_at": "2026-03-26T00:00:00Z"
}
```

- `daily_spent_usd` — 今天已花费的 USD 总额
- `daily_limit_usd` — 每日限额（未设置则为空）
- `remaining_usd` — 今天还能花多少（未设限额则为空）
- `reset_at` — 下次重置时间（UTC 零点）

无限额策略时返回：

```json
{
  "daily_spent_usd": "0",
  "daily_limit_usd": "",
  "remaining_usd": "",
  "reset_at": ""
}
```

### 查不到价格的处理

如果系统通过所有定价来源都无法查询到 token 的 USD 价格，该笔转账**自动进入审批流程**，由 Passkey 持有者人工确认。

这确保了：
- 主流 token（ETH、SOL、BNB、POL、AVAX、USDC、USDT 等）能自动按限额执行
- EVM 链上的 ERC-20 token（CoinGecko 有收录的）能自动定价
- Solana SPL token（有 DEX 流动性的）能通过 Jupiter 自动定价
- 未知/长尾/无流动性 token 不会绕过风控

---

## 合约操作审批

### 为什么合约操作不走限额

合约调用的金额无法可靠自动定价：

| 难点 | 说明 |
|------|------|
| **ABI 只描述类型，不描述语义** | 系统知道参数是 `uint256`，但不知道它是"金额"还是"时间戳"还是"费率" |
| **参数位置不固定** | 不同方法金额在不同位置，同名方法实现也不同 |
| **多 token 交互** | swap 涉及多种 token，decimals 和价格各不同 |
| **嵌套结构体** | DeFi 协议常用 tuple 嵌套，金额藏在深层 |
| **非标准合约** | 任何人可部署任意接口，没有通用规则 |

**因此，所有合约操作通过 API Key 调用时一律需要 Passkey 审批。** Passkey 操作（人已在场）则直接执行。

### 合约白名单

合约白名单的作用是**地址准入** — 只控制"能不能调这个合约"。

```json
POST /api/wallets/:id/contracts
{
  "contract_address": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
  "label": "USDC on Ethereum",
  "symbol": "USDC",
  "decimals": 6
}
```

| 字段 | 用途 |
|------|------|
| `contract_address` | 白名单地址，未白名单的合约返回 403 |
| `label` | 合约用途说明，帮助审批者识别 |
| `symbol` | token 符号，用于转账定价 |
| `decimals` | token 精度，用于转账金额换算 |

不再需要 `allowed_methods` 和 `auto_approve`：
- **`allowed_methods` 移除** — 每次合约调用都经过人工审批，审批者能看到调用的方法和参数，自行判断
- **`auto_approve` 移除** — 合约操作全部审批，不存在"自动通过"

Agent 提交白名单请求时，应附上合约地址、用途等上下文，帮助 Passkey 持有者做出审批决策。

---

## EVM 与 Solana 差异

### 模型区别

| | EVM | Solana |
|--|-----|--------|
| 转账限额 | ✅ ETH + ERC-20 | ✅ SOL + SPL Token |
| 合约操作 | 全部需要审批 | 全部需要审批 |
| DeFi 模型 | 先 approve 再 swap（两步） | 直接调用（一步） |

### Solana 转账定价

SPL Token 转账通过多级 fallback 定价：
1. **GetUSDPrice(symbol)** — 覆盖 SOL 原生币和稳定币
2. **CoinGecko Token Price API** — 按 mint address 查询
3. **Jupiter Price API** — CoinGecko 查不到时自动 fallback，覆盖几乎所有有 DEX 流动性的 SPL token

### Solana 合约操作

合约操作（程序调用）无法自动定价：
- **指令数据是原始 bytes** — 不同程序格式各异，无法通用解析出金额
- **无 approve 模型** — 没有"授权即限额"关卡

因此 Solana 合约操作与 EVM 一样，API Key 调用全部需要审批。

### 后续优化方向

1. **SPL Token 指令解析** — SPL Token 程序的指令格式固定，可单独解析 Transfer（03）指令自动提取金额，用于合约调用场景的定价

---

## 总结

| 操作类型 | 端点 | API Key 行为 | Passkey 行为 |
|---------|------|-------------|-------------|
| 原生币转账 | `/transfer` | 按 USD 限额自动判断 | 直接执行 |
| ERC-20/SPL 转账 | `/transfer` | 按 USD 限额自动判断（查不到价格走审批） | 直接执行 |
| 合约调用 | `/contract-call` | **需要审批** | 直接执行 |
| Token 授权 | `/approve-token` | **需要审批** | 直接执行 |
| 撤销授权 | `/revoke-approval` | **需要审批** | 直接执行 |
| 合约白名单 | `/contracts` | 需要审批（附合约信息） | 直接生效 |

**一句话：转账走限额，合约走审批，查不到价格也走审批。白名单只控准入，不控方法。**
