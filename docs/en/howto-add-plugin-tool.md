# How To: Add a Plugin Tool

## Checklist

1. **Create a tool file** in `plugin/src/tools/` -- use a TypeBox schema for input validation and export a handler function.

2. **Register the tool** in `plugin/index.ts`.

3. **Add an HTTP client method** in `plugin/src/api-client.ts` if the tool calls a new backend endpoint.

4. **For approval-aware tools:** return `pending_approval` status -- the SSE watcher in `approval-watcher.ts` picks it up automatically.

5. **Add tests** in `plugin/src/__tests__/` using `node --test`.

> **Watch out:** The plugin requires `full` tools profile in OpenClaw. Other profiles (`coding`, `messaging`, `minimal`) silently block tools with no error. Check with `openclaw config get tools.profile`.
