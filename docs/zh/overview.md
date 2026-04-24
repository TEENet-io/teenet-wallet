# 概览

> **Alpha 版本。** 公开的 TEENet Wallet 目前处于 alpha，运行时带 `ALPHA_MODE=true`，**只开放 8 条测试网**（Sepolia、Optimism Sepolia、Arbitrum Sepolia、Base Sepolia、Polygon Amoy、BSC Testnet、Avalanche Fuji、Solana Devnet）；注册名额为**前 500 人**，先到先得。主网链（Ethereum、Optimism、Arbitrum、Base、Polygon、BNB Chain、Avalanche、Solana）在 `chains.json` 里都已实现，alpha 验证完成后取消 `ALPHA_MODE` 即可重新开放。

TEENet Wallet 是一款多链加密钱包，让你的 AI Agent 处理日常交易——余额查询、转账、合约调用——同时由你来审批重要操作。你可以设置转账限额、限制 Agent 可交互的合约，并通过一次 Passkey 点击确认高价值操作。你的规则在硬件保护的安全飞地中执行，而不仅仅是在应用代码中。

私钥不会存在于任何单一机器上。它们在 TEE 节点内部生成，通过门限密码学分片到多个独立节点，且永远不会被导出或重组。签名需要多个节点协作——任何运营商、云服务商或被攻破的服务器都无法单方面访问你的密钥。
