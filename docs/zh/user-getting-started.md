[wallet-url]: https://test.teenet.io/instance/wallet/

# 快速上手

## 第一步：创建账户

1. 在浏览器中打开 [TEENet Wallet 注册页面][wallet-url]。推荐使用 Google Chrome。

> **注意：** TEENet Wallet 当前仍处于测试阶段，默认最多支持 500 个注册用户，按先到先得方式开放。

2. 在必填字段中输入你的姓名或邮箱。

<div align="center"><img src="picture/register.png" alt="注册页面" width="360" /></div>

3. 点击 **使用 Passkey 注册**。浏览器会提示你设置 **Passkey**：
   - 在带生物识别的手机或笔记本上：使用指纹或面容识别。
   - 在桌面电脑上：使用硬件安全密钥（USB 设备）或用手机扫码。

<div align="center"><img src="picture/register2.png" alt="创建 Passkey 提示" width="360" /></div>

4. 完成后，账户即创建成功并自动登录，无需记住密码。

> **什么是 Passkey？** Passkey 用指纹、面容识别或安全密钥替代密码。你的生物识别数据不会离开设备，Passkey 也不容易被钓鱼、猜测或因数据泄露而失窃。

## 第二步：生成 API 密钥

API 密钥允许你的 AI Agent 访问钱包。没有它，任何 Agent 都无法和你的钱包交互。

1. 点击右上角的 **设置** 图标（齿轮）。

2. 在 API Keys 区域输入一个标签名（例如 `my-agent`），然后点击 **Generate API Key**。

3. 使用 Passkey 完成验证。

4. 立即复制密钥（以 **ocw_** 开头）。该密钥只会显示一次。

<div align="center"><img src="picture/generate_api.png" alt="API 密钥已生成" width="360" /></div>

## 第三步：连接你的 Agent

把第二步生成的 **API 密钥** 和账户页中显示的 **钱包 API URL** 提供给你的 Agent。具体方式取决于你使用的 Agent 平台。

**OpenClaw 示例：**

1. 打开 OpenClaw 对话。

2. 复制并发送下面这条消息：

   ```
   Install this skill: https://github.com/TEENet-io/teenet-wallet/blob/master/skill/tee-wallet/SKILL.md
   ```

3. 当 OpenClaw 提示时，输入：
   - **TEE_WALLET_API_URL**：账户页中显示的钱包 API URL
   - **TEE_WALLET_API_KEY**：第二步复制的 API 密钥

<div align="center"><img src="picture/tg.png" alt="通过 OpenClaw 连接钱包" width="360" /></div>

## 第四步：测试钱包

让 Agent 做一次快速测试。在 OpenClaw 中可以直接输入 `/test`。如果你还没有测试网钱包，Agent 会先帮你创建一个，然后引导你完成：

1. **检查余额**：确认钱包已经可用
2. **领取测试代币**：从 faucet 获取免费测试资产
3. **给自己转账**：验证 TEE 分布式签名工作正常
4. **设置 1 美元审批阈值**：这一步需要你的 Passkey
5. **发送一笔小额转账**：低于阈值，自动执行
6. **发送一笔较大转账**：会被挂起，等待你用 Passkey 审批
7. **把一个代币加入白名单**：把测试用 USDC 加入合约白名单

测试网 faucet：
[Sepolia ETH](https://faucet.google.com/ethereum/sepolia) · [Solana Devnet](https://faucet.solana.com) · [Test USDC](https://faucet.circle.com)
