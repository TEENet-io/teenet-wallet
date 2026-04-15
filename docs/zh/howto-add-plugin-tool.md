# 操作指南：添加插件工具

## 检查清单

1. **在 `plugin/src/tools/` 中创建工具文件** —— 使用 TypeBox schema 进行输入验证，并导出一个处理函数。

2. **在 `plugin/index.ts` 中注册工具**。

3. **在 `plugin/src/api-client.ts` 中添加 HTTP 客户端方法**（如果工具调用了新的后端端点）。

4. **对于需要审批的工具：** 返回 `pending_approval` 状态 —— `approval-watcher.ts` 中的 SSE 监听器会自动处理。

5. **在 `plugin/src/__tests__/` 中添加测试**，使用 `node --test`。

> **注意：** 插件需要在 OpenClaw 中设置 `full` 工具配置。其他配置（`coding`、`messaging`、`minimal`）会静默阻止工具，不会报错。可通过 `openclaw config get tools.profile` 检查。
