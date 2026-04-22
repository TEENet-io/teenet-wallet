# 快速开始

从零到运行钱包只需 5 分钟。每个步骤都可以直接复制粘贴。

---

## 前提条件

- **Go 1.25+**
- **SQLite3 开发头文件**

为你的平台安装 SQLite 头文件：

```bash
# Debian / Ubuntu
sudo apt-get install libsqlite3-dev

# RHEL / Fedora / Rocky / AlmaLinux / 阿里云 Linux
sudo dnf install sqlite-devel

# macOS (已包含在 Xcode Command Line Tools 中)
xcode-select --install

# Alpine
apk add sqlite-dev gcc musl-dev
```

---

## TL;DR —— 一键启动

装好依赖以后,下面那一整套(clone SDK、构建两个服务、按匹配的端口和 origin 起来、健康检查)可以缩成一条:

```bash
./scripts/dev.sh up
```

可覆盖的环境变量:`MOCK_PORT=` / `WALLET_PORT=` / `APP_INSTANCE_ID=`,或 `AUTO_PORT=1` 自动跳过被占端口。配套还有 `down` / `status` / `logs`。运行时状态(PID / 日志 / SQLite)都在 `.dev/`。

用脚本的话直接跳到 **Step 3** 验证,再 **Step 4** 注册 + 建钱包。下面的手工步骤保留是为了让你看到每一步具体在做什么,或者一段一段单独跑。

---

## 1. 启动 mock 服务

Mock 服务在开发期间替代 TEENet 服务。打开一个终端：

```bash
git clone https://github.com/TEENet-io/teenet-sdk.git
cd teenet-sdk/mock-server
make run
```

预期输出：

```
Mock server starting on port 8089
Available test App IDs: ...
```

启动后，mock server 会打印可用的 app ID。把其中一个值作为启动钱包时的 `APP_INSTANCE_ID`。

保持此终端运行。

> **钱包端口不是默认的 18080?** WebAuthn 要求 `scheme://host:port` 精确匹配,若你在第 2 步自定义了 `PORT`,启动 mock 时要同步传入:`PASSKEY_RP_ORIGIN=http://localhost:<port> make run`。默认 `:18080` 跟钱包默认端口一致,不用改。
>
> **8089 端口被占?** 用 `MOCK_SERVER_PORT=<port>` 覆盖,并在第 2 步相应修改 `SERVICE_URL`。

---

## 2. 构建并运行钱包

打开一个**新终端**：

```bash
git clone https://github.com/TEENet-io/teenet-wallet.git
cd teenet-wallet
git submodule update --init --recursive
make frontend
make build
APP_INSTANCE_ID=<mock-app-instance-id> DATA_DIR=./data SERVICE_URL=http://127.0.0.1:8089 ./teenet-wallet
```

`DATA_DIR=./data` 会把 SQLite 数据库存放到当前项目中可写的目录。

预期输出：

```
..."msg":"server starting","addr":"0.0.0.0:18080"...
```

> **18080 端口被占?** 在启动命令里加上 `PORT=<port>`,并在第 1 步启动 mock 时同步传入 `PASSKEY_RP_ORIGIN=http://localhost:<port>`。

---

## 3. 验证运行状态

```bash
curl -s http://localhost:18080/api/health
```

预期响应：

```json
{
  "db": true,
  "service": "teenet-wallet",
  "status": "ok"
}
```

---

## 4. 创建你的第一个钱包

在浏览器中打开 [http://localhost:18080](http://localhost:18080)，按以下步骤注册：

1. 输入邮箱，点击 **发送验证码**
2. 填入 6 位验证码。**mock 模式下默认固定为 `999999`**，无需查收邮件（可通过 [`DEV_FIXED_CODE`](configuration.md) 覆盖；配置了 `SMTP_HOST` 则走真实邮件）
3. 使用 Passkey 完成注册

注册成功后进入**设置**生成 API key。密钥以 `ocw_` 开头，仅显示一次——请妥善保存。

然后创建钱包：

```bash
export API_KEY="ocw_..."
curl -s -X POST http://localhost:18080/api/wallets \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"chain": "sepolia", "label": "Test Wallet"}'
```

预期响应：

```json
{
  "success": true,
  "wallet": {
    "id": "8a2fbc16-faf4-451a-be34-9fc5c49cde00",
    "user_id": 1,
    "chain": "sepolia",
    "key_name": "wallet-8a2fbc16...",
    "public_key": "03abcd...",
    "address": "0x1234abcd5678ef90...",
    "label": "Test Wallet",
    "curve": "secp256k1",
    "protocol": "ecdsa",
    "status": "ready",
    "created_at": "2026-04-22T10:30:00Z"
  }
}
```

---

## 5. 运行测试套件

```bash
make test
```

---

## 6. 连接本地 OpenClaw Agent（可选）

拿到第 4 步的 API key 后，可以把本地 OpenClaw 指向这个钱包，跑一次完整的 Agent 端到端测试。

在 OpenClaw 对话里安装 skill：

```
Install this skill: https://github.com/TEENet-io/teenet-wallet/blob/main/skill/teenet-wallet/SKILL.md
```

按提示填入：

- **TEENET_WALLET_API_URL** —— `http://localhost:18080`（若 OpenClaw 跑在其它机器上，改成本机 LAN IP）
- **TEENET_WALLET_API_KEY** —— 第 4 步生成的 `ocw_...` key

然后让 Agent 跑一次端到端快速检查：

```
/test
```

这会依次走一遍余额查询、测试网 faucet、转账、审批阈值、合约白名单流程,整条链路都打通。

---

## 开始使用

- [安装与配置](installation.md) -- 完整构建选项、Docker、环境变量
- [核心概念](architecture-overview.md) -- 了解架构
