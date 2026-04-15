# TEENet Wallet

一款让你的 AI Agent 安全使用的钱包——不会让你的资产面临风险。

你的 Agent 处理日常任务（余额查询、转账、活动记录），而你来设定规则：转账限额、合约白名单和审批要求。当操作超出你的规则时，你只需一次 Passkey 确认即可介入。

> **免责声明：** 本软件管理真实的加密货币资产。使用风险自负。作者不对任何资金损失负责。请务必在测试网上充分测试后再使用真实资产。

---

## 有什么不同

- **密钥永不重组** -- 私钥通过门限密码学分片存储在 TEE 节点中。没有任何一台机器拥有完整密钥。
- **双重认证** -- API Key 给 AI Agent 和自动化；Passkey（WebAuthn）给人类审批高价值操作。
- **消费控制** -- 以 USD 计价的阈值、日限额和合约白名单在签名前强制执行。
- **多链，一个 API** -- 以太坊、Solana 及所有 EVM 兼容链，通过一个 REST API 访问。

---

## 我是用户

通过 [OpenClaw](https://openclaw.ai) 使用 TEENet Wallet——无需编程。

- [快速上手](/zh/user-getting-started.md) -- 创建账户和第一个钱包
- [和 OpenClaw 对话](/zh/user-commands.md) -- 管理钱包、发送加密货币、与 DeFi 交互
- [审批与 Web 界面](/zh/user-approvals.md) -- 如何使用 Web 控制台和审批交易
- [安全与常见问题](/zh/user-faq.md) -- 关于安全、密钥和使用的常见问题

## 我是开发者

基于 TEENet Wallet 的 REST API 构建应用，或参与项目贡献。

- [快速开始](/zh/quick-start.md) -- 5 分钟从零开始运行
- [架构概览](/zh/architecture-overview.md) -- 系统工作原理
- [添加链](/zh/howto-add-chain.md) -- 操作指南
- [AI Agent 集成](/zh/agent-integration.md) -- Agent 平台最佳实践
- [贡献流程](/zh/contributing-process.md) -- 如何参与贡献

---

## 支持的签名方案

TEENet Wallet 通过 [TEENet 平台](https://teenet.io) 支持区块链系统使用的所有主要签名方案。标记 **✓** 的链已完成端到端测试。

| 方案 | 区块链 |
|------|--------|
| ECDSA secp256k1 | Ethereum **✓**、Optimism **✓**、Base **✓**、BNB Chain **✓**、Avalanche **✓**、Arbitrum、Polygon、Bitcoin 及所有 EVM 链 |
| Ed25519 | Solana **✓** |

---

## TEENet 平台

这个钱包是构建在 [TEENet](https://teenet.io) 之上的一个应用——TEENet 是一个提供硬件隔离运行时和托管密钥保管的平台，适用于任何需要保护密钥的应用。TEENet 目前处于开发者预览阶段。

[平台文档](https://teenet-io.github.io/#/) · [TEENet SDK](https://github.com/TEENet-io/teenet-sdk) · [GitHub](https://github.com/TEENet-io/teenet-wallet)

**[English Documentation →](/en/overview)**
