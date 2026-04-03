[wallet-url]: https://test.teenet.io/instance/wallet/

# 快速上手

你告诉 OpenClaw 想做什么，它通过 TEENet Wallet 完成操作，大额交易需要你的指纹确认。以下是完整设置步骤。

---

## 第一步：创建账户

1. 在浏览器中打开 [TEENet Wallet][wallet-url]，推荐使用 Chrome。

2. 输入你的姓名或邮箱（必填）。

<div align="center"><img src="picture/register.png" alt="注册页面" width="360" /></div>

3. 点击 **使用 Passkey 注册**，按提示设置 Passkey：
   - 手机/笔记本：用指纹或面容识别
   - 桌面电脑：用硬件安全密钥或手机扫码

<div align="center"><img src="picture/register2.png" alt="创建 Passkey" width="360" /></div>

4. 完成。账户已创建，无需记密码。

> **什么是 Passkey？** Passkey 用指纹或面容代替密码。你的生物识别数据不会离开设备，无法被钓鱼或泄露。

## 第二步：生成 API 密钥

API 密钥是 OpenClaw 访问你钱包的凭证。

1. 点击右上角的 **设置** 图标（齿轮）。

2. 在 API Keys 区域，输入标签名（如"my-openclaw"），点击 **Generate API Key**。

3. 通过 Passkey 验证。

4. 立即复制密钥（以 **ocw_** 开头）-- 只显示一次。

<div align="center"><img src="picture/generate_api.png" alt="API 密钥已生成" width="360" /></div>

## 第三步：连接 OpenClaw

1. 打开 OpenClaw 的对话。

2. 发送 **"安装这个 skill:"** 然后附上链接：

   ```
   https://github.com/TEENet-io/teenet-wallet/blob/master/skill/tee-wallet/SKILL.md
   ```

3. 按提示输入：
   - **TEE_WALLET_API_URL** -- `https://test.teenet.io/instance/f8e649535e1d2838ae2817992f946d6a`
   - **TEE_WALLET_API_KEY** -- 第二步复制的密钥

<div align="center"><img src="picture/tg.png" alt="通过 Telegram 连接 OpenClaw" width="360" /></div>

设置完成，OpenClaw 可以操作你的钱包了。

## 第四步：创建钱包

告诉 OpenClaw：

- **"创建一个以太坊钱包"**
- **"创建一个叫'日常开销'的 Solana 钱包"**

以太坊钱包大约需要 1 分钟（分布式密钥生成），Solana 钱包秒级完成。拿到地址后从交易所或其他钱包充值即可。

<div align="center"><img src="picture/create.png" alt="通过 OpenClaw 创建钱包" width="480" /></div>

## 第五步：设置消费限额

设置审批策略，大额交易需要你的指纹确认。

**通过 Web 界面：**

1. 打开 [TEENet Wallet][wallet-url]，点击钱包进入详情页，选择 **阈值** 标签页。
2. 设置 **审批阈值（USD）**（如 100 美元）-- 超过这个金额需要审批。
3. 可选：设置 **日上限（USD）**（如 5000 美元）-- 每日硬性上限。
4. 点击 **保存策略**。

**或者直接告诉 OpenClaw：** "设置审批阈值为 100 美元" / "设置日限额为 5000 美元"

<div align="center"><img src="picture/threshold.png" alt="通过 OpenClaw 设置阈值" width="480" /></div>

策略变更始终需要 Passkey 确认，API 密钥泄露也无法降低你的保护级别。

---
[下一页: 和 OpenClaw 对话](/zh/user-commands.md)
