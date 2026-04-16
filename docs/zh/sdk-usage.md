# TEENet SDK 使用指南

本页介绍钱包所使用的 TEENet SDK 接口（`github.com/TEENet-io/teenet-sdk/go`），涵盖初始化、密钥生成、签名、Passkey 集成以及 nil-client 测试模式。

---

## 初始化

SDK 客户端在启动时创建一次：

```go
opts := &sdk.ClientOptions{
    RequestTimeout:     3 * time.Minute,
    PendingWaitTimeout: 3 * time.Minute,
}
sdkClient := sdk.NewClientWithOptions(serviceURL, opts)
```

- `serviceURL` 来自环境变量 `SERVICE_URL`（默认值：`http://localhost:8089`）。
- 客户端在关闭时通过 `defer sdkClient.Close()` 释放。

创建后，客户端加载应用身份标识：

```go
sdkClient.SetDefaultAppInstanceIDFromEnv()
```

此调用从环境中读取 `APP_INSTANCE_ID`。

---

## 密钥生成

```go
keyResult, err := sdkClient.GenerateKey(ctx, scheme, curve)
```

| 链族 | Scheme | Curve | 示例 |
|---|---|---|---|
| EVM（Ethereum、Avalanche 等） | `ecdsa` | `secp256k1` | `GenerateKey(ctx, "ecdsa", "secp256k1")` |
| Solana | `ed25519` | `ed25519` | `GenerateKey(ctx, "ed25519", "ed25519")` |

> **注意：** SDK 密钥生成接口正在更新中。当前代码可能仍使用 `GenerateECDSAKey` / `GenerateSchnorrKey`。请以实际代码中的函数签名为准。

返回值：

- `keyResult.PublicKey.Name` -- 唯一的密钥标识符，后续所有签名调用均使用此值。
- `keyResult.PublicKey.KeyData` -- 原始公钥字节，用于派生链地址。
- `keyResult.Success` 和 `keyResult.Message` -- 状态字段。

---

## 签名

```go
result, err := sdkClient.Sign(ctx, msgBytes, keyName)
```

- `msgBytes` -- 待签名的原始消息字节。
- `keyName` -- 来自 `GenerateKey` 的密钥标识符。
- `result.Signature` -- 返回的签名字节。
- `result.Success` -- 签名是否成功。

---

## Passkey 集成

SDK 处理所有 WebAuthn 流程：

| 方法 | 用途 |
|---|---|
| `InvitePasskeyUser` | 为新的 Passkey 用户创建邀请 |
| `PasskeyRegistrationOptions` | 获取 WebAuthn 注册挑战 |
| `PasskeyRegistrationVerify` | 注册后验证 WebAuthn 凭据 |
| `PasskeyLoginOptions` | 获取 WebAuthn 登录挑战 |
| `PasskeyLoginVerify` | 验证登录断言 |
| `PasskeyLoginVerifyAs` | 验证登录断言并确认其匹配特定用户 |
| `DeletePasskeyUser` | 从 TEENet service 中移除 Passkey 用户 |

`PasskeyLoginVerifyAs` 用于审批流程：它既确认硬件密钥断言有效，**又**验证审批者是钱包所有者。

---

## 密钥删除

```go
sdkClient.DeletePublicKey(ctx, keyName)
sdkClient.DeletePasskeyUser(ctx, passkeyUserID)
```

---

## Nil Client 模式

在测试中，SDK 客户端被设置为 `nil`。Handler 会正常执行验证、数据库操作和交易构建，然后在签名步骤失败。这是设计如此 -- 测试验证除实际签名以外的所有环节。需要真实签名的集成测试使用 Mock 服务。

---

## Mock 服务

`teenet-sdk/mock-server` 在本地开发时替代 TEENet service：

```bash
cd teenet-sdk/mock-server
make run    # listens on 127.0.0.1:8089 and sets localhost Passkey defaults
```

- 实现与生产环境 TEENet service 相同的 HTTP API。
- 执行真实的密码学运算（ECDSA、Ed25519 签名）。
- 使用确定性密钥以获得可复现的行为。
- 重启后清除所有状态（仅内存存储）。

**请勿用于生产环境** -- 确定性密钥是不安全的。

---

## 后续阅读

- [签名与 TEE 信任模型](signing-tee.md) -- TEENet service 内部的运作机制
- [架构概览](architecture-overview.md) -- 系统整体模型
