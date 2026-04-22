// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

import { describe, it, beforeEach } from "node:test";
import assert from "node:assert/strict";
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

});
