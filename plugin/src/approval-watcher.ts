// plugin/src/approval-watcher.ts
// SSE listener for approval events. Two modes:
//   A (blocking): waitForApproval() — Promise resolves on SSE event
//   B (subagent): onApprovalResolved() → subagent.run() in original session

import type { WalletAPI } from "./api-client.ts";

export interface ApprovalEvent {
  approval_id: number;
  status: string; // "approved" | "rejected" | "expired"
  approval_type: string;
  tx_hash?: string;
  wallet_id?: string;
}

export type SubagentRun = (opts: {
  sessionKey: string;
  message: string;
  deliver?: boolean;
  idempotencyKey?: string;
}) => Promise<{ runId: string }>;

type Resolver = (event: ApprovalEvent) => void;

type PluginLogger = {
  info?: (message: string, meta?: Record<string, unknown>) => void;
  error?: (message: string, meta?: Record<string, unknown>) => void;
};

export class ApprovalWatcher {
  private api: WalletAPI;
  private subagentRun: SubagentRun | null = null;
  private logger: PluginLogger | null = null;
  private abortController: AbortController | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private cleanupTimer: ReturnType<typeof setInterval> | null = null;
  private connected = false;
  private reconnectDelay = 5000;

  // Mode A: Promise-based waiters.
  private pending: Map<number, Resolver[]> = new Map();
  // Mode B: approval_id → { sessionKey, context, createdAt } for subagent routing.
  private sessionMap: Map<number, { sessionKey: string; context?: string; createdAt: number }> = new Map();

  constructor(api: WalletAPI) {
    this.api = api;
  }

  setSubagentRun(fn: SubagentRun): void {
    this.subagentRun = fn;
  }

  setLogger(logger: PluginLogger): void {
    this.logger = logger;
  }

  /** Associate an approval ID with the session that created it and an optional context description. */
  trackApproval(approvalId: number, sessionKey: string, context?: string): void {
    this.sessionMap.set(approvalId, { sessionKey, context, createdAt: Date.now() });
    this.log("trackApproval", { approvalId, sessionKey, context });
  }

  start(): void {
    this.log("start", { eventsUrl: this.api.eventsUrl });
    this.abortController = new AbortController();
    this.reconnectDelay = 5000;
    void this.connect();

    // Periodic cleanup of stale sessionMap entries (older than 24 hours).
    this.cleanupTimer = setInterval(() => {
      const cutoff = Date.now() - 24 * 60 * 60 * 1000;
      for (const [id, entry] of this.sessionMap) {
        if (entry.createdAt < cutoff) {
          this.log("cleanup.stale", { approvalId: id });
          this.sessionMap.delete(id);
        }
      }
    }, 5 * 60 * 1000);
  }

  stop(): void {
    this.log("stop");
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.cleanupTimer) {
      clearInterval(this.cleanupTimer);
      this.cleanupTimer = null;
    }
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }
    this.connected = false;
    for (const [id, resolvers] of this.pending) {
      for (const resolve of resolvers) {
        resolve({ approval_id: id, status: "error", approval_type: "unknown" });
      }
    }
    this.pending.clear();
  }

  get isConnected(): boolean {
    return this.connected;
  }

  /** Mode A: block until approval resolves. */
  waitForApproval(approvalId: number, timeoutMs: number = 30 * 60 * 1000): Promise<ApprovalEvent> {
    return new Promise<ApprovalEvent>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.removeResolver(approvalId, wrappedResolve);
        reject(new Error(`Approval ${approvalId} timed out after ${timeoutMs / 1000}s`));
      }, timeoutMs);

      const wrappedResolve = (event: ApprovalEvent) => {
        clearTimeout(timer);
        resolve(event);
      };

      const resolvers = this.pending.get(approvalId) || [];
      resolvers.push(wrappedResolve);
      this.pending.set(approvalId, resolvers);
    });
  }

  // ── SSE connection ────────────────────────────────────────────────

  private async connect(): Promise<void> {
    if (!this.abortController) return;

    try {
      const response = await fetch(this.api.eventsUrl, {
        headers: { Authorization: this.api.authHeader },
        signal: this.abortController.signal,
      });

      if (!response.ok || !response.body) {
        throw new Error(`SSE connection failed: ${response.status}`);
      }

      this.connected = true;
      this.reconnectDelay = 5000; // Reset backoff on successful connect.
      this.log("connected");

      // After (re)connect, check tracked approvals that may have resolved while disconnected.
      void this.reconcileTrackedApprovals();

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const messages = buffer.split("\n\n");
        buffer = messages.pop() || "";

        for (const msg of messages) {
          this.handleSSEMessage(msg);
        }
      }
    } catch (err) {
      if (this.abortController?.signal.aborted) return;
      this.logger?.error?.(`[teenet-wallet] SSE connect error`, { error: String(err) });
    }

    this.connected = false;
    if (this.abortController && !this.abortController.signal.aborted) {
      this.reconnectTimer = setTimeout(() => void this.connect(), this.reconnectDelay);
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, 60000);
    }
  }

  private handleSSEMessage(raw: string): void {
    let eventType = "";
    let data = "";

    for (const line of raw.split("\n")) {
      if (line.startsWith("event: ")) {
        eventType = line.slice(7).trim();
      } else if (line.startsWith("data: ")) {
        data = line.slice(6).trim();
      }
    }

    if (eventType === "approval_resolved" && data) {
      try {
        const event = JSON.parse(data) as ApprovalEvent;
        this.log("approval_resolved", { id: event.approval_id, status: event.status });
        this.onApprovalResolved(event);
      } catch {
        // Ignore malformed events.
      }
    }
  }

  // ── Event handling ────────────────────────────────────────────────

  private onApprovalResolved(event: ApprovalEvent): void {
    // Mode A: resolve blocking waiters.
    const resolvers = this.pending.get(event.approval_id);
    if (resolvers) {
      for (const resolve of resolvers) {
        resolve(event);
      }
      this.pending.delete(event.approval_id);
    }

    // Mode B: notify via subagent in the original session.
    const tracked = this.sessionMap.get(event.approval_id);
    if (tracked) {
      this.sessionMap.delete(event.approval_id);
      const { sessionKey, context } = tracked;
      const message = formatApprovalMessage(event);
      const fullMessage = `System: ${message}`;

      if (this.subagentRun) {
        this.log("notify.subagent", { approvalId: event.approval_id, sessionKey, message });
        this.subagentRun({
          sessionKey,
          message: fullMessage,
          deliver: true,
          idempotencyKey: `approval-${event.approval_id}`,
        }).then((res) => {
          this.log("notify.subagent.ok", { approvalId: event.approval_id, runId: res?.runId });
        }).catch((err) => {
          const errStr = (err as Error)?.stack || String(err);
          this.logger?.error?.(`[teenet-wallet] notify.subagent.error: ${errStr}`);
        });
      } else {
        this.log("notify.no-subagent", { approvalId: event.approval_id, sessionKey });
      }
    }
  }

  /**
   * After reconnect, poll tracked approvals to catch events missed during disconnect.
   * If an approval resolved while SSE was down, trigger notification now.
   */
  private async reconcileTrackedApprovals(): Promise<void> {
    if (this.sessionMap.size === 0) return;

    const ids = [...this.sessionMap.keys()];
    this.log("reconcile.start", { count: ids.length });

    const results = await Promise.allSettled(
      ids.map(async (approvalId) => {
        if (!this.sessionMap.has(approvalId)) return; // already handled by SSE
        const approval = await this.api.getApproval(approvalId);
        const status = approval?.status as string;
        if (status && status !== "pending") {
          this.log("reconcile.found", { approvalId, status });
          this.onApprovalResolved({
            approval_id: approvalId,
            status,
            approval_type: approval.approval_type || "unknown",
            tx_hash: approval.tx_hash,
            wallet_id: approval.wallet_id,
          });
        }
      }),
    );
    // Log any API errors (rejected promises); they will retry on next reconnect.
    for (const r of results) {
      if (r.status === "rejected") {
        this.log("reconcile.error", { error: String(r.reason) });
      }
    }
  }

  private removeResolver(approvalId: number, resolver: Resolver): void {
    const resolvers = this.pending.get(approvalId);
    if (resolvers) {
      const idx = resolvers.indexOf(resolver);
      if (idx >= 0) resolvers.splice(idx, 1);
      if (resolvers.length === 0) this.pending.delete(approvalId);
    }
  }

  private log(msg: string, meta?: Record<string, unknown>): void {
    this.logger?.info?.(`[teenet-wallet] ${msg}`, meta);
  }
}

function formatApprovalMessage(event: ApprovalEvent): string {
  const id = event.approval_id;
  switch (event.status) {
    case "approved":
      if (event.tx_hash) {
        return `Approval #${id} approved. Transaction broadcast: ${event.tx_hash}. Please share the explorer link with the user.`;
      }
      return `Approval #${id} approved (${event.approval_type}).`;
    case "rejected":
      return `Approval #${id} was rejected. No action was taken.`;
    case "expired":
      return `Approval #${id} has expired. Please try again.`;
    default:
      return `Approval #${id} status: ${event.status}`;
  }
}
