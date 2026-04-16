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

# macOS (已包含在 Xcode Command Line Tools 中)
xcode-select --install

# Alpine
apk add sqlite-dev gcc musl-dev
```

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
..."msg":"server starting","addr":"0.0.0.0:8080"...
```

---

## 3. 验证运行状态

```bash
curl -s http://localhost:8080/api/health
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

在浏览器中打开 [http://localhost:8080](http://localhost:8080)。使用 Passkey 注册，然后进入**设置**生成 API key。密钥以 `ocw_` 开头，仅显示一次——请妥善保存。

然后创建钱包：

```bash
export API_KEY="ocw_..."
curl -s -X POST http://localhost:8080/api/wallets \
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
    "chain": "sepolia",
    "address": "0x1234abcd5678ef90...",
    "label": "Test Wallet",
    "status": "ready"
  }
}
```

---

## 5. 运行测试套件

```bash
make test
```

---

## 开始使用

- [安装与配置](installation.md) -- 完整构建选项、Docker、环境变量
- [核心概念](architecture-overview.md) -- 了解架构
