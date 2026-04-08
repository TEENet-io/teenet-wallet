// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

import { Type } from "@sinclair/typebox";
import type { WalletAPI } from "../api-client.js";
import type { ApprovalWatcher } from "../approval-watcher.js";
import { jsonResult, approvalOrResult, type RegisterTool, type ToolContext } from "./tool-result.js";

export function registerTransferTools(
  registerTool: RegisterTool,
  api: WalletAPI,
  getApprovalUrl: (id: number) => string,
  watcher: ApprovalWatcher,
) {
  // Use a tool factory so we get sessionKey from the context.
  registerTool((ctx: ToolContext) => ({
    name: "teenet_wallet_transfer",
    description:
      "Send crypto (native or token) from a wallet. If the amount exceeds the wallet's approval threshold, returns pending_approval with an approval URL — the user must approve via Passkey in the browser. The system will automatically notify you when approval completes (no need to poll).",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
      to: Type.String({ description: "Recipient address or address book nickname" }),
      amount: Type.String({ description: "Amount in human-readable units (e.g. '0.1', '100')" }),
      token_contract: Type.Optional(Type.String({ description: "Token contract address (ERC-20) or mint (SPL). Omit for native transfer." })),
      token_symbol: Type.Optional(Type.String({ description: "Token symbol (e.g. USDC). Required if token_contract is set." })),
      token_decimals: Type.Optional(Type.Number({ description: "Token decimals (e.g. 6 for USDC). Required if token_contract is set." })),
      memo: Type.Optional(Type.String({ description: "Optional memo" })),
    }),
    async execute(_id: string, params: any) {
      const token =
        params.token_contract && params.token_symbol && params.token_decimals !== undefined
          ? { contract: params.token_contract, symbol: params.token_symbol, decimals: params.token_decimals }
          : undefined;

      const result = await api.transfer(params.wallet_id, params.to, params.amount, token, params.memo);

      if (result.status === "completed") {
        return jsonResult({ status: "completed", tx_hash: result.tx_hash, message: "Transfer successful." });
      }

      const context = `transfer ${params.amount} ${params.token_symbol || 'native'} to ${params.to} from wallet ${params.wallet_id}`;
      return approvalOrResult(result, getApprovalUrl, watcher, ctx?.sessionKey, context);
    },
  }));
}
