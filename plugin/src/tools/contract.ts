import { Type } from "@sinclair/typebox";
import type { WalletAPI } from "../api-client.js";
import type { ApprovalWatcher } from "../approval-watcher.js";
import { jsonResult, approvalOrResult } from "./tool-result.js";

export function registerContractTools(
  registerTool: (tool: any) => void,
  api: WalletAPI,
  getApprovalUrl: (id: number) => string,
  watcher: ApprovalWatcher,
) {
  registerTool({
    name: "teenet_wallet_list_contracts",
    description: "List all whitelisted token contracts for a wallet (ERC-20 tokens or SPL mints).",
    parameters: Type.Object({ wallet_id: Type.String({ description: "Wallet UUID" }) }),
    async execute(_id: string, params: any) { return jsonResult(await api.listContracts(params.wallet_id)); },
  });

  registerTool((ctx: any) => ({
    name: "teenet_wallet_add_contract",
    description: "Add a token contract to the wallet's whitelist. May return pending_approval if the operation requires Passkey approval.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
      contract_address: Type.String({ description: "Token contract address (ERC-20) or mint address (SPL)" }),
      symbol: Type.Optional(Type.String({ description: "Token symbol (e.g. USDC)" })),
      decimals: Type.Optional(Type.Number({ description: "Token decimals (e.g. 6 for USDC)" })),
      label: Type.Optional(Type.String({ description: "Optional human-readable label" })),
    }),
    async execute(_id: string, params: any) {
      const result = await api.addContract(params.wallet_id, params.contract_address, params.symbol, params.decimals, params.label);
      const context = `whitelist contract ${params.symbol || params.contract_address}${params.label ? ' (' + params.label + ')' : ''} on wallet ${params.wallet_id}`;
      return approvalOrResult(result, getApprovalUrl, watcher, ctx?.sessionKey, context);
    },
  }));

  registerTool((ctx: any) => ({
    name: "teenet_wallet_update_contract",
    description: "Update a whitelisted contract's metadata (label, symbol, decimals). May return pending_approval.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
      contract_id: Type.Number({ description: "Contract whitelist entry ID" }),
      label: Type.Optional(Type.String({ description: "New label" })),
      symbol: Type.Optional(Type.String({ description: "New symbol" })),
      decimals: Type.Optional(Type.Number({ description: "New decimals" })),
    }),
    async execute(_id: string, params: any) {
      const updates: Record<string, unknown> = {};
      if (params.label !== undefined) updates.label = params.label;
      if (params.symbol !== undefined) updates.symbol = params.symbol;
      if (params.decimals !== undefined) updates.decimals = params.decimals;
      const result = await api.updateContract(params.wallet_id, params.contract_id, updates);
      return approvalOrResult(result, getApprovalUrl, watcher, ctx?.sessionKey);
    },
  }));

  registerTool((ctx: any) => ({
    name: "teenet_wallet_contract_call",
    description: "Call a smart contract function (state-changing). Supports EVM and Solana. May return pending_approval.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
      contract: Type.String({ description: "Contract address or program ID" }),
      func_sig: Type.Optional(Type.String({ description: "Function signature (e.g. 'transfer(address,uint256)')" })),
      args: Type.Optional(Type.Array(Type.Unknown({ description: "Function argument value" }), { description: "Function arguments" })),
      value: Type.Optional(Type.String({ description: "Native value to send with the call (e.g. '0.01')" })),
      accounts: Type.Optional(Type.Array(Type.String({ description: "Additional Solana account" }), { description: "Additional accounts for Solana instructions" })),
      data: Type.Optional(Type.String({ description: "Raw calldata (hex) for low-level calls" })),
      memo: Type.Optional(Type.String({ description: "Optional memo" })),
      tx_context: Type.Optional(Type.Unknown({ description: "Optional chain-specific transaction context" })),
    }),
    async execute(_id: string, params: any) {
      const callParams: Record<string, unknown> = { contract: params.contract };
      if (params.func_sig !== undefined) callParams.func_sig = params.func_sig;
      if (params.args !== undefined) callParams.args = params.args;
      if (params.value !== undefined) callParams.value = params.value;
      if (params.accounts !== undefined) callParams.accounts = params.accounts;
      if (params.data !== undefined) callParams.data = params.data;
      if (params.memo !== undefined) callParams.memo = params.memo;
      if (params.tx_context !== undefined) callParams.tx_context = params.tx_context;
      const result = await api.contractCall(params.wallet_id, callParams);
      return approvalOrResult(result, getApprovalUrl, watcher, ctx?.sessionKey);
    },
  }));



  registerTool((ctx: any) => ({
    name: "teenet_wallet_approve_token",
    description: "Approve a spender to spend tokens on behalf of the wallet (ERC-20 approve). May return pending_approval.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
      contract: Type.String({ description: "Token contract address" }),
      spender: Type.String({ description: "Spender address to approve" }),
      amount: Type.String({ description: "Amount to approve in human-readable units" }),
      decimals: Type.Number({ description: "Token decimals" }),
    }),
    async execute(_id: string, params: any) {
      const result = await api.approveToken(params.wallet_id, params.contract, params.spender, params.amount, params.decimals);
      return approvalOrResult(result, getApprovalUrl, watcher, ctx?.sessionKey);
    },
  }));

  registerTool((ctx: any) => ({
    name: "teenet_wallet_revoke_approval",
    description: "Revoke a previously granted token approval by setting the allowance to zero. May return pending_approval.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
      contract: Type.String({ description: "Token contract address" }),
      spender: Type.String({ description: "Spender address whose approval will be revoked" }),
    }),
    async execute(_id: string, params: any) {
      const result = await api.revokeApproval(params.wallet_id, params.contract, params.spender);
      return approvalOrResult(result, getApprovalUrl, watcher, ctx?.sessionKey);
    },
  }));
}
