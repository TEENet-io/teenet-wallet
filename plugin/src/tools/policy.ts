// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

import { Type } from "@sinclair/typebox";
import type { WalletAPI } from "../api-client.js";
import type { ApprovalWatcher } from "../approval-watcher.js";
import { jsonResult, approvalOrResult, type RegisterTool, type ToolContext } from "./tool-result.js";

export function registerPolicyTools(
  registerTool: RegisterTool,
  api: WalletAPI,
  getApprovalUrl: (id: number) => string,
  watcher: ApprovalWatcher,
) {
  registerTool({
    name: "teenet_wallet_get_policy",
    description: "Get the transfer approval policy for a wallet, including the threshold USD amount, daily limit, and whether the policy is enabled.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
    }),
    async execute(_id: string, params: any) {
      return jsonResult(await api.getPolicy(params.wallet_id));
    },
  });

  registerTool((ctx: ToolContext) => ({
    name: "teenet_wallet_set_policy",
    description: "Set the transfer approval policy for a wallet. Transfers exceeding the threshold_usd will require Passkey approval. May return pending_approval if the policy change itself requires approval.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
      threshold_usd: Type.String({ description: "USD threshold above which transfers require approval (e.g. '100')" }),
      enabled: Type.Boolean({ description: "Whether the approval policy is enabled" }),
      daily_limit_usd: Type.Optional(Type.String({ description: "Optional daily spending limit in USD (e.g. '1000')" })),
    }),
    async execute(_id: string, params: any) {
      const result = await api.setPolicy(params.wallet_id, params.threshold_usd, params.enabled, params.daily_limit_usd);
      const context = `set approval policy: threshold $${params.threshold_usd} USD, enabled=${params.enabled} for wallet ${params.wallet_id}`;
      return approvalOrResult(result, getApprovalUrl, watcher, ctx?.sessionKey, context);
    },
  }));

  registerTool({
    name: "teenet_wallet_daily_spent",
    description: "Get the amount spent today for a wallet, used to track progress toward the daily spending limit.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
    }),
    async execute(_id: string, params: any) {
      return jsonResult(await api.getDailySpent(params.wallet_id));
    },
  });
}
