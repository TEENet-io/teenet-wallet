[wallet-url]: https://test.teenet.io/instance/f8e649535e1d2838ae2817992f946d6a

# 安全与常见问题

---

## 安全性

**私钥分片存储在 TEE 硬件节点。** 你的私钥在创建时被分割成多个碎片，分散存储在多个安全硬件节点中。签名时节点通过密码学协议协作完成，完整私钥从未被还原，也从未离开过安全硬件。

**OpenClaw 永远看不到你的私钥。** OpenClaw 只是发起签名请求，实际签名由 TEE 安全节点完成。即使 OpenClaw 被攻破，攻击者也无法获取你的私钥。

**大额交易需要你的指纹或面容。** 超过阈值的交易必须经过你的 Passkey 验证，任何人都无法伪造你的生物识别信息。

**消费限额由你控制。** 你设定 USD 阈值和日限额，OpenClaw 只能在这个范围内自主行动。

**随时可以撤销 OpenClaw 的访问权限。** 在 Web 界面的 Account 标签页删除 API 密钥，OpenClaw 的所有钱包操作权限瞬间归零。

---

## 常见问题

### 怎么安装钱包技能？

打开 OpenClaw 的对话，发送 **"安装这个 skill:"** 然后附上链接：

```
https://github.com/TEENet-io/teenet-wallet/blob/master/skill/tee-wallet/SKILL.md
```

OpenClaw 会引导你完成设置，你需要提供钱包地址和 API 密钥。详细步骤请参考"快速上手"中的第三步。

### OpenClaw 不需要我审批就能做什么？

在你设定的规则范围内，OpenClaw 可以自主完成：
- 发送低于 USD 阈值的转账
- 查询任意钱包的余额和交易状态
- 读取合约数据（查余额、查价格等只读操作）
- 查看钱包地址和公钥信息

阈值越高，OpenClaw 的自主权越大；阈值越低，你需要审批的频率越高。

### 什么操作一定需要我审批？

以下操作无论如何都需要你用 Passkey 确认：
- 超过 USD 阈值的任何转账
- 所有智能合约操作（交换、代币授权、DeFi 操作等）
- 添加新合约到白名单
- 修改或禁用审批策略
- 删除钱包
- 删除账户
- 生成或撤销 API 密钥

这些操作涉及资产安全或权限变更，系统设计上不允许任何人（包括 OpenClaw）绕过你的确认。

### 可以用多个 OpenClaw 机器人吗？

可以。为每个 OpenClaw 实例生成一个独立的 API 密钥即可。它们共享同样的审批策略和白名单。你可以在历史记录中区分不同实例的操作，也可以单独撤销某个实例的密钥。建议为每个实例使用不同的标签名（比如"openclaw-交易"和"openclaw-理财"），方便管理和追踪。

### OpenClaw 花超了怎么办？

多重机制保护你的资产：
- **USD 阈值** -- 超过阈值的交易自动暂停等待你审批，OpenClaw 无法绕过
- **日限额** -- 当天累计金额达到上限后，OpenClaw 无法再发起任何交易
- **白名单** -- OpenClaw 只能与你授权的合约交互，无法将资金发送到未知合约
- **撤销密钥** -- 发现异常时，立即在 Web 界面撤销 API 密钥，OpenClaw 的所有权限瞬间归零

### 怎么停止 OpenClaw 使用我的钱包？

登录 [TEENet Wallet][wallet-url]，切换到 **Account** 标签页，在 API 密钥列表中找到对应的密钥，点击 **Revoke**（撤销）。密钥立即失效，OpenClaw 将无法执行任何钱包操作。如果你不确定哪个密钥对应哪个 OpenClaw 实例，可以撤销所有密钥，然后为需要保留的实例重新生成。

### 什么是 Passkey？

Passkey（通行密钥）是一种取代传统密码的身份验证方式，利用你设备上的指纹传感器、面容识别或硬件安全密钥来验证身份。和密码不同，Passkey 不需要记忆、无法被钓鱼、无法被暴力破解。在 TEENet Wallet 中，Passkey 是你审批交易和确认敏感操作的唯一方式。

### 丢了设备怎么办？

恢复方式取决于你的 Passkey 同步设置：
- **Apple 设备** -- Passkey 通过 iCloud 钥匙串自动同步，在新设备上登录同一个 Apple ID 即可恢复
- **Android / Chrome** -- Passkey 通过 Google 密码管理器同步，登录同一个 Google 账户即可
- **硬件安全密钥** -- 请确保有备用密钥，唯一的安全密钥丢失将无法访问账户

建议在多个设备上设置 Passkey（例如手机和电脑），这样即使丢失一个设备也不会失去访问权限。

### 支持哪些区块链？

**正式网络：**

| 链 | 币种 | 类型 |
|----|------|------|
| Ethereum | ETH | EVM |
| Optimism | ETH | EVM 二层 |
| Solana | SOL | Solana |

**测试网络（免费练习用）：**

| 链 | 币种 | 类型 |
|----|------|------|
| Sepolia | ETH | EVM |
| Holesky | ETH | EVM |
| Base Sepolia | ETH | EVM 二层 |
| BSC Testnet | tBNB | EVM |
| Solana Devnet | SOL | Solana |

任何 EVM 兼容链（Polygon、Arbitrum、Avalanche 等）都可以在运行时添加。用 `/chains` 查看当前列表，或联系管理员添加。

---
[上一页: 审批与 Web 界面](/zh/user-approvals.md)
