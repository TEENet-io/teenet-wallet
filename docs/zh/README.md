# TEENet Wallet

> 私钥永远不离开 TEE 硬件的多链加密钱包

TEENet Wallet 使用门限密码学将每一把私钥分片存储在 **可信执行环境（TEE）** 节点集群中。签名交易时，法定数量的节点（如 5 选 3）协作生成有效签名 —— 完整私钥从不在任何单台机器上还原。

## 为什么选择 TEENet Wallet？

| 传统钱包 | TEENet Wallet |
|---|---|
| 私钥存在单一位置 | 密钥分片分布在 TEE 节点 |
| 单点攻破即失控 | M-of-N 门限 —— 单节点无法签名 |
| 手动审批或无审批 | 可配置 USD 阈值 + Passkey 硬件审批 |
| 一次只支持一条链 | 以太坊、Solana 及所有 EVM 链一个 API 搞定 |

## 适用场景

- **AI Agent** —— 通过 API Key 实现程序化托管，可配置安全护栏
- **DeFi 自动化** —— 阈值以下的交易无需人工干预即可执行
- **机构团队** —— 多层审批策略 + USD 计价日限额

## 快速链接

- [快速开始](zh/quick-start.md) —— 5 分钟创建钱包并发送第一笔交易
- [认证体系](zh/authentication.md) —— API Key 给 Agent，Passkey 给人类
- [审批系统](zh/approvals.md) —— USD 阈值、日限额、高危方法管控
- [智能合约交互](zh/smart-contracts.md) —— 合约白名单、ABI 编码、Solana 程序
- [Agent 集成指南](zh/agent-integration.md) —— Agent 集成最佳实践
- [API 概览](zh/api-overview.md) —— 完整接口参考
- [架构与安全](zh/whitepaper.md) —— 技术深度解析

---

**[English Documentation →](en/introduction.md)**
