// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { WalletAPI } from "../api-client.js";

function startMockServer(handler: (req: IncomingMessage, res: ServerResponse) => void): Promise<{ url: string; close: () => void }> {
  return new Promise((resolve) => {
    const server = createServer(handler);
    server.listen(0, () => {
      const addr = server.address() as { port: number };
      resolve({
        url: `http://localhost:${addr.port}`,
        close: () => server.close(),
      });
    });
  });
}

function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve) => {
    let data = "";
    req.on("data", (chunk: Buffer) => { data += chunk.toString(); });
    req.on("end", () => resolve(data));
  });
}

describe("WalletAPI", () => {
  it("sends correct auth header", async () => {
    let receivedAuth = "";
    const server = await startMockServer((req, res) => {
      receivedAuth = req.headers.authorization || "";
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ success: true, wallets: [] }));
    });

    try {
      const api = new WalletAPI({ apiUrl: server.url, apiKey: "ocw_test123" });
      await api.listWallets();
      assert.equal(receivedAuth, "Bearer ocw_test123");
    } finally {
      server.close();
    }
  });

  it("health endpoint does not send auth", async () => {
    let receivedAuth: string | undefined;
    const server = await startMockServer((req, res) => {
      receivedAuth = req.headers.authorization;
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ status: "ok", service: "teenet-wallet", db: true }));
    });

    try {
      const api = new WalletAPI({ apiUrl: server.url, apiKey: "ocw_test" });
      const result = await api.health();
      assert.equal(result.status, "ok");
      assert.equal(receivedAuth, undefined);
    } finally {
      server.close();
    }
  });

  it("transfer sends correct body", async () => {
    let receivedBody = "";
    let receivedPath = "";
    const server = await startMockServer(async (req, res) => {
      receivedPath = req.url || "";
      receivedBody = await readBody(req);
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ success: true, status: "completed", tx_hash: "0xabc" }));
    });

    try {
      const api = new WalletAPI({ apiUrl: server.url, apiKey: "ocw_test" });
      const result = await api.transfer("wallet-123", "0xrecipient", "0.1", { contract: "0xtoken", symbol: "USDC", decimals: 6 }, "test memo");
      assert.equal(result.status, "completed");
      assert.equal(result.tx_hash, "0xabc");
      assert.equal(receivedPath, "/api/wallets/wallet-123/transfer");
      const body = JSON.parse(receivedBody);
      assert.equal(body.to, "0xrecipient");
      assert.equal(body.amount, "0.1");
      assert.equal(body.token.contract, "0xtoken");
      assert.equal(body.memo, "test memo");
    } finally {
      server.close();
    }
  });

  it("throws on error response", async () => {
    const server = await startMockServer((req, res) => {
      res.writeHead(400, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ success: false, error: "invalid wallet" }));
    });

    try {
      const api = new WalletAPI({ apiUrl: server.url, apiKey: "ocw_test" });
      await assert.rejects(
        () => api.listWallets(),
        (err: any) => {
          assert.equal(err.message, "invalid wallet");
          assert.equal(err.status, 400);
          return true;
        },
      );
    } finally {
      server.close();
    }
  });

  it("updateContract sends PUT with only label", async () => {
    // Guards the backend contract: updateContract() must PUT only { label } —
    // symbol and decimals are on-chain metadata that the server ignores, and
    // sending them anyway would give a false impression they can be changed.
    let receivedMethod = "";
    let receivedPath = "";
    let receivedBody = "";
    const server = await startMockServer(async (req, res) => {
      receivedMethod = req.method || "";
      receivedPath = req.url || "";
      receivedBody = await readBody(req);
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ success: true, contract: { id: 42, label: "renamed" } }));
    });

    try {
      const api = new WalletAPI({ apiUrl: server.url, apiKey: "ocw_test" });
      await api.updateContract("sepolia", 42, { label: "renamed" });
      assert.equal(receivedMethod, "PUT");
      assert.equal(receivedPath, "/api/chains/sepolia/contracts/42");
      const body = JSON.parse(receivedBody);
      assert.deepEqual(Object.keys(body).sort(), ["label"]);
      assert.equal(body.label, "renamed");
      assert.equal(body.symbol, undefined);
      assert.equal(body.decimals, undefined);
    } finally {
      server.close();
    }
  });

  it("eventsUrl and authHeader getters", () => {
    const api = new WalletAPI({ apiUrl: "https://wallet.teenet.app", apiKey: "ocw_abc" });
    assert.equal(api.eventsUrl, "https://wallet.teenet.app/api/events/stream");
    assert.equal(api.authHeader, "Bearer ocw_abc");
  });

  it("strips trailing slash from apiUrl", () => {
    const api = new WalletAPI({ apiUrl: "https://wallet.teenet.app/", apiKey: "ocw_abc" });
    assert.equal(api.eventsUrl, "https://wallet.teenet.app/api/events/stream");
  });
});
