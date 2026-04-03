import { Type } from "@sinclair/typebox";
import type { WalletAPI } from "../api-client.js";
import type { ApprovalWatcher } from "../approval-watcher.js";
import { jsonResult, approvalOrResult } from "./tool-result.js";

export function registerAddressBookTools(
  registerTool: (tool: any) => void,
  api: WalletAPI,
  getApprovalUrl: (id: number) => string,
  watcher: ApprovalWatcher,
) {
  registerTool({
    name: "teenet_wallet_list_contacts",
    description: "List address book contacts. Optionally filter by nickname or chain.",
    parameters: Type.Object({
      nickname: Type.Optional(Type.String({ description: "Filter by nickname (partial match)" })),
      chain: Type.Optional(Type.String({ description: "Filter by chain name (e.g. sepolia, solana-devnet)" })),
    }),
    async execute(_id: string, params: any) {
      return jsonResult(await api.listAddressBook(params.nickname, params.chain));
    },
  });

  registerTool((ctx: any) => ({
    name: "teenet_wallet_add_contact",
    description: "Add a new contact to the address book. May return pending_approval if the operation requires Passkey approval.",
    parameters: Type.Object({
      nickname: Type.String({ description: "Contact nickname (used as an alias when sending)" }),
      chain: Type.String({ description: "Chain name (e.g. sepolia, solana-devnet)" }),
      address: Type.String({ description: "Blockchain address for this contact" }),
      memo: Type.Optional(Type.String({ description: "Optional memo or note" })),
    }),
    async execute(_id: string, params: any) {
      const result = await api.addAddressBookEntry(params.nickname, params.chain, params.address, params.memo);
      return approvalOrResult(result, getApprovalUrl, watcher, ctx?.sessionKey);
    },
  }));

  registerTool((ctx: any) => ({
    name: "teenet_wallet_update_contact",
    description: "Update an existing address book contact by ID. May return pending_approval if the change requires Passkey approval.",
    parameters: Type.Object({
      entry_id: Type.Number({ description: "Address book entry ID" }),
      nickname: Type.Optional(Type.String({ description: "New nickname" })),
      address: Type.Optional(Type.String({ description: "New blockchain address" })),
      memo: Type.Optional(Type.String({ description: "New memo or note" })),
    }),
    async execute(_id: string, params: any) {
      const updates: Record<string, string> = {};
      if (params.nickname !== undefined) updates.nickname = params.nickname;
      if (params.address !== undefined) updates.address = params.address;
      if (params.memo !== undefined) updates.memo = params.memo;
      const result = await api.updateAddressBookEntry(params.entry_id, updates);
      return approvalOrResult(result, getApprovalUrl, watcher, ctx?.sessionKey);
    },
  }));
}
