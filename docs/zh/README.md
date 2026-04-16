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

## 使用 TEENet Wallet

通过 [OpenClaw](https://openclaw.ai) 使用 TEENet Wallet——无需编程。

- [快速上手](/zh/user-getting-started.md) -- 创建账户和第一个钱包
- [和 OpenClaw 对话](/zh/user-commands.md) -- 管理钱包、发送加密货币、与 DeFi 交互
- [审批与 Web 界面](/zh/user-approvals.md) -- 如何使用 Web 控制台和审批交易
- [安全与常见问题](/zh/user-faq.md) -- 关于安全、密钥和使用的常见问题

## 构建与集成

基于 TEENet Wallet 的 REST API 构建应用，将其集成到 Agent 平台中，或为部署到 TEENet 做准备。

- **快速开始**
  - [快速开始](/zh/quick-start.md) -- 5 分钟从零开始运行
  - [安装与配置](/zh/installation.md) -- 构建方式、环境变量与 Docker
  - [故障排查](/zh/troubleshooting.md) -- 常见安装与运行问题
- **集成**
  - [AI Agent 集成](/zh/agent-integration.md) -- Agent 平台最佳实践
  - [TEENet SDK 使用](/zh/sdk-usage.md) -- 钱包如何使用 SDK 和 mock server
- **核心概念**
  - [架构概览](/zh/architecture-overview.md) -- 系统工作原理
  - [签名与 TEE 信任模型](/zh/signing-tee.md) -- 门限签名与托管模型
- **操作指南**
  - [添加链](/zh/howto-add-chain.md) -- 安全扩展链支持
  - [添加插件工具](/zh/howto-add-plugin-tool.md) -- 扩展 OpenClaw 插件能力
  - [在 TEENet 上部署你的钱包应用](/zh/howto-deploy.md) -- 部署边界与前提条件
- **API 参考**
  - [OpenAPI 规范](/api/openapi.yaml) -- 机器可读的 API schema
  - [认证体系](/zh/authentication.md) -- API 参考入口
  - [钱包管理](/zh/wallets.md) -- 钱包生命周期接口
  - [转账](/zh/transfers.md) -- 原生资产转账接口
  - [地址簿](/zh/addressbook.md) -- 收款人管理
  - [智能合约交互](/zh/smart-contracts.md) -- 合约调用与代币交互接口
  - [审批系统](/zh/approvals.md) -- 审批请求与确认流程
- **参考**
  - [配置参考](/zh/configuration.md) -- 环境变量与运行时行为
  - [错误码与状态码](/zh/error-codes.md) -- 错误模型与 HTTP 语义
  - [审计日志](/zh/audit-log.md) -- 审计轨迹接口与用法
  - [chains.json Schema](/zh/chains-schema.md) -- 链定义格式
  - [数据模型](/zh/data-model.md) -- 数据库实体与关系

## 贡献

- [贡献流程](/zh/contributing-process.md) -- 如何参与贡献
- [编码规范与 CI](/zh/coding-standards.md) -- 开发要求与检查项

## 支持

- [社区与支持](/zh/community.md) -- 项目链接、安全报告和维护者信息

---

## 支持的签名方案

TEENet Wallet 通过 [TEENet 平台](https://teenet.io) 支持区块链系统使用的所有主要签名方案。标记 **✓** 的链已完成端到端测试。

| 方案 | 区块链 |
|------|--------|
| ECDSA secp256k1 | Ethereum **✓**、Optimism **✓**、Base **✓**、BNB Chain **✓**、Avalanche **✓**、Arbitrum、Polygon、Bitcoin 及所有 EVM 链 |
| EdDSA / ed25519 | Solana **✓** |

---

## TEENet 平台

这个钱包是构建在 [TEENet](https://teenet.io) 之上的一个应用——TEENet 是一个提供硬件隔离运行时和托管密钥保管的平台，适用于任何需要保护密钥的应用。TEENet 目前处于开发者预览阶段。

[平台文档](https://teenet-io.github.io/#/) · [TEENet SDK](https://github.com/TEENet-io/teenet-sdk) · [GitHub](https://github.com/TEENet-io/teenet-wallet)

**[English Documentation →](/en/overview)**
