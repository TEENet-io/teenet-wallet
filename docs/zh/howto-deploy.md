# 在 TEENet 上部署你的钱包应用

本页说明 TEENet Wallet 的部署边界，以及在 TEENet 上部署前需要准备的内容。

## 当前状态

TEENet Wallet 作为 **TEENet 平台**上的应用进行部署。本页只覆盖钱包应用自身的部署要求。

生产部署依赖真实的 **TEENet service**，用于：

- 密钥生成
- 门限签名
- Passkey 管理
- 审批时的身份验证

本地 mock 服务仅供开发使用。它使用确定性密钥，绝不能用于真实资金。

如果你尚未获得 TEENet 环境访问权限，请先阅读未来提供的 TEENet 平台接入 / 开通页面。该页面应说明如何申请访问权限并准备环境。本页假设你已经在 TEENet 环境中部署钱包应用。

## 本页覆盖的内容

- TEENet Wallet 在 TEENet 上运行所需的条件
- TEENet 部署与本地开发的区别
- 部署前仍需要阅读的本地文档

本页不覆盖：

- 如何获得 TEENet 平台访问权限
- 如何开通或初始化 TEENet 环境
- 钱包应用之外的 TEENet 平台运维流程

## 在 TEENet 上部署前的前提条件

在 TEENet 上部署之前，你需要：

- 一个可访问的 TEENet 环境，并能连通对应的 TEENet service
- 从本仓库构建好的钱包应用
- 与目标链匹配的 RPC 配置
- 用于审批链接和浏览器访问的公开 base URL
- 已完成 TEENet 平台接入 / 开通流程

本地构建方式和运行时配置请参阅[安装与配置](installation.md)和[配置参考](configuration.md)。

## 开发环境与生产环境

mock 服务只适用于：

- 本地开发
- 集成测试
- API 调试与实验

不要将 mock 服务用于：

- 主网钱包
- 真实资产
- 任何涉及真实价值的生产演示

## 部署前建议先阅读

建议先阅读这些钱包侧文档：

- [安装与配置](installation.md) -- 构建要求与本地开发流程
- [配置参考](configuration.md) -- 运行时环境变量
- [架构概览](architecture-overview.md) -- 钱包如何依赖 TEENet service

然后再阅读 TEENet 平台接入 / 开通页面，完成环境访问和平台侧准备工作。

## 下一步

- TEENet 平台接入 / 开通页面 -- 获取环境访问权限并完成平台侧设置
- [TEENet Platform 文档](https://teenet-io.github.io/#/) -- 了解平台背景
- [社区与支持](community.md) -- 查看项目链接和维护者信息
