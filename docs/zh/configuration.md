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
| `API_KEY_RATE_LIMIT` | `100` | 每个 API Key 每分钟最大请求数 |
| `WALLET_CREATE_RATE_LIMIT` | `5` | 每个 Key 每分钟最大钱包创建数（DKG 资源密集） |
| `REGISTRATION_RATE_LIMIT` | `10` | 每个 IP 每分钟最大注册尝试次数 |
| `RPC_RATE_LIMIT` | `50` | 每个用户每分钟走上游 RPC 的总上限 —— 读（`/call-read`、`/balance`）和资金移动类（`/transfer`、`/contract-call`、`/approve-token`、`/revoke-approval`、`/wrap-sol`、`/unwrap-sol`）共用同一个桶。 |
| `APPROVAL_EXPIRY_MINUTES` | `1440` | 待审批请求的过期时间（分钟） |
| `MAX_WALLETS_PER_USER` | `10` | 每个用户可创建的最大钱包数 |
| `MAX_API_KEYS_PER_USER` | `10` | 每个用户最大 API Key 数量 |
| `MAX_USERS` | `500` | 最大注册用户数（0 表示不限制） |
| `SMTP_HOST` | （空） | 邮箱验证码的 SMTP 服务器地址。为空时启用 mock 模式，验证码会打印到 stdout 而不发送邮件。`SMTP_PORT`、`SMTP_USERNAME`、`SMTP_PASSWORD`、`SMTP_FROM` 配置发件账户。 |
| `SMTP_PASSWORD_KEY` | （空） | **生产环境推荐。** 存在 TEE 中的 API key 名称，其 value 作为 SMTP 密码使用。配置后，启动时通过 `sdk.GetAPIKey(name)` 拉取，密码不再出现在 `docker inspect` 或进程 env 中。优先级高于 `SMTP_PASSWORD`。key 拉不到或 TEE 不可达时启动失败。key 必须在部署前以对应 `APP_INSTANCE_ID` 的身份创建——可通过 TEENet 管理平台（实例页面的 API Keys 标签）录入，或程序化调用 `sdk.CreateAPIKey`（示例见 [`teenet-sdk/go/examples/apikey`](https://github.com/TEENet-io/teenet-sdk) / `examples/admin`）。 |
| `DEV_FIXED_CODE` | `999999`（仅 mock 模式） | **仅用于开发。** 在 mock 模式下（即 `SMTP_HOST` 未配置），所有邮箱验证码固定为该值，而非随机 6 位数字——方便本地注册测试，无需 SMTP 也无需翻日志。若 `999999` 与测试数据冲突可覆盖为其它 6 位值。配置了 SMTP 时自动忽略。**生产环境禁止设置。** |
| `QUICKNODE_ENDPOINT` | （空） | QuickNode endpoint 子域名（例如 `wispy-wiser-road`）。配置后，`chains.json` 中每条填了 `quicknode_network` 字段的链,启动时 `rpc_url` 会被改写为 `https://{endpoint}.{network}.quiknode.pro/{token}/`。未配置则保留各链的公共 fallback RPC。 |
| `QUICKNODE_TOKEN` | （空） | QuickNode 访问 token（URL 路径部分）。会出现在 `docker inspect` 中，生产环境请用 `QUICKNODE_TOKEN_KEY`。 |
| `QUICKNODE_TOKEN_KEY` | （空） | **生产环境推荐。** 存在 TEE 中的 API key 名称，其 value 作为 QuickNode token 使用。启动时通过 `sdk.GetAPIKey(name)` 拉取，token 不再出现在 docker env 或进程 env 中。优先级高于 `QUICKNODE_TOKEN`。key 拉不到或 TEE 不可达时启动失败。可通过 TEENet 管理平台（实例页面 API Keys 标签）录入，或程序化调用 `sdk.CreateAPIKey`。 |
| `PRICE_CACHE_TTL` | `120`（Docker 镜像）/ `60`（代码默认） | CoinGecko USD 价格缓存 TTL，单位秒。同时决定后台刷新线程的轮询间隔。镜像默认 `120`，因为 CoinGecko 免费档对单个 IP 的实际频率限制大概是每 2 分钟 1 次，拉得更快只会刷出一堆无害的 429 警告。有 CoinGecko 付费计划时可下调到 `60`。 |

区块链 RPC URL 在 `chains.json` 文件中定义，不作为独立环境变量。可通过 `CHAINS_FILE` 环境变量指定自定义路径。新增/删除链请编辑 `chains.json` 并重启服务——该文件仅在启动时加载一次。

### QuickNode RPC 覆写

`publicnode.com` 等公共 RPC 限流激进、偶尔抽风。要走 [QuickNode](https://www.quicknode.com/):

1. 在 QuickNode 建一个 endpoint,勾选需要的网络(一个 endpoint + 一个 token 可通过 "Multi-Chain" 选项服务多条链)。
2. 在 `chains.json` 对应链的条目里加 `quicknode_network` 字段,值从 QuickNode dashboard 的 URL 中提取——例如 `https://wispy-wiser-road.ethereum-sepolia.quiknode.pro/...` → `"quicknode_network": "ethereum-sepolia"`。Ethereum Mainnet 是特例:QuickNode 没有子域,用 `"quicknode_network": "-"`。需要 token 后路径后缀的链(Avalanche C-Chain 的 `/ext/bc/C/rpc`),再加 `"quicknode_path": "/ext/bc/C/rpc"`。
3. 在钱包容器上设置 `QUICKNODE_ENDPOINT`(子域名) 和 `QUICKNODE_TOKEN`(开发) 或 `QUICKNODE_TOKEN_KEY`(生产)。

启动时钱包会把匹配链的 `rpc_url` 改写掉。没填 `quicknode_network` 的链不受影响。内置 `chains.json` 对所有默认链都已填好 slug。

**运行时 fallback:** `rpc_url` 被覆写时,`chains.json` 里原来的 URL(publicnode 等)会被注册为 fallback。QuickNode 请求遇到 transport 或 HTTP 错误(DNS 失败、超时、5xx、429)时,RPC 层会对当前这一次调用透明切到 fallback 重试,后续调用仍然优先走 QuickNode。应用层 JSON-RPC 错误(`execution reverted`、`nonce too low` 等)会直接短路返回,不触发 fallback——换个 provider 返的也是同一个错。每次 fallback 会在日志里打出 host(不是完整 URL,后者含 token)。

---
[上一页: 快速开始](/zh/quick-start.md) | [下一页: 认证体系](/zh/authentication.md)
