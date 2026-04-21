// Shim for openclaw/plugin-sdk/plugin-entry.
//
// OpenClaw ships the plugin SDK globally (resolved at runtime by the host
// gateway), but its package.json does not expose `plugin-sdk/plugin-entry`
// via the `exports` map — Node16 moduleResolution therefore refuses it at
// typecheck time. This shim lets tsc resolve the import locally with a
// minimal, hand-rolled type surface sufficient for our usage. It has no
// runtime effect; the real module is loaded by the gateway.
//
// Keep in sync with openclaw/dist/plugin-sdk/plugin-entry.d.ts if we start
// exercising new fields on the plugin api.

declare module "openclaw/plugin-sdk/plugin-entry" {
  export interface PluginLogger {
    // Real openclaw signature takes only `message: string`. The extra `meta`
    // arg is ignored by the host logger but accepted here so plugin callsites
    // can still pass structured context.
    debug?: (message: string, meta?: Record<string, unknown>) => void;
    info: (message: string, meta?: Record<string, unknown>) => void;
    warn: (message: string, meta?: Record<string, unknown>) => void;
    error: (message: string, meta?: Record<string, unknown>) => void;
  }

  export interface SubagentRun {
    run?: (opts: {
      sessionKey: string;
      message: string;
      deliver?: boolean;
      idempotencyKey?: string;
    }) => Promise<{ runId: string }>;
  }

  export interface OpenClawPluginApi {
    pluginConfig: unknown;
    logger: PluginLogger;
    runtime?: {
      subagent?: SubagentRun;
    };
    dataDir?: string;
    registerTool: (tool: unknown) => void;
    registerService: (service: {
      id: string;
      start?: () => void | Promise<void>;
      stop?: () => void | Promise<void>;
    }) => void;
  }

  export interface DefinePluginEntryOptions {
    id: string;
    name: string;
    description: string;
    register: (api: OpenClawPluginApi) => void;
  }

  export function definePluginEntry(options: DefinePluginEntryOptions): unknown;
}
