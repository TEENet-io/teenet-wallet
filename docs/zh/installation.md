# 安装与配置

teenet-wallet 完整安装参考。如需最快上手路径，请参阅[快速开始](quick-start.md)。

---

## 支持的平台

| 平台 | 说明 |
|------|------|
| Linux (Debian/Ubuntu) | 主要目标平台 |
| Linux (Alpine) | 需要额外的 CGo 相关包 |
| macOS | 需要 Xcode Command Line Tools |
| Docker | 包含多阶段构建 |

---

## Go 版本

需要 Go **1.25+**。CGo 必须启用（`CGO_ENABLED=1`，这是默认值），因为 SQLite 驱动是一个 C 库。

---

## 依赖项

SQLite3 开发头文件是唯一的外部依赖。

```bash
# Debian / Ubuntu
sudo apt-get install libsqlite3-dev

# Alpine
apk add sqlite-dev gcc musl-dev

# macOS (已包含在 Xcode Command Line Tools 中)
xcode-select --install
```

---

## 从源码构建

```bash
git clone https://github.com/TEENet-io/teenet-wallet.git
cd teenet-wallet
git submodule update --init --recursive
make frontend
make build
```

二进制文件输出到项目根目录的 `./teenet-wallet`。

同时构建前端：

```bash
git submodule update --init
make frontend
```

---

## Docker

```bash
make docker
docker run -p 8080:8080 \
  -e SERVICE_URL=http://host.docker.internal:8089 \
  -v wallet-data:/data \
  teenet-wallet:latest
```

镜像使用多阶段构建。`host.docker.internal` 路由到宿主机，使容器可以访问在 Docker 外部运行的 mock 服务。

---

## Mock 服务

Mock 服务在开发期间替代 TEENet 服务。它实现了完整的 TEENet 服务 HTTP API 并使用真实的密码学签名，因此钱包的行为与连接生产环境一致。

```bash
git clone https://github.com/TEENet-io/teenet-sdk.git
cd teenet-sdk/mock-server
make run
```

Mock 服务默认监听 `127.0.0.1:8089`。

**自定义端口和绑定地址：**

```bash
MOCK_SERVER_PORT=9090 MOCK_SERVER_BIND=0.0.0.0 ./mock-server
```

如果更改了端口，启动钱包时需要相应更新 `SERVICE_URL`。

> Mock 服务仅使用内存存储——重启后状态会重置。请勿在生产环境中使用。

---

## 环境变量

最重要的变量：

| 变量 | 默认值 | 描述 |
|------|--------|------|
| `SERVICE_URL` | `http://localhost:8089` | TEENet 服务端点 |
| `DATA_DIR` | `/data` | SQLite 数据库文件（`wallet.db`）所在目录 |
| `BASE_URL` | `http://localhost:<PORT>` | 用于审批链接的公开访问 URL |
| `FRONTEND_URL` | _(空)_ | Web UI 的 CORS 允许来源 |

如果你是在本地通过 `teenet-sdk/mock-server` 开发，还需要把 `APP_INSTANCE_ID` 设置为 mock server 启动时打印出的某个 app ID。如果是从源码直接运行钱包，请把 `DATA_DIR` 设置为可写的本地目录，例如 `./data`。

完整环境变量参考请参阅[配置](configuration.md)。

---

## chains.json

链定义（名称、RPC URL、链 ID、曲线、协议）存放在项目根目录的 `chains.json` 中。可通过 API 在运行时添加额外的 EVM 链。

完整字段参考请参阅 [chains.json Schema](chains-schema.md)。

---

## 前端子模块

Web UI 是一个单文件 SPA，存放在 git 子模块中。初始化方式：

```bash
git submodule update --init
make frontend
```

前端文件必须位于 `./frontend/` 目录中，服务器才能提供服务。

---

## 验证安装

**健康检查：**

```bash
curl -s http://localhost:8080/api/health
```

**创建用户：** 打开 [http://localhost:8080](http://localhost:8080) 并完成 Passkey 注册流程。

**创建钱包：** 从 **设置** 中生成 API key，然后通过 API 创建钱包。

完整的分步操作指南请参阅[快速开始](quick-start.md)。
