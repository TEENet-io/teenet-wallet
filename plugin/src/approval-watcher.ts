// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

// plugin/src/approval-watcher.ts
// SSE listener for approval events.
// On approval resolution, notifies the original session via subagent.run().

import fs from "node:fs";
import path from "node:path";
import type { WalletAPI } from "./api-client.js";

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

type PluginLogger = {
  info?: (message: string, meta?: Record<string, unknown>) => void;
  error?: (message: string, meta?: Record<string, unknown>) => void;
};

type ApprovalTrackingEntry = {
  sessionKey: string;
  context?: string;
  createdAt: number;
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
  private storagePath = "";

  // approval_id → { sessionKey, context, createdAt } for subagent routing.
  private sessionMap: Map<number, ApprovalTrackingEntry> = new Map();

  constructor(api: WalletAPI) {
    this.api = api;
  }

  /** Set the disk path for persisting approval tracking state. Call before start(). */
  setStoragePath(p: string): void {
    this.storagePath = p;
    this.loadPersistedMap();
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
    this.persistMap();
    this.log("trackApproval", { approvalId, sessionKey, context });
  }

  start(): void {
    this.log("start", { eventsUrl: this.api.eventsUrl, trackedCount: this.sessionMap.size });
    this.abortController = new AbortController();
    this.reconnectDelay = 5000;
    void this.connect();

    // Periodic cleanup of stale sessionMap entries (older than 24 hours).
    this.cleanupTimer = setInterval(() => {
      const cutoff = Date.now() - 24 * 60 * 60 * 1000;
      let changed = false;
      for (const [id, entry] of this.sessionMap) {
        if (entry.createdAt < cutoff) {
          this.log("cleanup.stale", { approvalId: id });
          this.sessionMap.delete(id);
          changed = true;
        }
      }
      if (changed) this.persistMap();
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
  }

  get isConnected(): boolean {
    return this.connected;
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

      try {
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
      } finally {
        reader.releaseLock();
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
    // Notify via subagent in the original session.
    let tracked = this.sessionMap.get(event.approval_id);
    // If not in memory, another watcher instance may have written it to disk.
    // Reload from disk and try again.
    if (!tracked) {
      this.loadPersistedMap();
      tracked = this.sessionMap.get(event.approval_id);
      if (tracked) {
        this.log("notify.recovered-from-disk", { approvalId: event.approval_id });
      }
    }
    if (tracked) {
      this.sessionMap.delete(event.approval_id);
      this.persistMap();
      const { sessionKey, context } = tracked;
      const message = formatApprovalMessage(event);
      const fullMessage = `System: ${message}`;

      if (this.subagentRun) {
        this.log("notify.subagent", { approvalId: event.approval_id, sessionKey, message, context });
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
    } else {
      this.log("notify.missing-track", { approvalId: event.approval_id, status: event.status, mapSize: this.sessionMap.size });
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

  private loadPersistedMap(): void {
    if (!this.storagePath) return;
    try {
      if (!fs.existsSync(this.storagePath)) return;
      const raw = fs.readFileSync(this.storagePath, "utf8");
      const parsed = JSON.parse(raw) as Record<string, ApprovalTrackingEntry>;
      const cutoff = Date.now() - 24 * 60 * 60 * 1000;
      let restored = 0;
      for (const [approvalId, entry] of Object.entries(parsed || {})) {
        const id = Number(approvalId);
        if (!Number.isFinite(id) || !entry?.sessionKey) continue;
        if (entry.createdAt < cutoff) continue; // skip stale entries
        this.sessionMap.set(id, entry);
        restored++;
      }
      if (restored > 0) {
        this.log("persist.load", { restored, total: Object.keys(parsed || {}).length });
      }
    } catch (err) {
      this.logger?.error?.(`[teenet-wallet] loadPersistedMap.error`, { error: String(err) });
    }
  }

  private persistMap(): void {
    if (!this.storagePath) return;
    try {
      const dir = path.dirname(this.storagePath);
      fs.mkdirSync(dir, { recursive: true });
      const obj: Record<string, ApprovalTrackingEntry> = {};
      for (const [approvalId, entry] of this.sessionMap.entries()) {
        obj[String(approvalId)] = entry;
      }
      fs.writeFileSync(this.storagePath, JSON.stringify(obj, null, 2));
    } catch (err) {
      this.logger?.error?.(`[teenet-wallet] persistMap.error`, { error: String(err) });
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
