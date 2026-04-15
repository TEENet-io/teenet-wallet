# 故障排除

常见问题及解决方法。

---

## CGo / SQLite 构建失败

**问题：** 构建失败，出现缺少 SQLite 符号或 C 编译器相关的错误。

**原因：** 缺少 SQLite3 开发头文件、Go 版本不正确或 CGo 被禁用。

**解决方法：**

- 为你的平台安装头文件：
  ```bash
  # Debian / Ubuntu
  sudo apt-get install libsqlite3-dev

  # Alpine
  apk add sqlite-dev gcc musl-dev
  ```
- 验证 Go 版本为 1.24+：`go version`
- 验证 CGo 已启用：`go env CGO_ENABLED` 应输出 `1`。如果输出 `0`，使用 `CGO_ENABLED=1 make build` 运行。CGo 默认启用，但某些环境可能会覆盖此设置。

---

## 连接 :8089 被拒绝

**问题：** 钱包日志中出现连接端口 8089 的错误。

**原因：** Mock 服务未运行，或运行在不同的端口上。

**解决方法：**

- 启动 mock 服务：`cd teenet-sdk/mock-server && go build && ./mock-server`
- 如果你使用了自定义端口（`MOCK_SERVER_PORT=9090`），需要更新钱包的 `SERVICE_URL`：
  ```bash
  SERVICE_URL=http://127.0.0.1:9090 ./teenet-wallet
  ```

---

## API key 无效

**问题：** API 请求返回 `401 Unauthorized` 并提示 "invalid API key" 错误。

**原因：** API key 复制错误，或已被撤销。API key 仅在生成时显示一次。

**解决方法：** 在 Web UI 的**设置**中生成新的 API key。

---

## 查看 SQLite 数据库

调试时，你可以直接查询数据库：

```bash
sqlite3 /data/wallet.db
.tables
SELECT id, chain, address, status FROM wallets;
SELECT * FROM approval_requests WHERE status = 'pending';
SELECT action, created_at FROM audit_logs ORDER BY created_at DESC LIMIT 10;
```

如果你更改了 `DATA_DIR`，请将 `/data` 替换为你配置的目录。

---

## 开发时触发速率限制

**问题：** API 请求返回 `429 Too Many Requests`。

**原因：** 默认速率限制较为保守：每个 API key 每分钟 200 次请求，钱包创建每分钟 5 次。

**解决方法：** 使用环境变量覆盖限制：

```bash
API_KEY_RATE_LIMIT=1000 WALLET_CREATE_RATE_LIMIT=50 ./teenet-wallet
```

---

## 前端无法加载

**问题：** 打开 http://localhost:8080 显示空白页面或 404。

**原因：** 前端子模块未初始化，或构建后的文件缺失。

**解决方法：**

```bash
git submodule update --init
make frontend
```

前端文件必须位于 `./frontend/` 目录中。构建后需要重启钱包。
