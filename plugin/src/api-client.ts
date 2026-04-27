// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

export interface WalletAPIConfig {
  apiUrl: string;
  apiKey: string;
}

export interface Wallet {
  id: string;
  chain: string;
  address: string;
  label: string;
  status: string;
  public_key: string;
  curve: string;
  protocol: string;
  created_at: string;
}

export interface ChainInfo {
  name: string;
  label: string;
  protocol: string;
  curve: string;
  currency: string;
  family: string;
  rpc_url: string;
  chain_id: number;
  custom: boolean;
}

export interface TransferResult {
  success: boolean;
  status: string; // "completed" | "pending_approval"
  tx_hash?: string;
  approval_id?: number;
  approval_url?: string;
  // Index signature mirrors MutationResult so this shape is directly
  // assignable to approvalOrResult's permissive param type.
  [key: string]: unknown;
}

export interface PolicyResult {
  success: boolean;
  status?: string;
  approval_id?: number;
  policy?: {
    threshold_usd: string;
    enabled: boolean;
    daily_limit_usd: string;
  };
  [key: string]: unknown;
}

export interface ContractEntry {
  id: number;
  contract_address: string;
  symbol: string;
  decimals: number;
  label: string;
}

export interface AddressBookEntry {
  id: number;
  nickname: string;
  chain: string;
  address: string;
  memo: string;
}

export interface HealthResponse {
  status: string;
  service: string;
  db: boolean;
}

export interface PriceData {
  [currency: string]: number;
}

export interface RenameResult {
  success: boolean;
}

export interface DailySpentResult {
  success: boolean;
  daily_spent_usd: string;
  daily_limit_usd: string;
  reset_at: string;
}

export interface ApprovalDetail {
  success: boolean;
  /** Top-level fields returned by the API (also available via the nested `approval` object). */
  status?: string;
  approval_type?: string;
  tx_hash?: string;
  wallet_id?: string;
  /** Chain name of the wallet this approval belongs to, when WalletID is set. */
  chain?: string;
  approval: {
    id: number;
    status: string;
    approval_type: string;
    tx_hash?: string;
    wallet_id?: string;
    created_at: string;
    expires_at: string;
  };
  tx_context?: unknown;
}

export interface AuditLogsResponse {
  success: boolean;
  logs: Array<{
    id: number;
    action: string;
    status: string;
    details: string;
    created_at: string;
  }>;
  total: number;
  page: number;
  limit: number;
}

export interface PubkeyResponse {
  success: boolean;
  public_key: string;
  key_name: string;
}

export interface FaucetResult {
  success: boolean;
  tx_hash: string;
  amount: string;
  chain: string;
  address: string;
}

export interface MutationResult {
  success: boolean;
  status?: string;
  pending?: boolean;
  approval_id?: number;
  approval_url?: string;
  [key: string]: unknown;
}

export class ApiError extends Error {
  readonly status: number;
  readonly details?: unknown;
  readonly body?: string;

  constructor(message: string, status: number, details?: unknown, body?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.details = details;
    this.body = body;
  }
}

export class WalletAPI {
  private readonly baseUrl: string;
  private readonly apiKey: string;

  constructor(config: WalletAPIConfig) {
    this.baseUrl = config.apiUrl.replace(/\/$/, "");
    this.apiKey = config.apiKey;
  }

  private headers(): Record<string, string> {
    return {
      "Content-Type": "application/json",
      Authorization: `Bearer ${this.apiKey}`,
    };
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown
  ): Promise<T> {
    const res = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers: this.headers(),
      body: body ? JSON.stringify(body) : undefined,
      signal: AbortSignal.timeout(30_000),
    });
    const text = await res.text();
    let data: any;
    try {
      data = JSON.parse(text);
    } catch {
      throw new ApiError(`HTTP ${res.status}: non-JSON response`, res.status, undefined, text.slice(0, 200));
    }
    if (!res.ok || (data && data.success === false)) {
      throw new ApiError(data.error || `HTTP ${res.status}`, res.status, data);
    }
    return data as T;
  }

  /** Retry wrapper for idempotent GET requests. Retries up to 2 times on 503/network errors. */
  private async requestWithRetry<T>(path: string, maxRetries = 2): Promise<T> {
    let lastErr: unknown;
    for (let attempt = 0; attempt <= maxRetries; attempt++) {
      try {
        return await this.request<T>("GET", path);
      } catch (err) {
        lastErr = err;
        const status = err instanceof ApiError ? err.status : 0;
        // Only retry on 503 (service unavailable) or network-level errors (status 0)
        if (status !== 503 && status !== 0) throw err;
        if (attempt < maxRetries) {
          await new Promise((r) => setTimeout(r, (attempt + 1) * 1000));
        }
      }
    }
    throw lastErr;
  }

  // ── Getters for ApprovalWatcher ──────────────────────────────────────────

  get eventsUrl(): string {
    return `${this.baseUrl}/api/events/stream`;
  }

  get authHeader(): string {
    return `Bearer ${this.apiKey}`;
  }

  // ── Public (no auth required) ────────────────────────────────────────────

  async health(): Promise<HealthResponse> {
    const res = await fetch(`${this.baseUrl}/api/health`, {
      signal: AbortSignal.timeout(10_000),
    });
    const data = await res.json();
    if (!res.ok || (data && data.success === false)) {
      throw new ApiError(data.error || `HTTP ${res.status}`, res.status, data);
    }
    return data as HealthResponse;
  }

  async listChains(): Promise<ChainInfo[]> {
    const data = await this.request<{ chains: ChainInfo[] }>(
      "GET",
      "/api/chains"
    );
    return data.chains;
  }

  async getPrices(): Promise<PriceData> {
    const data = await this.request<{ prices: PriceData }>("GET", "/api/prices");
    return data.prices;
  }

  // ── Wallets ──────────────────────────────────────────────────────────────

  async createWallet(chain: string, label: string): Promise<Wallet> {
    const data = await this.request<{ wallet: Wallet }>("POST", "/api/wallets", { chain, label });
    return data.wallet;
  }

  async listWallets(): Promise<Wallet[]> {
    const data = await this.requestWithRetry<{ wallets: Wallet[] }>("/api/wallets");
    return data.wallets;
  }

  async getWallet(id: string): Promise<Wallet> {
    const data = await this.requestWithRetry<{ wallet: Wallet }>(`/api/wallets/${id}`);
    return data.wallet;
  }

  async renameWallet(id: string, label: string): Promise<RenameResult> {
    return this.request<RenameResult>("PATCH", `/api/wallets/${id}`, { label });
  }

  // ── Balance ──────────────────────────────────────────────────────────────

  async getBalance(walletId: string): Promise<{ balance: string; currency: string; chain: string; address: string }> {
    const resp = await this.requestWithRetry<{ data: { balance: string; currency: string; chain: string; address: string } }>(`/api/wallets/${walletId}/balance`);
    return resp.data;
  }

  // ── Transfer ─────────────────────────────────────────────────────────────

  async transfer(
    walletId: string,
    to: string,
    amount: string,
    token?: { contract: string; symbol: string; decimals: number },
    memo?: string
  ): Promise<TransferResult> {
    const body: Record<string, unknown> = { to, amount };
    if (token) body.token = token;
    if (memo) body.memo = memo;
    return this.request<TransferResult>(
      "POST",
      `/api/wallets/${walletId}/transfer`,
      body
    );
  }

  // ── Policy ───────────────────────────────────────────────────────────────

  async getPolicy(walletId: string): Promise<PolicyResult> {
    return this.request<PolicyResult>(
      "GET",
      `/api/wallets/${walletId}/policy`
    );
  }

  async setPolicy(
    walletId: string,
    thresholdUsd: string,
    enabled: boolean,
    dailyLimitUsd?: string
  ): Promise<PolicyResult> {
    const body: Record<string, unknown> = {
      threshold_usd: thresholdUsd,
      enabled,
    };
    if (dailyLimitUsd !== undefined) body.daily_limit_usd = dailyLimitUsd;
    return this.request<PolicyResult>(
      "PUT",
      `/api/wallets/${walletId}/policy`,
      body
    );
  }

  async getDailySpent(walletId: string): Promise<DailySpentResult> {
    return this.request<DailySpentResult>("GET", `/api/wallets/${walletId}/daily-spent`);
  }

  // ── Contracts ────────────────────────────────────────────────────────────

  async listContracts(chain: string): Promise<ContractEntry[]> {
    const data = await this.request<{ contracts: ContractEntry[] }>(
      "GET",
      `/api/chains/${chain}/contracts`
    );
    return data.contracts;
  }

  async addContract(
    chain: string,
    contractAddress: string,
    symbol?: string,
    decimals?: number,
    label?: string
  ): Promise<MutationResult> {
    const body: Record<string, unknown> = { contract_address: contractAddress };
    if (symbol !== undefined) body.symbol = symbol;
    if (decimals !== undefined) body.decimals = decimals;
    if (label !== undefined) body.label = label;
    return this.request<MutationResult>(
      "POST",
      `/api/chains/${chain}/contracts`,
      body
    );
  }

  async updateContract(
    chain: string,
    contractId: number,
    updates: { label: string }
  ): Promise<MutationResult> {
    return this.request<MutationResult>(
      "PUT",
      `/api/chains/${chain}/contracts/${contractId}`,
      updates
    );
  }

  async contractCall(walletId: string, params: unknown): Promise<MutationResult> {
    return this.request<MutationResult>(
      "POST",
      `/api/wallets/${walletId}/contract-call`,
      params
    );
  }

  async callRead(
    walletId: string,
    contract: string,
    funcSig: string,
    args?: unknown[]
  ): Promise<{ success: boolean; result: string; contract: string; method: string }> {
    const body: Record<string, unknown> = { contract, func_sig: funcSig };
    if (args !== undefined) body.args = args;
    return this.request<{ success: boolean; result: string; contract: string; method: string }>(
      "POST",
      `/api/wallets/${walletId}/call-read`,
      body
    );
  }


  async approveToken(
    walletId: string,
    contract: string,
    spender: string,
    amount: string,
    decimals: number
  ): Promise<MutationResult> {
    return this.request<MutationResult>(
      "POST",
      `/api/wallets/${walletId}/approve-token`,
      { contract, spender, amount, decimals }
    );
  }

  async revokeApproval(
    walletId: string,
    contract: string,
    spender: string
  ): Promise<MutationResult> {
    return this.request<MutationResult>(
      "POST",
      `/api/wallets/${walletId}/revoke-approval`,
      { contract, spender }
    );
  }

  // ── Wrap/Unwrap SOL ──────────────────────────────────────────────────────

  async wrapSol(walletId: string, amount: string): Promise<MutationResult> {
    return this.request<MutationResult>("POST", `/api/wallets/${walletId}/wrap-sol`, {
      amount,
    });
  }

  async unwrapSol(walletId: string): Promise<MutationResult> {
    return this.request<MutationResult>("POST", `/api/wallets/${walletId}/unwrap-sol`);
  }

  // ── Address Book ─────────────────────────────────────────────────────────

  async listAddressBook(nickname?: string, chain?: string): Promise<AddressBookEntry[]> {
    const params = new URLSearchParams();
    if (nickname) params.set("nickname", nickname);
    if (chain) params.set("chain", chain);
    const qs = params.toString() ? `?${params.toString()}` : "";
    const data = await this.request<{ entries: AddressBookEntry[] }>("GET", `/api/addressbook${qs}`);
    return data.entries;
  }

  async addAddressBookEntry(
    nickname: string,
    chain: string,
    address: string,
    memo?: string
  ): Promise<MutationResult> {
    const body: Record<string, unknown> = { nickname, chain, address };
    if (memo !== undefined) body.memo = memo;
    return this.request<MutationResult>("POST", "/api/addressbook", body);
  }

  async updateAddressBookEntry(
    id: number,
    updates: Partial<Pick<AddressBookEntry, "nickname" | "chain" | "address" | "memo">>
  ): Promise<MutationResult> {
    return this.request<MutationResult>("PUT", `/api/addressbook/${id}`, updates);
  }

  // ── Approvals ────────────────────────────────────────────────────────────

  async listPendingApprovals(): Promise<ApprovalDetail[]> {
    const data = await this.request<{ approvals: ApprovalDetail[] }>("GET", "/api/approvals/pending");
    return data.approvals;
  }

  async getApproval(id: number): Promise<ApprovalDetail> {
    return this.request<ApprovalDetail>("GET", `/api/approvals/${id}`);
  }

  // ── Misc ─────────────────────────────────────────────────────────────────

  async claimFaucet(walletId: string): Promise<FaucetResult> {
    return this.request<FaucetResult>("POST", "/api/faucet", { wallet_id: walletId });
  }

  async auditLogs(
    page?: number,
    limit?: number,
    action?: string,
    walletId?: string
  ): Promise<AuditLogsResponse> {
    const params = new URLSearchParams();
    if (page !== undefined) params.set("page", String(page));
    if (limit !== undefined) params.set("limit", String(limit));
    if (action) params.set("action", action);
    if (walletId) params.set("wallet_id", walletId);
    const qs = params.toString() ? `?${params.toString()}` : "";
    return this.request<AuditLogsResponse>("GET", `/api/audit/logs${qs}`);
  }

  async getPubkey(walletId: string): Promise<PubkeyResponse> {
    return this.request<PubkeyResponse>("GET", `/api/wallets/${walletId}/pubkey`);
  }
}
