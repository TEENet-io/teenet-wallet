// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

import { Type } from "@sinclair/typebox";
import type { WalletAPI } from "../api-client.js";
import type { ApprovalWatcher } from "../approval-watcher.js";
import { jsonResult, approvalOrResult, type RegisterTool, type ToolContext } from "./tool-result.js";

export function registerContractTools(
  registerTool: RegisterTool,
  api: WalletAPI,
  getApprovalUrl: (id: number) => string,
  watcher: ApprovalWatcher,
) {
  registerTool({
    name: "teenet_wallet_list_contracts",
    description: "List all whitelisted token contracts on a chain (ERC-20 tokens or SPL mints). The whitelist is per (user, chain) — independent of any wallet.",
    parameters: Type.Object({ chain: Type.String({ description: "Chain name (e.g. 'sepolia', 'base', 'solana-devnet')" }) }),
    async execute(_id: string, params: any) { return jsonResult(await api.listContracts(params.chain)); },
  });

  registerTool((ctx: ToolContext) => ({
    name: "teenet_wallet_add_contract",
    description: "Add a token contract to the chain's whitelist. The whitelist is per (user, chain) — no wallet required. May return pending_approval if the operation requires Passkey approval.",
    parameters: Type.Object({
      chain: Type.String({ description: "Chain name (e.g. 'sepolia', 'base', 'solana-devnet')" }),
      contract_address: Type.String({ description: "Token contract address (ERC-20) or mint address (SPL)" }),
      symbol: Type.Optional(Type.String({ description: "Token symbol (e.g. USDC)" })),
      decimals: Type.Optional(Type.Number({ description: "Token decimals (e.g. 6 for USDC)" })),
      label: Type.Optional(Type.String({ description: "Optional human-readable label" })),
    }),
    async execute(_id: string, params: any) {
      const result = await api.addContract(params.chain, params.contract_address, params.symbol, params.decimals, params.label);
      const context = `whitelist contract ${params.symbol || params.contract_address}${params.label ? ' (' + params.label + ')' : ''} on ${params.chain}`;
      return approvalOrResult(result, getApprovalUrl, watcher, ctx?.sessionKey, context);
    },
  }));

  registerTool({
    name: "teenet_wallet_update_contract",
    description: "Rename a whitelisted contract. Only the display label is editable; symbol and decimals are on-chain metadata and cannot be changed here. Applied immediately — no approval required.",
    parameters: Type.Object({
      chain: Type.String({ description: "Chain name the contract was whitelisted on" }),
      contract_id: Type.Number({ description: "Contract whitelist entry ID" }),
      label: Type.String({ description: "New display label" }),
    }),
    async execute(_id: string, params: any) {
      return jsonResult(await api.updateContract(params.chain, params.contract_id, { label: params.label }));
    },
  });

  registerTool((ctx: ToolContext) => ({
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



  registerTool({
    name: "teenet_wallet_call_read",
    description:
      "Read-only contract call (EVM eth_call). No signing, no gas, no approval, and no contract whitelist required. Use for balanceOf, allowance, totalSupply, pool quotes, and any view/pure function. Returns hex-encoded result.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID (must be on an EVM chain)" }),
      contract: Type.String({ description: "Contract address (0x...)" }),
      func_sig: Type.String({ description: "Function signature, e.g. 'balanceOf(address)' or 'quoteExactInputSingle((address,address,uint256,uint24,uint160))'" }),
      args: Type.Optional(Type.Array(Type.Unknown({ description: "Function argument value" }), { description: "Function arguments in declaration order" })),
    }),
    async execute(_id: string, params: any) {
      return jsonResult(
        await api.callRead(params.wallet_id, params.contract, params.func_sig, params.args),
      );
    },
  });

  registerTool((ctx: ToolContext) => ({
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

  registerTool((ctx: ToolContext) => ({
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
