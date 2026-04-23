// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

import { describe, it, beforeEach } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import os from "node:os";
import { ApprovalWatcher, type SubagentRun, type ApprovalEvent } from "../approval-watcher.js";
import type { WalletAPI } from "../api-client.js";

// Minimal mock of WalletAPI — only needs eventsUrl and authHeader.
function mockApi(): WalletAPI {
  return {
    eventsUrl: "http://localhost:9999/api/events/stream",
    authHeader: "Bearer test_key",
  } as unknown as WalletAPI;
}

// Mock SubagentRun that records calls.
function mockSubagentRun(): SubagentRun & { calls: Array<{ sessionKey: string; message: string; deliver?: boolean }> } {
  const calls: Array<{ sessionKey: string; message: string; deliver?: boolean }> = [];
  const fn = async (opts: { sessionKey: string; message: string; deliver?: boolean }) => {
    calls.push(opts);
    return { runId: "mock-run-id" };
  };
  fn.calls = calls;
  return fn;
}

describe("ApprovalWatcher", () => {
  it("can be created and stopped without starting", () => {
    const watcher = new ApprovalWatcher(mockApi());
    assert.equal(watcher.isConnected, false);
    watcher.stop(); // should not throw
  });

  it("notifies via subagent on approval resolved", async () => {
    const watcher = new ApprovalWatcher(mockApi());
    const subagent = mockSubagentRun();
    watcher.setSubagentRun(subagent);

    watcher.trackApproval(7, "agent:main:telegram:direct:12345");

    (watcher as any).onApprovalResolved({
      approval_id: 7,
      status: "rejected",
      approval_type: "policy_change",
    });

    // Give the async subagent call time to execute.
    await new Promise((r) => setTimeout(r, 50));

    assert.equal(subagent.calls.length, 1);
    assert.equal(subagent.calls[0].sessionKey, "agent:main:telegram:direct:12345");
    assert.equal(subagent.calls[0].deliver, true);
    assert.ok(subagent.calls[0].message.includes("rejected"));
  });

  it("does not notify without subagent or tracked session", async () => {
    const watcher = new ApprovalWatcher(mockApi());
    // No setSubagentRun, no trackApproval — should not throw.
    (watcher as any).onApprovalResolved({
      approval_id: 1,
      status: "approved",
      approval_type: "transfer",
    });
  });

  it("handles SSE message parsing", () => {
    const watcher = new ApprovalWatcher(mockApi());

    const events: ApprovalEvent[] = [];
    const origResolve = (watcher as any).onApprovalResolved.bind(watcher);
    (watcher as any).onApprovalResolved = (event: ApprovalEvent) => {
      events.push(event);
      return origResolve(event);
    };

    (watcher as any).handleSSEMessage(
      'event: approval_resolved\ndata: {"approval_id":5,"status":"approved","approval_type":"transfer","tx_hash":"0xabc"}'
    );

    assert.equal(events.length, 1);
    assert.equal(events[0].approval_id, 5);
    assert.equal(events[0].tx_hash, "0xabc");
  });

  it("embeds explorer URL in agent message when chain is present", async () => {
    const watcher = new ApprovalWatcher(mockApi());
    const subagent = mockSubagentRun();
    watcher.setSubagentRun(subagent);

    watcher.trackApproval(8, "agent:main:telegram:direct:42");

    (watcher as any).onApprovalResolved({
      approval_id: 8,
      status: "approved",
      approval_type: "transfer",
      tx_hash: "0xe24998fe",
      chain: "sepolia",
    });

    await new Promise((r) => setTimeout(r, 50));

    assert.equal(subagent.calls.length, 1);
    const msg = subagent.calls[0].message;
    assert.ok(
      msg.includes("https://sepolia.etherscan.io/tx/0xe24998fe"),
      `expected explorer URL in message, got: ${msg}`,
    );
  });

  it("falls back to legacy phrasing when chain is missing or unknown", async () => {
    const watcher = new ApprovalWatcher(mockApi());
    const subagent = mockSubagentRun();
    watcher.setSubagentRun(subagent);

    watcher.trackApproval(9, "agent:main:telegram:direct:42");
    watcher.trackApproval(10, "agent:main:telegram:direct:42");

    // No chain at all — legacy behaviour.
    (watcher as any).onApprovalResolved({
      approval_id: 9,
      status: "approved",
      approval_type: "transfer",
      tx_hash: "0xabc",
    });
    // Unknown chain — same fallback.
    (watcher as any).onApprovalResolved({
      approval_id: 10,
      status: "approved",
      approval_type: "transfer",
      tx_hash: "0xdef",
      chain: "unknown-chain",
    });

    await new Promise((r) => setTimeout(r, 50));

    assert.equal(subagent.calls.length, 2);
    for (const call of subagent.calls) {
      assert.ok(
        call.message.includes("Please share the explorer link"),
        `expected fallback phrasing, got: ${call.message}`,
      );
    }
  });

  it("reconcile path forwards chain into agent message (bug B regression)", async () => {
    // When SSE was disconnected and a tracked approval resolved in the meantime,
    // reconcileTrackedApprovals polls GET /api/approvals/:id to recover. It must
    // forward the chain field so the explorer URL gets built.
    const mockGetApproval = async (_id: number) => ({
      success: true,
      status: "approved",
      approval_type: "transfer",
      tx_hash: "0xrecon",
      wallet_id: "w-1",
      chain: "sepolia",
      approval: {
        id: 11,
        status: "approved",
        approval_type: "transfer",
        tx_hash: "0xrecon",
        wallet_id: "w-1",
        created_at: "",
        expires_at: "",
      },
    });
    const api = {
      ...mockApi(),
      getApproval: mockGetApproval,
    } as unknown as WalletAPI;

    const watcher = new ApprovalWatcher(api);
    const subagent = mockSubagentRun();
    watcher.setSubagentRun(subagent);

    watcher.trackApproval(11, "agent:main:x:42");
    await (watcher as any).reconcileTrackedApprovals();
    await new Promise((r) => setTimeout(r, 50));

    assert.equal(subagent.calls.length, 1);
    assert.ok(
      subagent.calls[0].message.includes("https://sepolia.etherscan.io/tx/0xrecon"),
      `expected explorer URL in reconciled message, got: ${subagent.calls[0].message}`,
    );
  });

  it("reconcile falls back to nested approval.status when top-level is missing", async () => {
    // Backwards-compat: older backend responses nest everything under
    // "approval". Reconcile should still fire.
    const mockGetApproval = async (_id: number) => ({
      success: true,
      // no top-level status / approval_type / tx_hash / chain
      approval: {
        id: 12,
        status: "approved",
        approval_type: "transfer",
        tx_hash: "0xlegacy",
        wallet_id: "w-2",
        created_at: "",
        expires_at: "",
      },
    });
    const api = {
      ...mockApi(),
      getApproval: mockGetApproval,
    } as unknown as WalletAPI;

    const watcher = new ApprovalWatcher(api);
    const subagent = mockSubagentRun();
    watcher.setSubagentRun(subagent);

    watcher.trackApproval(12, "agent:main:x:42");
    await (watcher as any).reconcileTrackedApprovals();
    await new Promise((r) => setTimeout(r, 50));

    assert.equal(subagent.calls.length, 1);
    // No chain available — should use fallback phrasing, not error out.
    assert.ok(
      subagent.calls[0].message.includes("0xlegacy"),
      `expected tx hash in fallback message, got: ${subagent.calls[0].message}`,
    );
  });

  it("ignores non-approval SSE events", () => {
    const watcher = new ApprovalWatcher(mockApi());
    const subagent = mockSubagentRun();
    watcher.setSubagentRun(subagent);

    (watcher as any).handleSSEMessage('event: heartbeat\ndata: {}');

    assert.equal(subagent.calls.length, 0);
  });

  it("ignores malformed SSE data", () => {
    const watcher = new ApprovalWatcher(mockApi());
    const subagent = mockSubagentRun();
    watcher.setSubagentRun(subagent);

    (watcher as any).handleSSEMessage('event: approval_resolved\ndata: {not json}');

    assert.equal(subagent.calls.length, 0);
  });

  // ─── Regression: duplicate-delivery guard (bug "reconcile burst") ──────────

  it("fires subagent at most once per approval id within one process", async () => {
    // The original bug: SSE delivered `approval_resolved` once → watcher
    // notified. Reconnect fired reconcileTrackedApprovals, which polled
    // the same id via REST, got back "approved", and called
    // onApprovalResolved again → second notification. Both the live path
    // and the reconcile path now hit the in-memory `notified` guard.
    const watcher = new ApprovalWatcher(mockApi());
    const subagent = mockSubagentRun();
    watcher.setSubagentRun(subagent);

    watcher.trackApproval(100, "agent:main:telegram:direct:42");

    const event: ApprovalEvent = {
      approval_id: 100,
      status: "approved",
      approval_type: "transfer",
      tx_hash: "0xfeedface",
      chain: "sepolia",
    };

    // First delivery (live SSE).
    (watcher as any).onApprovalResolved(event);
    // Second delivery (reconcile after reconnect).
    (watcher as any).onApprovalResolved(event);
    // Third delivery (hypothetical dual-watcher race).
    (watcher as any).onApprovalResolved(event);

    await new Promise((r) => setTimeout(r, 50));

    assert.equal(
      subagent.calls.length,
      1,
      `expected exactly one notification across three deliveries, got ${subagent.calls.length}`,
    );
  });

  it("does not resurrect stale disk entries on missed-event lookup", async () => {
    // Old behaviour: onApprovalResolved for an unknown id triggered a full
    // disk reload, which silently restored stale entries back into the
    // in-memory map. Those then survived to the next reconcile pass and
    // fired as ghost notifications. New behaviour: unknown id → drop.
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "watcher-test-"));
    const storagePath = path.join(dir, "approvals.json");
    // Seed disk with a "stale" entry — NOT tracked via trackApproval.
    fs.writeFileSync(
      storagePath,
      JSON.stringify({
        "999": {
          sessionKey: "old:session",
          context: "stale",
          createdAt: Date.now(),
        },
      }),
    );

    const watcher = new ApprovalWatcher(mockApi());
    const subagent = mockSubagentRun();
    watcher.setSubagentRun(subagent);
    // setStoragePath loads disk exactly once at construction.
    watcher.setStoragePath(storagePath);

    // Fire a completely unrelated unknown id. Old code would reload disk
    // here, find nothing for 500 but leak id 999 into memory as a
    // side-effect. New code logs missing-track and moves on.
    (watcher as any).onApprovalResolved({
      approval_id: 500,
      status: "approved",
      approval_type: "transfer",
    });

    // 999 must NOT have been re-restored into the sessionMap as a side
    // effect. It was in the map at startup (legit), and since nothing
    // touched it, it's still there — but onApprovalResolved for 500
    // should not have CHANGED anything about 999.
    const internal = watcher as unknown as { sessionMap: Map<number, unknown> };
    assert.equal(internal.sessionMap.size, 1, "only the legitimate startup-loaded entry should remain");
    assert.ok(internal.sessionMap.has(999), "startup entry should still be present");

    // Now resolve 999. That SHOULD notify once.
    (watcher as any).onApprovalResolved({
      approval_id: 999,
      status: "approved",
      approval_type: "transfer",
    });
    await new Promise((r) => setTimeout(r, 50));

    assert.equal(subagent.calls.length, 1);
    // Disk must reflect the deletion from tracked, AND record 999 as
    // notified so future reconciles won't re-fire the notification.
    const afterDisk = JSON.parse(fs.readFileSync(storagePath, "utf8"));
    assert.deepEqual(afterDisk.tracked, {}, `tracked should be empty after resolution, got ${JSON.stringify(afterDisk)}`);
    assert.ok(afterDisk.notified?.["999"]?.notifiedAt, `999 should be tombstoned in notified, got ${JSON.stringify(afterDisk)}`);

    fs.rmSync(dir, { recursive: true, force: true });
  });

  it("replace-not-merge: loadPersistedMap drops in-memory ids not on disk", () => {
    // Before the fix, loadPersistedMap merged disk entries into the
    // existing sessionMap, so a disk read after in-memory deletes could
    // never shrink the map. After the fix, sessionMap reflects exactly
    // what's on disk at load time.
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "watcher-test-"));
    const storagePath = path.join(dir, "approvals.json");
    fs.writeFileSync(storagePath, JSON.stringify({
      "1": { sessionKey: "s", createdAt: Date.now() },
      "2": { sessionKey: "s", createdAt: Date.now() },
    }));

    const watcher = new ApprovalWatcher(mockApi());
    watcher.setStoragePath(storagePath); // first load: map = {1, 2}
    const internal = watcher as unknown as { sessionMap: Map<number, unknown>; loadPersistedMap: () => void };
    assert.equal(internal.sessionMap.size, 2);

    // Shrink the disk to just {3} and reload. After the fix, in-memory
    // 1 and 2 must be gone, and 3 must be present.
    fs.writeFileSync(storagePath, JSON.stringify({
      "3": { sessionKey: "s", createdAt: Date.now() },
    }));
    internal.loadPersistedMap();

    assert.equal(internal.sessionMap.size, 1);
    assert.ok(internal.sessionMap.has(3));
    assert.ok(!internal.sessionMap.has(1), "stale id 1 must be dropped");
    assert.ok(!internal.sessionMap.has(2), "stale id 2 must be dropped");

    fs.rmSync(dir, { recursive: true, force: true });
  });

  it("persistMap writes atomically (no stray tmp files on success)", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "watcher-test-"));
    const storagePath = path.join(dir, "approvals.json");

    const watcher = new ApprovalWatcher(mockApi());
    watcher.setStoragePath(storagePath);
    watcher.trackApproval(42, "sess-a");
    watcher.trackApproval(43, "sess-b");

    // After several writes, the directory should contain only the final
    // file — no leftover `.tmp-<pid>-<ts>` stragglers.
    const entries = fs.readdirSync(dir);
    assert.deepEqual(entries, ["approvals.json"], `unexpected dir contents: ${entries.join(",")}`);

    const parsed = JSON.parse(fs.readFileSync(storagePath, "utf8"));
    assert.deepEqual(Object.keys(parsed.tracked).sort(), ["42", "43"]);
    assert.deepEqual(parsed.notified, {}, "no approvals have been resolved yet");

    fs.rmSync(dir, { recursive: true, force: true });
  });

  // ─── Regression: notified set persists across watcher instances ─────────────

  it("does not re-notify an approval whose tombstone is on disk", async () => {
    // Simulates the gateway-restart scenario: a tracked approval was
    // resolved and notified by the previous process, and its tombstone
    // landed in `notified` on disk. A fresh watcher must see that
    // tombstone on startup and silently drop any further delivery of
    // the same id (whether via SSE replay or reconcile poll). Without
    // this, every gateway restart would re-notify every resolved
    // approval still on disk — the exact bug the user flagged.
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "watcher-test-"));
    const storagePath = path.join(dir, "approvals.json");
    fs.writeFileSync(storagePath, JSON.stringify({
      tracked: {},
      notified: {
        "12345": { notifiedAt: Date.now() - 60_000 },
      },
    }));

    const watcher = new ApprovalWatcher(mockApi());
    const subagent = mockSubagentRun();
    watcher.setSubagentRun(subagent);
    watcher.setStoragePath(storagePath);

    // Simulate reconcile re-delivering a previously-notified id — before
    // the fix, this would track + notify again. After the fix, the
    // tombstone guard short-circuits.
    (watcher as any).onApprovalResolved({
      approval_id: 12345,
      status: "approved",
      approval_type: "transfer",
      tx_hash: "0xghost",
    });
    await new Promise((r) => setTimeout(r, 50));

    assert.equal(subagent.calls.length, 0, "tombstoned id must not notify");

    fs.rmSync(dir, { recursive: true, force: true });
  });

  it("accepts legacy flat disk shape for backward compat", () => {
    // Pre-fix files were a flat map of tracked entries. On first read,
    // those must still load into sessionMap without error; on the next
    // write the file is transparently upgraded to {tracked, notified}.
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "watcher-test-"));
    const storagePath = path.join(dir, "approvals.json");
    fs.writeFileSync(storagePath, JSON.stringify({
      "7": { sessionKey: "s", createdAt: Date.now() },
      "8": { sessionKey: "s", createdAt: Date.now() },
    }));

    const watcher = new ApprovalWatcher(mockApi());
    watcher.setStoragePath(storagePath);
    const internal = watcher as unknown as {
      sessionMap: Map<number, unknown>;
      notified: Map<number, unknown>;
    };
    assert.equal(internal.sessionMap.size, 2, "legacy tracked entries should load");
    assert.equal(internal.notified.size, 0, "legacy file has no notified section");

    // Any write should upgrade the on-disk shape.
    watcher.trackApproval(9, "s");
    const upgraded = JSON.parse(fs.readFileSync(storagePath, "utf8"));
    assert.ok(upgraded.tracked, "file should be upgraded to new shape");
    assert.ok("notified" in upgraded, "notified section should be written even if empty");

    fs.rmSync(dir, { recursive: true, force: true });
  });

});
