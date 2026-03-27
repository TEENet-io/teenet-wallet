# TEENet Wallet

> 私钥永远不离开 TEE 硬件的多链加密钱包

TEENet Wallet 使用门限密码学将每一把私钥分片存储在 **可信执行环境（TEE）** 节点集群中。签名交易时，法定数量的节点（如 5 选 3）协作生成有效签名 -- 完整私钥从不在任何单台机器上还原。

## 为什么选择 TEENet Wallet？

| 传统钱包 | TEENet Wallet |
|---|---|
| 私钥存在单一位置 | 密钥分片分布在 TEE 节点 |
| 单点攻破即失控 | M-of-N 门限 -- 单节点无法签名 |
| 手动审批或无审批 | 可配置 USD 阈值 + Passkey 硬件审批 |
| 一次只支持一条链 | 以太坊、Solana 及所有 EVM 链一个 API 搞定 |

---

## 我是用户

通过 OpenClaw 使用 TEENet Wallet，无需编程。

- [快速上手](/zh/user-getting-started.md) -- 创建账户、连接 OpenClaw、设置第一个钱包
- [和 OpenClaw 对话](/zh/user-commands.md) -- 管理钱包和转账
- [审批与 Web 界面](/zh/user-approvals.md) -- 审批交易和使用控制面板
- [安全与常见问题](/zh/user-faq.md) -- 私钥如何被保护 + 常见问题

## 我要接入

通过 REST API 集成 TEENet Wallet。

- [快速开始](/zh/quick-start.md) -- 编译部署，发起第一个 API 调用
- [认证体系](/zh/authentication.md) -- API Key 给 Agent，Passkey 给人类
- [钱包管理](/zh/wallets.md) -- 通过 API 创建、列出和管理钱包
- [转账](/zh/transfers.md) -- 发送原生代币、ERC-20 和 SPL 代币
- [智能合约交互](/zh/smart-contracts.md) -- 合约白名单、ABI 编码、Solana 程序
- [审批系统](/zh/approvals.md) -- USD 阈值、日限额、合约调用审批
- [AI Agent 集成](/zh/agent-integration.md) -- Agent 集成最佳实践
- [API 概览](/zh/api-overview.md) -- 完整接口参考
- [配置参考](/zh/configuration.md) -- 环境变量和部署选项
- [架构与安全](/zh/whitepaper.md) -- 技术深度解析

---

**[English Documentation →](/en/introduction.md)**
