# 数据模型

## 数据库 Schema

SQLite，WAL 模式。启动时通过 GORM 自动迁移表结构：

| 表 | 模型 | 用途 |
|-----|------|------|
| `users` | `User` | 注册用户 |
| `api_keys` | `APIKey` | API key（哈希存储，前缀 `ocw_`） |
| `wallets` | `Wallet` | 钱包，包含链、地址、公钥 |
| `approval_policies` | `ApprovalPolicy` | USD 阈值和每日限额 |
| `approval_requests` | `ApprovalRequest` | 待处理/已批准/已拒绝的审批 |
| `allowed_contracts` | `AllowedContract` | 每个钱包的合约白名单 |
| `audit_logs` | `AuditLog` | 完整的操作审计记录 |
| `idempotency_records` | `IdempotencyRecord` | Idempotency-Key 缓存（24 小时 TTL） |
| `address_book_entries` | `AddressBookEntry` | 每个用户/链的已保存联系人（唯一昵称） |

## GORM 模式

- **启动时自动迁移** —— 服务器启动时自动应用 schema 变更。没有迁移文件。
- **WAL 模式** —— SQLite 配置了预写式日志，以提升并发读取性能。
- **无仓储层** —— handler 直接使用 GORM；代码库足够简单，抽象层不会带来额外价值。

## 链配置

- **内置链** 在启动时从 `chains.json` 加载。
- 完整字段参考请参阅 [chains.json Schema](chains-schema.md)。
