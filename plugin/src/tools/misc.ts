import { Type } from "@sinclair/typebox";
import type { WalletAPI } from "../api-client.js";
import type { ApprovalWatcher } from "../approval-watcher.js";
import { jsonResult, approvalOrResult } from "./tool-result.js";

export function registerMiscTools(
  registerTool: (tool: any) => void,
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

  registerTool((ctx: any) => ({
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

  registerTool((ctx: any) => ({
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
    name: "teenet_wallet_wait_approval",
    description:
      "Block and wait for a specific approval to be resolved via Passkey. Use this in multi-step flows (e.g. guided test) where you need the approval result before proceeding to the next step. For single operations, prefer the non-blocking approach (the system will notify you automatically).",
    parameters: Type.Object({
      approval_id: Type.Number({ description: "Approval ID to wait for" }),
      timeout_seconds: Type.Optional(Type.Number({ description: "Max seconds to wait (default 1800 = 30 min)" })),
    }),
    async execute(
      _id: string,
      params: { approval_id: number; timeout_seconds?: number },
    ) {
      const timeoutMs = (params.timeout_seconds ?? 1800) * 1000;
      try {
        const event = await watcher.waitForApproval(params.approval_id, timeoutMs);
        return jsonResult({
          approval_id: event.approval_id,
          status: event.status,
          approval_type: event.approval_type,
          ...(event.tx_hash ? { tx_hash: event.tx_hash } : {}),
          ...(event.wallet_id ? { wallet_id: event.wallet_id } : {}),
          message: event.status === "approved"
            ? `Approved.${event.tx_hash ? ` TX: ${event.tx_hash}` : ""}`
            : event.status === "rejected"
              ? "Rejected. No action was taken."
              : `Status: ${event.status}`,
        });
      } catch {
        return jsonResult({
          approval_id: params.approval_id,
          status: "timeout",
          message: "Timed out waiting for approval.",
        });
      }
    },
  });

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
