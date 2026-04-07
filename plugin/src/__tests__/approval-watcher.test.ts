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
