// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

import { join } from "node:path";
import { definePluginEntry } from "openclaw/plugin-sdk/plugin-entry";
import { WalletAPI } from "./src/api-client.js";
import { ApprovalWatcher } from "./src/approval-watcher.js";
import { registerWalletTools } from "./src/tools/wallet.js";
import { registerBalanceTools } from "./src/tools/balance.js";
import { registerTransferTools } from "./src/tools/transfer.js";
import { registerPolicyTools } from "./src/tools/policy.js";
import { registerContractTools } from "./src/tools/contract.js";
import { registerAddressBookTools } from "./src/tools/address-book.js";
import { registerMiscTools } from "./src/tools/misc.js";

export default definePluginEntry({
  id: "teenet-wallet",
  name: "TEENet Wallet",
  description: "Manage crypto wallets secured by TEE hardware",
  register(api) {
    const config = api.pluginConfig as { apiUrl: string; apiKey: string };
    api.logger.info("[teenet-wallet] register", { apiUrl: config?.apiUrl, hasKey: !!config?.apiKey });

    const apiUrl = config?.apiUrl || "";
    const apiKey = config?.apiKey || "";
    if (!apiUrl || !apiKey) {
      api.logger.error("[teenet-wallet] apiUrl and apiKey must be configured");
      return;
    }

    const walletApi = new WalletAPI({ apiUrl, apiKey });
    const watcher = new ApprovalWatcher(walletApi);
    watcher.setLogger(api.logger);

    // Persist approval tracking to disk so it survives plugin re-registration / gateway restarts.
    const dataDir = (api as any).dataDir || join(process.env.HOME || "/tmp", ".openclaw", "workspace", "memory");
    watcher.setStoragePath(join(dataDir, "approval-notifications.json"));

    // Wire subagent API for approval notifications.
    if (api.runtime?.subagent?.run) {
      watcher.setSubagentRun(api.runtime.subagent.run);
    }

    const baseUrl = apiUrl.replace(/\/$/, "");
    const getApprovalUrl = (id: number) => `${baseUrl}/#/approve/${id}`;

    const reg = api.registerTool.bind(api);
    registerWalletTools(reg, walletApi);
    registerBalanceTools(reg, walletApi);
    registerTransferTools(reg, walletApi, getApprovalUrl, watcher);
    registerPolicyTools(reg, walletApi, getApprovalUrl, watcher);
    registerContractTools(reg, walletApi, getApprovalUrl, watcher);
    registerAddressBookTools(reg, walletApi, getApprovalUrl, watcher);
    registerMiscTools(reg, walletApi, getApprovalUrl, watcher);

    api.registerService({
      id: "teenet-wallet-sse",
      start() {
        api.logger.info("[teenet-wallet] service.start — connecting SSE");
        watcher.start();
      },
      stop() {
        api.logger.info("[teenet-wallet] service.stop");
        watcher.stop();
      },
    });
  },
});
