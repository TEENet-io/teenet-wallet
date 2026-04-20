# AI Agent 集成

TEENet Wallet 原生支持 AI Agent 集成，提供两种接入方式：

1. **Skill 方式（REST）** -- `skill/teenet-wallet/SKILL.md` 用 curl 示例描述所有操作，Agent 读取 Skill 后直接发 HTTP 请求，任何支持 Skill 的 Agent 平台都能用。
2. **OpenClaw 插件（原生工具）** -- `plugin/` 目录提供用 TypeScript 编写的一等 [OpenClaw](https://openclaw.io) 插件，每个钱包操作都是强类型工具，并通过 SSE 事件流让 Agent 在用户完成 Passkey 审批后自动继续执行。

### OpenClaw Skill

Skill 定义位于 `skill/teenet-wallet/` 目录，兼容 [OpenClaw](https://openclaw.io) AI 助手平台。

**配置 AI Agent 所需的环境变量：**

| 变量 | 说明 |
|------|------|
| `TEENET_WALLET_API_URL` | 钱包服务地址（如 `https://wallet.teenet.app`） |
| `TEENET_WALLET_API_KEY` | API Key（`ocw_` 前缀） |

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

6. **审批轮询**：收到 `pending_approval` 状态后，每 15 秒轮询一次 `GET /api/approvals/{id}`，向用户展示剩余等待时间。审批默认 24 小时过期（可通过 `APPROVAL_EXPIRY_MINUTES` 配置）。

7. **动态链列表**：不要硬编码链名称，始终通过 `GET /api/chains` 获取可用链（包括用户添加的自定义链）。

8. **附带 explorer 链接**：交易成功后始终附带区块浏览器链接：
   - Ethereum 主网：`https://etherscan.io/tx/{hash}`
   - Sepolia：`https://sepolia.etherscan.io/tx/{hash}`
   - Base Sepolia：`https://sepolia.basescan.org/tx/{hash}`
   - Solana 主网：`https://solscan.io/tx/{hash}`
   - Solana Devnet：`https://solscan.io/tx/{hash}?cluster=devnet`


---

## OpenClaw 插件（`plugin/`）

如果你用的是 [OpenClaw](https://openclaw.io) >= `2026.3.24-beta.2`，可以直接使用 `plugin/` 目录中的原生 TypeScript 插件。相比 Skill 方式（Agent 自己发 HTTP 请求），插件把每个钱包操作注册为强类型工具，并通过 SSE 事件流让 Agent 无需轮询就能感知 Passkey 审批结果。

### 安装

```bash
openclaw plugins install "/path/to/teenet-wallet/plugin"
openclaw config set plugins.entries.teenet-wallet.config.apiUrl "https://your-wallet-instance/"
openclaw config set plugins.entries.teenet-wallet.config.apiKey "ocw_your_api_key"
openclaw config set plugins.entries.teenet-wallet.enabled true
openclaw gateway restart
openclaw plugins inspect teenet-wallet   # 期望 Status: loaded
```

配置参数（来自 `openclaw.plugin.json`）：

| 参数 | 必填 | 说明 |
|------|------|------|
| `apiUrl` | 是 | 钱包后端地址（如 `https://wallet.teenet.app`） |
| `apiKey` | 是 | 带 `ocw_` 前缀的 API Key |

> **注意 `tools.profile`**。插件要求 `full` profile（默认值）。如果被设为 `coding`、`messaging` 或 `minimal`，工具会被静默屏蔽且不会报错。可通过 `openclaw config get tools.profile` 检查，必要时 `openclaw config unset tools.profile` 清除。

### 可用工具

所有工具名都以 `teenet_wallet_` 为前缀：

| 分类 | 工具 |
|------|------|
| 钱包 | `create`、`list`、`get`、`rename`、`balance` |
| 转账 | `transfer`、`wrap_sol`、`unwrap_sol` |
| 合约 | `list_contracts`、`add_contract`、`update_contract`、`contract_call`、`approve_token`、`revoke_approval` |
| 策略 | `get_policy`、`set_policy`、`daily_spent` |
| 通讯录 | `list_contacts`、`add_contact`、`update_contact` |
| 审批 | `pending_approvals`、`check_approval` |
| 工具 | `list_chains`、`health`、`faucet`、`audit_logs`、`prices`、`get_pubkey` |

### 基于 SSE 的审批流程

```
用户（聊天） → Agent → 插件工具 → 钱包后端
                ↑                      |
          subagent.run()          pending_approval
          (deliver=true)               ↓
                ↑                 SSE 事件流
                └── ApprovalWatcher ←──┘
```

1. Agent 调用工具（如 `teenet_wallet_transfer`）。
2. 如果后端返回 `pending_approval`，Agent 向用户发送审批链接。
3. 用户用 Passkey（指纹 / 安全密钥）确认。
4. `ApprovalWatcher` 收到 SSE 事件，在原对话中触发 `subagent.run()` -- 不需要轮询，也不需要用户来回复 "我批了"。
5. Agent 返回交易哈希和区块浏览器链接。

### 安全说明

- **API Key 永不暴露给 LLM** -- 存储在插件配置里，仅由 HTTP 客户端注入。
- **SSE 事件按用户隔离** -- 每个用户只能收到自己的审批事件。
- **所有写操作都在后端检查审批策略** -- 插件无法绕过 USD 阈值和日限额。
- **自定义链 RPC URL 有 SSRF 防护** -- 内网 IP 和云元数据地址在后端被拦截。
