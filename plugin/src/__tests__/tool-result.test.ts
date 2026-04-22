// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { jsonResult, approvalOrResult } from "../tools/tool-result.js";

describe("jsonResult", () => {
  it("wraps data as MCP tool result", () => {
    const result = jsonResult({ foo: "bar" });
    assert.equal(result.content.length, 1);
    assert.equal(result.content[0].type, "text");
    const parsed = JSON.parse(result.content[0].text);
    assert.deepEqual(parsed, { foo: "bar" });
  });
});

describe("approvalOrResult", () => {
  const getUrl = (id: number) => `https://example.com/approve/${id}`;

  it("returns approval info when status is pending_approval", () => {
    const result = approvalOrResult(
      { status: "pending_approval", approval_id: 42, extra: "data" },
      getUrl,
    );
    const parsed = JSON.parse(result.content[0].text);
    assert.equal(parsed.status, "pending_approval");
    assert.equal(parsed.approval_id, 42);
    assert.equal(parsed.approval_url, "https://example.com/approve/42");
    assert.ok(parsed.message.includes("Approval required"));
  });

  it("returns raw result when status is not pending_approval", () => {
    const input = { status: "completed", tx_hash: "0xabc" };
    const result = approvalOrResult(input, getUrl);
    const parsed = JSON.parse(result.content[0].text);
    assert.deepEqual(parsed, input);
  });

  it("forwards chain and tx_hash on completed transfer (bug A regression)", () => {
    // Backend returns chain alongside tx_hash; agent needs both to build
    // the explorer URL from the SKILL.md table. The plugin must not strip them.
    const input = {
      status: "completed",
      tx_hash: "0xdeadbeef",
      chain: "sepolia",
      from: "0xabc",
      to: "0xdef",
      amount: "0.001",
      currency: "ETH",
    };
    const result = approvalOrResult(input, getUrl);
    const parsed = JSON.parse(result.content[0].text);
    assert.equal(parsed.chain, "sepolia");
    assert.equal(parsed.tx_hash, "0xdeadbeef");
    assert.equal(parsed.status, "completed");
  });

  it("returns raw result when approval_id is missing", () => {
    const input = { status: "pending_approval" }; // no approval_id
    const result = approvalOrResult(input, getUrl);
    const parsed = JSON.parse(result.content[0].text);
    assert.equal(parsed.status, "pending_approval");
    assert.equal(parsed.approval_url, undefined); // not wrapped
  });
});
