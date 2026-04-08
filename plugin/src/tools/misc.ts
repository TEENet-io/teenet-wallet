// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

import { Type } from "@sinclair/typebox";
import type { WalletAPI } from "../api-client.js";
import type { ApprovalWatcher } from "../approval-watcher.js";
import { jsonResult, approvalOrResult, type RegisterTool, type ToolContext } from "./tool-result.js";

export function registerMiscTools(
  registerTool: RegisterTool,
  api: WalletAPI,
  getApprovalUrl: (id: number) => string,
  watcher: ApprovalWatcher,
) {
  registerTool({
    name: "teenet_wallet_list_chains",
    description: "List all supported blockchain networks with their details (name, protocol, curve, RPC URL, etc.).",
    parameters: Type.Object({}),
    async execute() {
      return jsonResult(await api.listChains());
    },
  });

  registerTool({
    name: "teenet_wallet_health",
    description: "Check the health status of the wallet API service.",
    parameters: Type.Object({}),
    async execute() {
      return jsonResult(await api.health());
    },
  });

  registerTool({
    name: "teenet_wallet_faucet",
    description: "Claim testnet tokens from the faucet for a wallet. Only available on test networks.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
    }),
    async execute(_id: string, params: { wallet_id: string }) {
      return jsonResult(await api.claimFaucet(params.wallet_id));
    },
  });

  registerTool({
    name: "teenet_wallet_pending_approvals",
    description: "List all pending approval requests that require Passkey confirmation.",
    parameters: Type.Object({}),
    async execute() {
      return jsonResult(await api.listPendingApprovals());
    },
  });

  registerTool({
    name: "teenet_wallet_check_approval",
    description: "Check the status of a specific approval request by ID.",
    parameters: Type.Object({
      approval_id: Type.Number({ description: "Approval request ID" }),
    }),
    async execute(_id: string, params: { approval_id: number }) {
      return jsonResult(await api.getApproval(params.approval_id));
    },
  });

  registerTool({
    name: "teenet_wallet_audit_logs",
    description: "View audit logs for wallet operations. Supports pagination and filtering by action type or wallet.",
    parameters: Type.Object({
      page: Type.Optional(Type.Number({ description: "Page number (default 1)" })),
      limit: Type.Optional(Type.Number({ description: "Results per page (default 20)" })),
      action: Type.Optional(Type.String({ description: "Filter by action type (e.g. transfer, sign, policy)" })),
      wallet_id: Type.Optional(Type.String({ description: "Filter by wallet UUID" })),
    }),
    async execute(
      _id: string,
      params: { page?: number; limit?: number; action?: string; wallet_id?: string },
    ) {
      return jsonResult(await api.auditLogs(params.page, params.limit, params.action, params.wallet_id));
    },
  });

  registerTool({
    name: "teenet_wallet_prices",
    description: "Get current token/asset prices.",
    parameters: Type.Object({}),
    async execute() {
      return jsonResult(await api.getPrices());
    },
  });

  registerTool((ctx: ToolContext) => ({
    name: "teenet_wallet_wrap_sol",
    description: "Wrap native SOL into wSOL (wrapped SOL) for use in Solana DeFi protocols. May return pending_approval.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID (must be a Solana wallet)" }),
      amount: Type.String({ description: "Amount of SOL to wrap (e.g. '0.5')" }),
    }),
    async execute(_id: string, params: { wallet_id: string; amount: string }) {
      const result = await api.wrapSol(params.wallet_id, params.amount);
      const context = `wrap ${params.amount} SOL in wallet ${params.wallet_id}`;
      return approvalOrResult(result, getApprovalUrl, watcher, ctx?.sessionKey, context);
    },
  }));

  registerTool((ctx: ToolContext) => ({
    name: "teenet_wallet_unwrap_sol",
    description: "Unwrap all wSOL back to native SOL in a Solana wallet. May return pending_approval.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID (must be a Solana wallet)" }),
    }),
    async execute(_id: string, params: { wallet_id: string }) {
      const result = await api.unwrapSol(params.wallet_id);
      const context = `unwrap SOL in wallet ${params.wallet_id}`;
      return approvalOrResult(result, getApprovalUrl, watcher, ctx?.sessionKey, context);
    },
  }));

  registerTool({
    name: "teenet_wallet_get_pubkey",
    description: "Get the raw public key for a wallet.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
    }),
    async execute(_id: string, params: { wallet_id: string }) {
      return jsonResult(await api.getPubkey(params.wallet_id));
    },
  });

}
