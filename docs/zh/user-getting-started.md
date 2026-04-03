[wallet-url]: https://test.teenet.io/instance/wallet/

# 快速上手

你告诉 OpenClaw 想做什么，它通过 TEENet Wallet 完成操作，大额交易需要你的指纹确认。以下是完整设置步骤。

---

## 第一步：创建账户

1. 在浏览器中打开 [TEENet Wallet][wallet-url]，推荐使用 Chrome。

2. 点击顶部的 **Register**（注册）。

3. 输入一个显示名称，比如你的昵称。

4. 点击 **Register Device**（注册设备），按提示设置 Passkey：
   - 手机/笔记本：用指纹或面容识别
   - 桌面电脑：用硬件安全密钥或手机扫码

5. 完成。账户已创建，无需记密码。

> **什么是 Passkey？** Passkey 用指纹或面容代替密码。你的生物识别数据不会离开设备，无法被钓鱼或泄露。

## 第二步：生成 API 密钥

API 密钥是 OpenClaw 访问你钱包的凭证。

1. 切换到 **Account**（账户）标签页。

2. 输入标签名（如"my-openclaw"），点击 **Generate API Key**。

3. 通过 Passkey 验证。

4. 立即复制密钥（以 **ocw_** 开头）-- 只显示一次。

## 第三步：连接 OpenClaw

1. 打开 OpenClaw 的对话。

2. 发送 **"安装这个 skill:"** 然后附上链接：

   ```
   https://github.com/TEENet-io/teenet-wallet/blob/master/skill/tee-wallet/SKILL.md
   ```

3. 按提示输入：
   - **TEE_WALLET_API_URL** -- `https://test.teenet.io/instance/f8e649535e1d2838ae2817992f946d6a`
   - **TEE_WALLET_API_KEY** -- 第二步复制的密钥

设置完成，OpenClaw 可以操作你的钱包了。

## 第四步：创建钱包

告诉 OpenClaw：

- **"创建一个以太坊钱包"**
- **"创建一个叫'日常开销'的 Solana 钱包"**

以太坊钱包大约需要 1 分钟（分布式密钥生成），Solana 钱包秒级完成。拿到地址后从交易所或其他钱包充值即可。

## 第五步：设置消费限额

设置审批策略，大额交易需要你的指纹确认。

**通过 Web 界面：**

1. 打开 [TEENet Wallet][wallet-url]，展开钱包，点击 **Policy** 标签页。
2. 设置 **USD 阈值**（如 100 美元）-- 超过这个金额需要审批。
3. 可选：设置 **日限额**（如 5000 美元）-- 每日硬性上限。
4. 点击 **Save** 保存。

**或者直接告诉 OpenClaw：** "设置审批阈值为 100 美元" / "设置日限额为 5000 美元"

策略变更始终需要 Passkey 确认，API 密钥泄露也无法降低你的保护级别。

---
[下一页: 和 OpenClaw 对话](/zh/user-commands.md)
