# 概览

TEENet Wallet 是一款多链加密钱包，让你的 AI Agent 处理日常交易——余额查询、转账、合约调用——同时由你来审批重要操作。你可以设置转账限额、限制 Agent 可交互的合约，并通过一次 Passkey 点击确认高价值操作。你的规则在硬件保护的安全飞地中执行，而不仅仅是在应用代码中。

私钥不会存在于任何单一机器上。它们在 TEE 节点内部生成，通过门限密码学分片到多个独立节点，且永远不会被导出或重组。签名需要多个节点协作——任何运营商、云服务商或被攻破的服务器都无法单方面访问你的密钥。

---

## 我是用户

设置你的钱包、管理资产并审批交易。

- [快速上手](en/user-getting-started.md) — 创建账户和第一个钱包
- [支持的操作](en/user-commands.md) — 支持的操作和链
- [Web UI 与审批](en/user-approvals.md) — 使用 Passkey 审批交易
- [常见问题](en/user-faq.md) — 常见问题

## 我是开发者

基于钱包 API 构建应用、集成 Agent 平台或贡献代码。

- [快速开始](en/quick-start.md) — 快速上手
- [架构概览](en/architecture-overview.md) — 核心概念
- [添加链](en/howto-add-chain.md) — 操作指南
- [Agent 集成](en/agent-integration.md) — Agent 集成
- [贡献流程](en/contributing-process.md) — 参与贡献
