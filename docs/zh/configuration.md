# 配置参考

所有配置通过环境变量完成，无需配置文件：

| 环境变量 | 默认值 | 说明 |
|---------|-------|------|
| `SERVICE_URL` | `http://localhost:8089` | 本地 TEENet 服务节点地址 |
| `HOST` | `0.0.0.0` | 服务绑定地址 |
| `PORT` | `18080` | HTTP 监听端口 |
| `DATA_DIR` | `/data` | SQLite 数据库存储目录 |
| `BASE_URL` | `http://localhost:<PORT>` | 公网访问地址（用于审批链接生成） |
| `FRONTEND_URL` | （空） | 允许的 CORS 来源地址；为空则不发送 CORS 头 |
| `CHAINS_FILE` | `./chains.json` | 链配置文件路径 |
| `APP_INSTANCE_ID` | （来自 TEENet） | TEENet 应用实例标识符 |
| `API_KEY_RATE_LIMIT` | `200` | 每个 API Key 每分钟最大请求数 |
| `WALLET_CREATE_RATE_LIMIT` | `5` | 每个 Key 每分钟最大钱包创建数（DKG 资源密集） |
| `REGISTRATION_RATE_LIMIT` | `10` | 每个 IP 每分钟最大注册尝试次数 |
| `APPROVAL_EXPIRY_MINUTES` | `1440` | 待审批请求的过期时间（分钟） |
| `MAX_WALLETS_PER_USER` | `10` | 每个用户可创建的最大钱包数 |
| `MAX_API_KEYS_PER_USER` | `10` | 每个用户最大 API Key 数量 |
| `MAX_USERS` | `500` | 最大注册用户数（0 表示不限制） |
| `SMTP_HOST` | （空） | 邮箱验证码的 SMTP 服务器地址。为空时启用 mock 模式，验证码会打印到 stdout 而不发送邮件。`SMTP_PORT`、`SMTP_USERNAME`、`SMTP_PASSWORD`、`SMTP_FROM` 配置发件账户。 |
| `SMTP_PASSWORD_KEY` | （空） | **生产环境推荐。** 存在 TEE 中的 API key 名称，其 value 作为 SMTP 密码使用。配置后，启动时通过 `sdk.GetAPIKey(name)` 拉取，密码不再出现在 `docker inspect` 或进程 env 中。优先级高于 `SMTP_PASSWORD`。key 拉不到或 TEE 不可达时启动失败。key 必须在部署前以对应 `APP_INSTANCE_ID` 的身份创建——可通过 TEENet 管理平台（实例页面的 API Keys 标签）录入，或程序化调用 `sdk.CreateAPIKey`（示例见 [`teenet-sdk/go/examples/apikey`](https://github.com/TEENet-io/teenet-sdk) / `examples/admin`）。 |
| `DEV_FIXED_CODE` | `999999`（仅 mock 模式） | **仅用于开发。** 在 mock 模式下（即 `SMTP_HOST` 未配置），所有邮箱验证码固定为该值，而非随机 6 位数字——方便本地注册测试，无需 SMTP 也无需翻日志。若 `999999` 与测试数据冲突可覆盖为其它 6 位值。配置了 SMTP 时自动忽略。**生产环境禁止设置。** |

区块链 RPC URL 在 `chains.json` 文件中定义，不作为独立环境变量。可通过 `CHAINS_FILE` 环境变量指定自定义路径。也可在运行时通过 `POST /api/chains` 动态添加自定义 EVM 链。

---
[上一页: 快速开始](/zh/quick-start.md) | [下一页: 认证体系](/zh/authentication.md)
