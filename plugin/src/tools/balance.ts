import { Type } from "@sinclair/typebox";
import type { WalletAPI } from "../api-client.js";
import { jsonResult, type RegisterTool } from "./tool-result.js";

export function registerBalanceTools(
  registerTool: RegisterTool,
  api: WalletAPI,
) {
  registerTool({
    name: "teenet_wallet_balance",
    description:
      "Get the native balance and whitelisted token list for a wallet. Returns both the native balance and all whitelisted token contracts in a single call.",
    parameters: Type.Object({
      wallet_id: Type.String({ description: "Wallet UUID" }),
    }),
    async execute(_id: string, params: { wallet_id: string }) {
      const [balance, contracts] = await Promise.all([
        api.getBalance(params.wallet_id),
        api.listContracts(params.wallet_id),
      ]);
      return jsonResult({ balance, contracts });
    },
  });
}
