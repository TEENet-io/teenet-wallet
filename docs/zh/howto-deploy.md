# 申请 TEENet 部署

> **Alpha 说明：** 公开的 TEENet Wallet 目前处于 alpha，运行时带 `ALPHA_MODE=true`，启动阶段会把主网从链注册表里过滤掉，只剩 8 条测试网（Sepolia、Optimism Sepolia、Arbitrum Sepolia、Base Sepolia、Polygon Amoy、BSC Testnet、Avalanche Fuji、Solana Devnet）。主网链本身都写在 `chains.json` 里，功能完整，只是 alpha 开关把它们挡掉了。**通过本页申请的托管部署不受 alpha 链集合限制**——你自己的生产实例不开 `ALPHA_MODE`，可以用任意受支持的链，包括主网。

如果你想在自己的基础设施上跑一个连接 mock 服务的钱包实例，参见 [快速开始](quick-start.md) 和 [安装与配置](installation.md)。这条路径适合本地开发、集成测试、内部评估——但它使用 mock 服务生成的确定性密钥，**不适用于真实资金**。

本页讲的是另一件事：**在 TEENet 平台上部署生产实例**，由真实 TEE 节点持有密钥分片并在多个独立 enclave 之间完成门限签名。

> 本页介绍的托管部署流程**同时适用于本钱包**以及**任何基于 [TEENet SDK](https://github.com/TEENet-io/teenet-sdk) 构建的应用**（Go 或 TypeScript）。如果你写了自己的 Agent、交易系统、托管服务或其它 SDK 集成应用，同样可以通过本页底部的链接发起托管部署申请。

## 托管交付，不自助

TEENet 平台部署由 TEENet 团队端到端交付。我们负责：

- 开通 TEE 节点与安全网络
- 把你的应用注册到 TEENet 服务
- 引导密钥材料与 Passkey 基础设施
- 配置生产 base URL、审批回调、监控
- 平台侧日常运维与升级

你描述需求；我们交付一个带管理员访问的运行中实例。

## 你会拿到什么

- 运行在 TEENet 平台上的应用实例（本钱包，或你的 SDK 集成应用）
- 跨 TEE 节点的真实门限签名——密钥从不离开 enclave 硬件
- 平台维护由 TEENet 团队负责

对于**钱包部署**，还包括：

- 钱包的公开 URL，用户在上面自助注册（邮箱 → 6 位验证码 → Passkey），并在自己的设置页生成 API key
- 预配置的链集合，可按需定制

## 你需要提供什么

- 申请方的组织 / 项目名
- 一个主要联系人
- 部署的内容（本钱包 / 自定义 SDK 应用）以及使用场景
- 预期用户量或负载情况
- 所需链（仅钱包部署；默认集合不够时填写）
- 合规或数据驻留要求
- 目标时间线

## 如何申请

在 GitHub 上提交一个部署申请 issue：

**[→ 提交部署申请](https://github.com/TEENet-io/teenet-wallet/issues/new?template=deployment-request.yml)**

表单会收集我们评估所需的所有信息——无论是本钱包还是自定义 SDK 应用。我们会在该 issue 上回复，或通过你提供的联系方式跟进。

## 交付之后

实例上线后，你会收到：

- 应用的公开 URL
- 供 Agent 或应用使用的 SDK / 集成配置

钱包部署时，首个用户和其他用户一样通过公开 URL 自助注册（邮箱 → 验证码 → Passkey），没有单独的管理员账号。之后就是常规使用流程——参见 [快速上手](user-getting-started.md)。

## 开发环境与生产环境

**Mock 服务**适用于：

- 本地开发
- 集成测试
- API 调试与实验

**不要**在以下场景使用 mock 服务：

- 主网钱包
- 真实资产
- 任何涉及真实价值的生产流程

## 相关链接

- [TEENet Platform](https://teenet.io) —— 平台概览
- [TEENet SDK](https://github.com/TEENet-io/teenet-sdk) —— 构建自己的 SDK 应用
- [架构概览](architecture-overview.md) —— 钱包如何依赖 TEE 节点
- [社区与支持](community.md) —— 项目通用沟通渠道
