# AI Agent 集成

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
[上一页: 审批系统](approvals.md) | [下一页: API 参考](api-overview.md)
