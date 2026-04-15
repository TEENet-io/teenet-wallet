# 贡献流程与 PR 检查清单

## 工作流程

1. **Fork** 仓库并克隆到本地：
   ```bash
   git clone https://github.com/<your-username>/teenet-wallet.git
   cd teenet-wallet
   ```

2. 从 `main` 分支创建**新分支**：
   ```bash
   git checkout -b feature/my-change
   ```

3. 按照[编码规范](coding-standards.md)**实现**你的更改。

4. 在本地**测试**：
   ```bash
   make lint
   make test
   ```

5. **推送**并向 `main` 分支提交 **pull request**。

## 提交信息规范

使用以下约定前缀：

- `feat:` -- 新功能
- `fix:` -- 修复 Bug
- `docs:` -- 仅文档变更
- `refactor:` -- 既不修复 Bug 也不添加功能的代码重构
- `test:` -- 添加或更新测试
- `chore:` -- 维护工作（CI、依赖、构建）

示例：`feat: add support for Arbitrum chain`

## PR 描述模板

每个 pull request 应包含以下内容：

- **变更内容** -- 本次更改做了什么
- **变更原因** -- 为什么需要此更改
- **测试方法** -- 如何测试
- **破坏性变更**或迁移步骤（如有）

## CI 要求

合并前 CI 必须通过。流水线会在每个 PR 上运行 lint 检查、测试（含竞态检测器）和漏洞扫描。

## 安全漏洞

如果你发现安全漏洞，请按照 [SECURITY.md](https://github.com/TEENet-io/teenet-wallet/blob/main/SECURITY.md) 中的负责任披露流程处理。请勿通过公开 issue 报告安全问题。

## PR 检查清单

在请求审查之前，请确认：

- [ ] `make lint` 通过
- [ ] `make test` 通过（含竞态检测器）
- [ ] 新端点已添加测试
- [ ] 如 API 有变更，已更新 OpenAPI 规范
- [ ] 如用户可见行为有变更，已更新文档
