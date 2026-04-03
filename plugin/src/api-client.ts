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
    });
    const text = await res.text();
    let data: any;
    try {
      data = JSON.parse(text);
    } catch {
      // Non-JSON response (e.g. reverse proxy HTML error page).
      const err = new Error(`HTTP ${res.status}: non-JSON response`);
      (err as any).status = res.status;
      (err as any).body = text.slice(0, 200);
      throw err;
    }
    if (!res.ok || (data && data.success === false)) {
      const err = new Error(data.error || `HTTP ${res.status}`);
      (err as any).details = data;
      (err as any).status = res.status;
      throw err;
    }
    return data as T;
  }

  // ── Getters for ApprovalWatcher ──────────────────────────────────────────

  get eventsUrl(): string {
    return `${this.baseUrl}/api/events/stream`;
  }

  get authHeader(): string {
    return `Bearer ${this.apiKey}`;
  }

  // ── Public (no auth required) ────────────────────────────────────────────

  async health(): Promise<any> {
    const res = await fetch(`${this.baseUrl}/api/health`);
    const data = await res.json();
    if (!res.ok || (data && data.success === false)) {
      const err = new Error(data.error || `HTTP ${res.status}`);
      (err as any).details = data;
      (err as any).status = res.status;
      throw err;
    }
    return data;
  }

  async listChains(): Promise<ChainInfo[]> {
    const data = await this.request<{ chains: ChainInfo[] }>(
      "GET",
      "/api/chains"
    );
    return data.chains;
  }

  async getPrices(): Promise<any> {
    const data = await this.request<{ prices: any }>("GET", "/api/prices");
    return data.prices;
  }

  // ── Wallets ──────────────────────────────────────────────────────────────

  async createWallet(chain: string, label: string): Promise<Wallet> {
    const data = await this.request<{ wallet: Wallet }>("POST", "/api/wallets", { chain, label });
    return data.wallet;
  }

  async listWallets(): Promise<Wallet[]> {
    const data = await this.request<{ wallets: Wallet[] }>(
      "GET",
      "/api/wallets"
    );
    return data.wallets;
  }

  async getWallet(id: string): Promise<Wallet> {
    const data = await this.request<{ wallet: Wallet }>("GET", `/api/wallets/${id}`);
    return data.wallet;
  }

  async renameWallet(id: string, label: string): Promise<any> {
    return this.request<any>("PATCH", `/api/wallets/${id}`, { label });
  }

  // ── Balance ──────────────────────────────────────────────────────────────

  async getBalance(walletId: string): Promise<{ balance: string; currency: string; chain: string; address: string }> {
    const resp = await this.request<{ data: { balance: string; currency: string; chain: string; address: string } }>("GET", `/api/wallets/${walletId}/balance`);
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

  async getDailySpent(walletId: string): Promise<any> {
    return this.request<any>("GET", `/api/wallets/${walletId}/daily-spent`);
  }

  // ── Contracts ────────────────────────────────────────────────────────────

  async listContracts(walletId: string): Promise<ContractEntry[]> {
    const data = await this.request<{ contracts: ContractEntry[] }>(
      "GET",
      `/api/wallets/${walletId}/contracts`
    );
    return data.contracts;
  }

  async addContract(
    walletId: string,
    contractAddress: string,
    symbol?: string,
    decimals?: number,
    label?: string
  ): Promise<any> {
    const body: Record<string, unknown> = { contract_address: contractAddress };
    if (symbol !== undefined) body.symbol = symbol;
    if (decimals !== undefined) body.decimals = decimals;
    if (label !== undefined) body.label = label;
    return this.request<any>(
      "POST",
      `/api/wallets/${walletId}/contracts`,
      body
    );
  }

  async updateContract(
    walletId: string,
    contractId: number,
    updates: Partial<Pick<ContractEntry, "symbol" | "decimals" | "label">>
  ): Promise<any> {
    return this.request<any>(
      "PUT",
      `/api/wallets/${walletId}/contracts/${contractId}`,
      updates
    );
  }

  async contractCall(walletId: string, params: unknown): Promise<any> {
    return this.request<any>(
      "POST",
      `/api/wallets/${walletId}/contract-call`,
      params
    );
  }


  async approveToken(
    walletId: string,
    contract: string,
    spender: string,
    amount: string,
    decimals: number
  ): Promise<any> {
    return this.request<any>(
      "POST",
      `/api/wallets/${walletId}/approve-token`,
      { contract, spender, amount, decimals }
    );
  }

  async revokeApproval(
    walletId: string,
    contract: string,
    spender: string
  ): Promise<any> {
    return this.request<any>(
      "POST",
      `/api/wallets/${walletId}/revoke-approval`,
      { contract, spender }
    );
  }

  // ── Wrap/Unwrap SOL ──────────────────────────────────────────────────────

  async wrapSol(walletId: string, amount: string): Promise<any> {
    return this.request<any>("POST", `/api/wallets/${walletId}/wrap-sol`, {
      amount,
    });
  }

  async unwrapSol(walletId: string): Promise<any> {
    return this.request<any>("POST", `/api/wallets/${walletId}/unwrap-sol`);
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
  ): Promise<any> {
    const body: Record<string, unknown> = { nickname, chain, address };
    if (memo !== undefined) body.memo = memo;
    return this.request<any>("POST", "/api/addressbook", body);
  }

  async updateAddressBookEntry(
    id: number,
    updates: Partial<Pick<AddressBookEntry, "nickname" | "chain" | "address" | "memo">>
  ): Promise<any> {
    return this.request<any>("PUT", `/api/addressbook/${id}`, updates);
  }

  // ── Approvals ────────────────────────────────────────────────────────────

  async listPendingApprovals(): Promise<any[]> {
    const data = await this.request<{ approvals: any[] }>("GET", "/api/approvals/pending");
    return data.approvals;
  }

  async getApproval(id: number): Promise<any> {
    return this.request<any>("GET", `/api/approvals/${id}`);
  }

  // ── Misc ─────────────────────────────────────────────────────────────────

  async claimFaucet(walletId: string): Promise<any> {
    return this.request<any>("POST", "/api/faucet", { wallet_id: walletId });
  }

  async auditLogs(
    page?: number,
    limit?: number,
    action?: string,
    walletId?: string
  ): Promise<any> {
    const params = new URLSearchParams();
    if (page !== undefined) params.set("page", String(page));
    if (limit !== undefined) params.set("limit", String(limit));
    if (action) params.set("action", action);
    if (walletId) params.set("wallet_id", walletId);
    const qs = params.toString() ? `?${params.toString()}` : "";
    return this.request<any>("GET", `/api/audit/logs${qs}`);
  }

  async getPubkey(walletId: string): Promise<any> {
    return this.request<any>("GET", `/api/wallets/${walletId}/pubkey`);
  }
}
