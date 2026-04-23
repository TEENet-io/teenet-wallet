// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { WalletAPI } from "../api-client.js";
import { ApprovalWatcher } from "../approval-watcher.js";

const API_URL = process.env.TEENET_TEST_API_URL || "https://wallet.teenet.app";
const API_KEY = process.env.TEENET_TEST_API_KEY || "ocw_test_placeholder";
const api = new WalletAPI({ apiUrl: API_URL, apiKey: API_KEY });

// Reuse existing wallets — no DKG on every run.
const ETH_WALLET = "1b9814d1-d28a-4629-8365-8e11eab394f9";
const ETH_ADDR = "0xd7e27B678206c0EE45C2678A88923c8a6F343C21";
const SOL_WALLET = "b742b69b-e81e-4593-9195-a2b48a5f9209";
const MAX_WALLETS = 10;

// ─── Basic API ──────────────────────────────────────────────────────────────

describe("Basic API", () => {
  it("health", async () => {
    const h = await api.health();
    assert.equal(h.status, "ok");
  });

  it("chains", async () => {
    const chains = await api.listChains();
    const names = chains.map((c) => c.name);
    assert.ok(names.includes("sepolia"));
    assert.ok(names.includes("solana-devnet"));
  });

  it("prices", async () => {
    const p = await api.getPrices();
    assert.ok(typeof p === "object");
  });

  it("audit logs", async () => {
    const r = await api.auditLogs(1, 5);
    assert.ok(r.logs !== undefined);
  });
});

// ─── Wallet CRUD ────────────────────────────────────────────────────────────

describe("Wallet CRUD", () => {
  it("list wallets", async () => {
    const wallets = await api.listWallets();
    assert.ok(wallets.length > 0);
    assert.ok(wallets.some((w) => w.id === ETH_WALLET));
  });

  it("get wallet", async () => {
    const w = await api.getWallet(ETH_WALLET);
    assert.equal(w.id, ETH_WALLET);
    assert.equal(w.chain, "sepolia");
    assert.ok(w.public_key);
  });

  it("rename wallet", async () => {
    const label = `E2E ${Date.now()}`;
    await api.renameWallet(ETH_WALLET, label);
    const w = await api.getWallet(ETH_WALLET);
    assert.equal(w.label, label);
  });

  it("get pubkey", async () => {
    const r = await api.getPubkey(ETH_WALLET);
    assert.ok(r.public_key);
  });

  it("create wallet (best-effort)", async () => {
    try {
      const w = await api.createWallet("sepolia", `New ${Date.now()}`);
      assert.ok(w.id);
      assert.equal(w.chain, "sepolia");
      console.log(`    Created: ${w.id} (${w.status})`);
    } catch (err: any) {
      // May fail due to wallet limit or DB constraints
      console.log(`    Create skipped: ${err.message}`);
    }
  });
});

// ─── Solana ─────────────────────────────────────────────────────────────────

describe("Solana", () => {
  it("get solana wallet", async () => {
    const w = await api.getWallet(SOL_WALLET);
    assert.equal(w.curve, "ed25519");
    assert.equal(w.protocol, "eddsa");
  });

  it("solana balance", async () => {
    const b = await api.getBalance(SOL_WALLET);
    assert.equal(b.currency, "SOL");
  });

  it("create solana wallet (best-effort)", async () => {
    try {
      const w = await api.createWallet("solana-devnet", `SOL ${Date.now()}`);
      assert.equal(w.status, "ready");
      assert.equal(w.curve, "ed25519");
      console.log(`    Created: ${w.address}`);
    } catch (err: any) {
      console.log(`    Create skipped: ${err.message}`);
    }
  });
});

// ─── Balance ────────────────────────────────────────────────────────────────

describe("Balance", () => {
  it("ETH balance", async () => {
    const b = await api.getBalance(ETH_WALLET);
    assert.ok(parseFloat(b.balance) >= 0);
    assert.equal(b.currency, "ETH");
    console.log(`    ${b.balance} ETH`);
  });
});

// ─── Faucet (reuse existing wallet, tolerate rate limit) ────────────────────

describe("Faucet", () => {
  it("claim faucet", async () => {
    try {
      const r = await api.claimFaucet(ETH_WALLET);
      assert.ok(r.success);
      assert.ok(r.tx_hash);
      console.log(`    tx: ${r.tx_hash}`);
    } catch (err: any) {
      assert.ok(err.message.includes("already claimed"), `unexpected: ${err.message}`);
      console.log(`    Rate-limited (expected): ${err.message}`);
    }
  });
});

// ─── Transfer ───────────────────────────────────────────────────────────────

describe("Transfer", () => {
  it("transfer 0.0001 ETH to self", async () => {
    const r = await api.transfer(ETH_WALLET, ETH_ADDR, "0.0001", undefined, "E2E");
    assert.ok(r.status === "completed" || r.status === "pending_approval");
    if (r.status === "completed") assert.ok(r.tx_hash);
    console.log(`    status=${r.status} tx=${r.tx_hash || "pending"}`);
  });
});

// ─── Policy ─────────────────────────────────────────────────────────────────

describe("Policy", () => {
  it("get policy", async () => {
    const r = await api.getPolicy(ETH_WALLET);
    assert.ok(r !== undefined);
  });

  it("set policy → pending_approval", async () => {
    const r = await api.setPolicy(ETH_WALLET, "10", true, "1000");
    assert.ok(r.approval_id);
    console.log(`    approval_id=${r.approval_id}`);
  });

  it("daily spent", async () => {
    const r = await api.getDailySpent(ETH_WALLET);
    assert.ok(r !== undefined);
  });
});

// ─── Contracts ──────────────────────────────────────────────────────────────

describe("Contracts", () => {
  it("list contracts", async () => {
    const c = await api.listContracts(ETH_WALLET);
    assert.ok(Array.isArray(c));
  });

  it("add contract → pending_approval", async () => {
    const r = await api.addContract(ETH_WALLET, "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238", "USDC", 6);
    assert.ok(r.approval_id);
    console.log(`    approval_id=${r.approval_id}`);
  });
});

// ─── Address Book ───────────────────────────────────────────────────────────

describe("Address Book", () => {
  it("list", async () => {
    const e = await api.listAddressBook();
    assert.ok(Array.isArray(e));
  });

  it("add → pending_approval", async () => {
    const r = await api.addAddressBookEntry(`t${Date.now()}`, "sepolia", "0x0000000000000000000000000000000000000001");
    assert.ok(r.approval_id);
  });

  it("filter by chain", async () => {
    const e = await api.listAddressBook(undefined, "sepolia");
    assert.ok(Array.isArray(e));
  });
});

// ─── Approvals ──────────────────────────────────────────────────────────────

describe("Approvals", () => {
  it("list pending", async () => {
    const a = await api.listPendingApprovals();
    assert.ok(Array.isArray(a));
    console.log(`    ${a.length} pending`);
  });

  it("get specific approval", async () => {
    const policy = await api.setPolicy(ETH_WALLET, "5", true);
    if (policy.approval_id) {
      const a = await api.getApproval(policy.approval_id);
      assert.ok(a);
    }
  });
});

// ─── SSE ────────────────────────────────────────────────────────────────────

describe("SSE", () => {
  // IMPORTANT: every test here MUST clean up its AbortController / ApprovalWatcher
  // in a `finally` block. On assertion failure (e.g. invalid API_KEY returning
  // 401 instead of 200), skipping cleanup leaks the pending reconnect setTimeout
  // inside ApprovalWatcher and keeps the node test runner process alive
  // indefinitely — retrying the SSE connect every ~60s forever.

  it("connect and receive connected message", async () => {
    const ctrl = new AbortController();
    try {
      const res = await fetch(`${API_URL}/api/events/stream`, {
        headers: { Authorization: `Bearer ${API_KEY}` },
        signal: ctrl.signal,
      });
      assert.equal(res.status, 200);
      assert.ok(res.headers.get("content-type")?.includes("text/event-stream"));
      const reader = res.body!.getReader();
      const { value } = await reader.read();
      assert.ok(new TextDecoder().decode(value).includes("connected"));
    } finally {
      ctrl.abort();
    }
  });

  it("ApprovalWatcher connects", async () => {
    const w = new ApprovalWatcher(api);
    try {
      w.start();
      await new Promise((r) => setTimeout(r, 1500));
      assert.ok(w.isConnected);
    } finally {
      w.stop();
    }
    assert.ok(!w.isConnected);
  });

  it("subagent run wired", async () => {
    const w = new ApprovalWatcher(api);
    try {
      const calls: any[] = [];
      w.setSubagentRun(async (opts) => { calls.push(opts); return { runId: "mock" }; });
      w.start();
      await new Promise((r) => setTimeout(r, 1500));
      assert.ok(w.isConnected);
    } finally {
      w.stop();
    }
  });
});

// ─── Errors ─────────────────────────────────────────────────────────────────

describe("Errors", () => {
  it("invalid wallet", async () => {
    await assert.rejects(() => api.getWallet("nonexistent"), (e: any) => e.status >= 400);
  });

  it("invalid API key", async () => {
    const bad = new WalletAPI({ apiUrl: API_URL, apiKey: "ocw_invalid" });
    await assert.rejects(() => bad.listWallets(), (e: any) => e.status === 401 || e.status === 403);
  });

  it("insufficient balance", async () => {
    await assert.rejects(() => api.transfer(ETH_WALLET, ETH_ADDR, "999999"));
  });
});
