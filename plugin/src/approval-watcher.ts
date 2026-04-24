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
  chain?: string;
}

// Explorer base URLs per chain. Mirrored from SKILL.md — the authoritative
// list. Keep in sync if chains.json gains or loses an entry.
const EXPLORER_BASE: Record<string, string> = {
  ethereum: "https://etherscan.io",
  optimism: "https://optimistic.etherscan.io",
  arbitrum: "https://arbiscan.io",
  base: "https://basescan.org",
  polygon: "https://polygonscan.com",
  bsc: "https://bscscan.com",
  avalanche: "https://snowtrace.io",
  sepolia: "https://sepolia.etherscan.io",
  "optimism-sepolia": "https://sepolia-optimism.etherscan.io",
  "arbitrum-sepolia": "https://sepolia.arbiscan.io",
  "base-sepolia": "https://sepolia.basescan.org",
  "polygon-amoy": "https://amoy.polygonscan.com",
  "bsc-testnet": "https://testnet.bscscan.com",
  "avalanche-fuji": "https://testnet.snowtrace.io",
  solana: "https://solscan.io",
  "solana-devnet": "https://solscan.io",
};

function explorerTxUrl(chain: string | undefined, txHash: string): string | null {
  if (!chain) return null;
  const base = EXPLORER_BASE[chain];
  if (!base) return null;
  const suffix = chain === "solana-devnet" ? "?cluster=devnet" : "";
  return `${base}/tx/${txHash}${suffix}`;
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

type NotifiedEntry = {
  notifiedAt: number;
};

// Persisted on disk as `{tracked, notified}`. `tracked` is the set of
// approvals we're waiting to resolve; `notified` is a tombstone set so
// that the same id is never re-notified — even across process restarts,
// gateway restarts, or a stale disk entry being reconciled later.
type PersistedState = {
  tracked: Record<string, ApprovalTrackingEntry>;
  notified: Record<string, NotifiedEntry>;
};

// Retention windows.
const TRACKED_TTL_MS = 24 * 60 * 60 * 1000;        // 1 day: unresolved tracking
const NOTIFIED_TTL_MS = 30 * 24 * 60 * 60 * 1000;  // 30 days: dedup tombstones

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

  // Tombstone set of approval ids we've already notified about. Both live
  // SSE and reconcile-on-reconnect can deliver the same approval_id more
  // than once (SSE buffered replay, reconnect race, stale disk entry that
  // reconcile hits again after a restart); the watcher must not re-fire
  // subagentRun for any of them. Persisted to disk alongside `sessionMap`
  // so the guarantee survives gateway restarts; entries age out after
  // NOTIFIED_TTL_MS to keep the file bounded.
  private notified: Map<number, NotifiedEntry> = new Map();

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

    // Periodic cleanup of both halves of the persisted state:
    //   - stale tracked entries (approvals still marked pending after 24h
    //     — either expired on the backend or orphaned by a bug)
    //   - stale notified tombstones (older than NOTIFIED_TTL_MS). Letting
    //     these accumulate indefinitely would grow the JSON without bound.
    this.cleanupTimer = setInterval(() => {
      const now = Date.now();
      const trackedCutoff = now - TRACKED_TTL_MS;
      const notifiedCutoff = now - NOTIFIED_TTL_MS;
      let changed = false;
      for (const [id, entry] of this.sessionMap) {
        if (entry.createdAt < trackedCutoff) {
          this.log("cleanup.stale-tracked", { approvalId: id });
          this.sessionMap.delete(id);
          changed = true;
        }
      }
      for (const [id, entry] of this.notified) {
        if (entry.notifiedAt < notifiedCutoff) {
          this.notified.delete(id);
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
    // Layer 1 (process-local dedup). If we've already notified for this
    // approval in this process, do nothing. Prevents the "reconcile burst"
    // bug where reconnect fires reconcile, which polls /api/approvals/:id
    // and re-invokes onApprovalResolved for ids we already handled.
    if (this.notified.has(event.approval_id)) {
      this.log("notify.duplicate", { approvalId: event.approval_id });
      return;
    }

    // Claim + resolve the route in one step, using disk as the source of
    // truth when instances race. This avoids the gap where one instance has
    // already removed tracked[id] from disk but another instance has not yet
    // materialized notified[id] in its own memory.
    const claimed = this.claimApprovalForNotification(event.approval_id);
    if (claimed.alreadyNotified) {
      this.log("notify.duplicate", { approvalId: event.approval_id, source: claimed.source });
      return;
    }
    const tracked = claimed.tracked;
    if (!tracked) {
      this.log("notify.missing-track", {
        approvalId: event.approval_id,
        status: event.status,
        mapSize: this.sessionMap.size,
        source: claimed.source,
      });
      return;
    }
    if (claimed.source === "disk") {
      this.log("notify.recovered-from-disk", {
        approvalId: event.approval_id,
        sessionKey: tracked.sessionKey,
      });
    }

    const { sessionKey, context } = tracked;
    const message = formatApprovalMessage(event);
    const fullMessage = `System: ${message}`;

    if (!this.subagentRun) {
      this.log("notify.no-subagent", { approvalId: event.approval_id, sessionKey });
      return;
    }

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
  }

  private claimApprovalForNotification(approvalId: number): {
    tracked: ApprovalTrackingEntry | null;
    source: "memory" | "disk" | "none";
    alreadyNotified: boolean;
  } {
    // Fast path: current process already owns the route.
    const memoryTracked = this.sessionMap.get(approvalId);
    if (memoryTracked) {
      this.notified.set(approvalId, { notifiedAt: Date.now() });
      this.sessionMap.delete(approvalId);
      this.persistMap();
      return { tracked: memoryTracked, source: "memory", alreadyNotified: false };
    }

    if (!this.storagePath) {
      return { tracked: null, source: "none", alreadyNotified: false };
    }

    try {
      if (!fs.existsSync(this.storagePath)) {
        return { tracked: null, source: "none", alreadyNotified: false };
      }
      const raw = fs.readFileSync(this.storagePath, "utf8");
      const parsed = JSON.parse(raw) as unknown;
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
        return { tracked: null, source: "none", alreadyNotified: false };
      }

      const obj = parsed as Record<string, unknown>;
      const trackedSource = ((obj.tracked as Record<string, ApprovalTrackingEntry>) || {}) as Record<string, ApprovalTrackingEntry>;
      const notifiedSource = ((obj.notified as Record<string, NotifiedEntry>) || {}) as Record<string, NotifiedEntry>;

      const notified = notifiedSource[String(approvalId)];
      if (typeof notified?.notifiedAt === "number" && notified.notifiedAt >= Date.now() - NOTIFIED_TTL_MS) {
        this.notified.set(approvalId, { notifiedAt: notified.notifiedAt });
        return { tracked: null, source: "disk", alreadyNotified: true };
      }

      const entry = trackedSource[String(approvalId)];
      if (!entry?.sessionKey) {
        return { tracked: null, source: "none", alreadyNotified: false };
      }
      if (typeof entry.createdAt !== "number" || entry.createdAt < Date.now() - TRACKED_TTL_MS) {
        return { tracked: null, source: "none", alreadyNotified: false };
      }

      // Atomically rewrite persisted state for this id: tracked -> notified.
      delete trackedSource[String(approvalId)];
      const now = Date.now();
      notifiedSource[String(approvalId)] = { notifiedAt: now };
      const state: PersistedState = { tracked: trackedSource, notified: notifiedSource };
      const tmp = `${this.storagePath}.tmp-${process.pid}-${now}`;
      fs.writeFileSync(tmp, JSON.stringify(state, null, 2));
      fs.renameSync(tmp, this.storagePath);

      this.notified.set(approvalId, { notifiedAt: now });
      this.sessionMap.delete(approvalId);
      return { tracked: entry, source: "disk", alreadyNotified: false };
    } catch (err) {
      this.logger?.error?.(`[teenet-wallet] claimApprovalForNotification.error`, { approvalId, error: String(err) });
      return { tracked: null, source: "none", alreadyNotified: false };
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
        const status = (approval?.status ?? approval?.approval?.status) as string;
        if (status && status !== "pending") {
          this.log("reconcile.found", { approvalId, status });
          this.onApprovalResolved({
            approval_id: approvalId,
            status,
            approval_type: approval.approval_type ?? approval.approval?.approval_type ?? "unknown",
            tx_hash: approval.tx_hash ?? approval.approval?.tx_hash,
            wallet_id: approval.wallet_id ?? approval.approval?.wallet_id,
            chain: approval.chain,
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

  // loadPersistedMap is meant to be called exactly once, from setStoragePath()
  // at construction. Calling it later (e.g. from onApprovalResolved as a
  // "not-in-memory fallback") has historically caused ghost notifications:
  // it silently merged stale disk entries back into the in-memory map,
  // which then survived to the next reconnect's reconcile pass and fired
  // as duplicate notifications. The replace-not-merge semantics here make
  // that failure mode explicit if it ever happens again.
  //
  // Persisted shape: `{tracked: {...}, notified: {...}}`. The legacy flat
  // shape (where the entire object was a tracked map) is still accepted
  // on read for backward compat — older files will be transparently
  // upgraded on the next write.
  private loadPersistedMap(): void {
    if (!this.storagePath) return;
    try {
      if (!fs.existsSync(this.storagePath)) return;
      const raw = fs.readFileSync(this.storagePath, "utf8");
      const parsed = JSON.parse(raw) as unknown;

      let trackedSource: Record<string, ApprovalTrackingEntry> = {};
      let notifiedSource: Record<string, NotifiedEntry> = {};
      if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
        const obj = parsed as Record<string, unknown>;
        if ("tracked" in obj || "notified" in obj) {
          trackedSource = (obj.tracked as Record<string, ApprovalTrackingEntry>) || {};
          notifiedSource = (obj.notified as Record<string, NotifiedEntry>) || {};
        } else {
          // Legacy flat shape: whole object is tracked.
          trackedSource = obj as Record<string, ApprovalTrackingEntry>;
        }
      }

      const now = Date.now();
      const trackedCutoff = now - TRACKED_TTL_MS;
      const nextTracked = new Map<number, ApprovalTrackingEntry>();
      for (const [approvalId, entry] of Object.entries(trackedSource)) {
        const id = Number(approvalId);
        if (!Number.isFinite(id) || !entry?.sessionKey) continue;
        if (entry.createdAt < trackedCutoff) continue;
        nextTracked.set(id, entry);
      }
      this.sessionMap = nextTracked;

      const notifiedCutoff = now - NOTIFIED_TTL_MS;
      const nextNotified = new Map<number, NotifiedEntry>();
      for (const [approvalId, entry] of Object.entries(notifiedSource)) {
        const id = Number(approvalId);
        if (!Number.isFinite(id)) continue;
        const ts = typeof entry?.notifiedAt === "number" ? entry.notifiedAt : 0;
        if (ts < notifiedCutoff) continue;
        nextNotified.set(id, { notifiedAt: ts });
      }
      this.notified = nextNotified;

      if (nextTracked.size > 0 || nextNotified.size > 0) {
        this.log("persist.load", { tracked: nextTracked.size, notified: nextNotified.size });
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
      const tracked: Record<string, ApprovalTrackingEntry> = {};
      for (const [approvalId, entry] of this.sessionMap.entries()) {
        tracked[String(approvalId)] = entry;
      }
      const notified: Record<string, NotifiedEntry> = {};
      for (const [approvalId, entry] of this.notified.entries()) {
        notified[String(approvalId)] = entry;
      }
      const state: PersistedState = { tracked, notified };
      // Atomic write: stage to a unique temp file, then rename into place.
      // POSIX rename within one filesystem is atomic, so concurrent readers
      // always see either the old content or the full new content — never
      // a truncated/torn file that a `writeFileSync` could leave if the
      // process is killed mid-write.
      const tmp = `${this.storagePath}.tmp-${process.pid}-${Date.now()}`;
      fs.writeFileSync(tmp, JSON.stringify(state, null, 2));
      fs.renameSync(tmp, this.storagePath);
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
        const url = explorerTxUrl(event.chain, event.tx_hash);
        if (url) {
          return `Approval #${id} approved. Transaction: ${event.tx_hash} — ${url}`;
        }
        // Chain missing or unrecognised — fall back to asking the agent to
        // construct the link from context.
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
