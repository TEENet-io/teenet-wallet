import { Type } from "@sinclair/typebox";
import type { WalletAPI } from "../api-client.js";
import { jsonResult } from "./tool-result.js";

export function registerWalletTools(
  registerTool: (tool: any) => void,
  api: WalletAPI,
) {
  registerTool({
    name: "teenet_wallet_create",
    description:
      "Create a new crypto wallet on the specified chain. EVM wallets (Ethereum, Sepolia, etc.) take 1-2 minutes due to distributed key generation. Solana wallets are instant.",
    parameters: Type.Object({
      chain: Type.String({ description: "Chain name from list_chains (e.g. sepolia, solana-devnet)" }),
      label: Type.String({ description: "Human-readable wallet label" }),
    }),
    async execute(_id: string, params: { chain: string; label: string }) {
      const wallet = await api.createWallet(params.chain, params.label);
      return jsonResult(wallet);
    },
  });

  registerTool({
    name: "teenet_wallet_list",
    description: "List all wallets for the current user, including address, chain, label, and status.",
    parameters: Type.Object({}),
    async execute() {
      const wallets = await api.listWallets();
      return jsonResult(wallets);
    },
  });

  registerTool({
    name: "teenet_wallet_get",
    description: "Get details of a specific wallet by ID.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
    }),
    async execute(_id: string, params: { wallet_id: string }) {
      const wallet = await api.getWallet(params.wallet_id);
      return jsonResult(wallet);
    },
  });

  registerTool({
    name: "teenet_wallet_rename",
    description: "Rename a wallet's label. No approval needed.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
      label: Type.String({ description: "New label" }),
    }),
    async execute(_id: string, params: { wallet_id: string; label: string }) {
      await api.renameWallet(params.wallet_id, params.label);
      return jsonResult("Wallet renamed successfully.");
    },
  });
}
