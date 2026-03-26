# Simplify Contract Security Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Simplify the contract call security model: limits only apply to transfers, all contract operations require Passkey approval via API Key, remove `allowed_methods`/`auto_approve`/`amount_usd`/high-risk logic, add token price lookup for transfer pricing, add daily-spent query endpoint.

**Architecture:** Remove unused security layers (`allowed_methods`, `auto_approve`, `highRiskMethods`) since all contract operations now require approval. Add `GetTokenUSDPrice` to PriceService for ERC-20 transfer pricing. Make transfers with unpriced tokens require approval. Add `GET /api/wallets/:id/daily-spent` endpoint.

**Tech Stack:** Go 1.24+, Gin, GORM (SQLite), CoinGecko API, httptest

**Design docs:** `docs/approval-threshold-design.md`, `docs/contract-call-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `model/contract.go` | Modify | Remove `AllowedMethods` and `AutoApprove` fields |
| `handler/contract_call.go` | Modify | Remove high-risk/AutoApprove/method-restriction/amount_usd; all API Key contract ops → approval |
| `handler/contract.go` | Modify | Remove `allowed_methods`/`auto_approve` from Add/Update handlers |
| `handler/approval.go:141-142` | Modify | Remove `allowed_methods`/`auto_approve` from contract-update apply |
| `handler/wallet.go` | Modify | Wire `GetTokenUSDPrice` into transfer pricing; unknown price → approval; add `DailySpent` handler |
| `handler/price.go` | Modify | Add `GetTokenUSDPrice(chainName, contractAddr)` |
| `main.go` | Modify | Register `/daily-spent` route |
| `handler/contract_call_test.go` | Modify | Update all tests for removed fields and simplified security |
| `handler/contract_test.go` | Modify | Remove `allowed_methods`/`auto_approve` from tests |
| `handler/policy_usd_test.go` | Modify | Remove `amount_usd` tests, add unknown-token and daily-spent tests |
| `handler/price_test.go` | Modify | Add `GetTokenUSDPrice` tests |
| `README.md` | Modify | Update security model docs |
| `skill/tee-wallet/SKILL.md` | Modify | Update contract/approval sections |

Note: GORM AutoMigrate does not drop columns from SQLite. Removed fields simply become unmapped — no migration needed, old data is harmless.

---

### Task 1: PriceService — Add `GetTokenUSDPrice`

**Files:**
- Modify: `handler/price.go`
- Test: `handler/price_test.go`

- [ ] **Step 1: Write failing tests**

Add `"strings"` to imports in `handler/price_test.go`, then add:

```go
func fakeCoinGeckoTokenServer(prices map[string]float64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "/api/v3/simple/price") {
			resp := map[string]map[string]float64{
				"ethereum": {"usd": 3500},
				"solana":   {"usd": 150},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		if strings.Contains(path, "/api/v3/simple/token_price/") {
			addr := strings.ToLower(r.URL.Query().Get("contract_addresses"))
			if p, ok := prices[addr]; ok {
				resp := map[string]map[string]float64{addr: {"usd": p}}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			} else {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{})
			}
			return
		}
		http.NotFound(w, r)
	}))
}

func TestGetTokenUSDPrice_Success(t *testing.T) {
	srv := fakeCoinGeckoTokenServer(map[string]float64{
		"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48": 1.0,
	})
	defer srv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)
	price, err := ps.GetTokenUSDPrice("ethereum", "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 1.0 {
		t.Fatalf("expected 1.0, got %f", price)
	}
}

func TestGetTokenUSDPrice_CacheHit(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/v3/simple/token_price/") {
			callCount++
			addr := strings.ToLower(r.URL.Query().Get("contract_addresses"))
			resp := map[string]map[string]float64{addr: {"usd": 5.0}}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		resp := map[string]map[string]float64{"ethereum": {"usd": 3500}, "solana": {"usd": 150}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)
	ps.GetTokenUSDPrice("ethereum", "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48")
	ps.GetTokenUSDPrice("ethereum", "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48")
	if callCount != 1 {
		t.Fatalf("expected 1 API call (cached), got %d", callCount)
	}
}

func TestGetTokenUSDPrice_TestnetFails(t *testing.T) {
	srv := fakeCoinGeckoTokenServer(nil)
	defer srv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)
	_, err := ps.GetTokenUSDPrice("sepolia", "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48")
	if err == nil {
		t.Fatal("expected error for testnet chain")
	}
}

func TestGetTokenUSDPrice_UnknownToken(t *testing.T) {
	srv := fakeCoinGeckoTokenServer(map[string]float64{})
	defer srv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)
	_, err := ps.GetTokenUSDPrice("ethereum", "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./handler/ -run TestGetTokenUSDPrice -v`
Expected: FAIL — method does not exist

- [ ] **Step 3: Implement**

In `handler/price.go`, add after `coinGeckoIDs` var block:

```go
var coinGeckoPlatformIDs = map[string]string{
	"ethereum": "ethereum",
	"optimism": "optimistic-ethereum",
}
```

Add fields to `PriceService` struct after `baseURL`:
```go
	tokenPrices map[string]float64
	tokenExpiry map[string]time.Time
```

Initialize in `NewPriceServiceWithBaseURL`:
```go
	tokenPrices: make(map[string]float64),
	tokenExpiry: make(map[string]time.Time),
```

Add method:
```go
func (ps *PriceService) GetTokenUSDPrice(chainName, contractAddress string) (float64, error) {
	platform, ok := coinGeckoPlatformIDs[strings.ToLower(chainName)]
	if !ok {
		return 0, fmt.Errorf("no token price support for chain %q", chainName)
	}
	addr := strings.ToLower(strings.TrimSpace(contractAddress))
	cacheKey := platform + ":" + addr

	ps.mu.RLock()
	if price, ok := ps.tokenPrices[cacheKey]; ok {
		if time.Now().Before(ps.tokenExpiry[cacheKey]) {
			ps.mu.RUnlock()
			return price, nil
		}
	}
	ps.mu.RUnlock()

	url := fmt.Sprintf("%s/api/v3/simple/token_price/%s?contract_addresses=%s&vs_currencies=usd",
		ps.baseURL, platform, addr)
	resp, err := ps.client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("token price fetch failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("token price API returned %d", resp.StatusCode)
	}

	var result map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("token price parse failed: %w", err)
	}
	data, exists := result[addr]
	if !exists {
		return 0, fmt.Errorf("no price data for token %s on %s", addr, platform)
	}
	usd, ok := data["usd"]
	if !ok || usd <= 0 {
		return 0, fmt.Errorf("invalid USD price for token %s", addr)
	}

	ps.mu.Lock()
	ps.tokenPrices[cacheKey] = usd
	ps.tokenExpiry[cacheKey] = time.Now().Add(ps.ttl)
	ps.mu.Unlock()
	return usd, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./handler/ -run "TestGetTokenUSDPrice|TestPriceService" -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add handler/price.go handler/price_test.go
git commit -m "feat: PriceService.GetTokenUSDPrice — ERC-20 price lookup by contract address via CoinGecko"
```

---

### Task 2: Remove `AllowedMethods`, `AutoApprove`, high-risk logic, `amount_usd` — all contract ops require approval

This is one atomic task because model fields, handler logic, and tests must change together to compile.

**Files:**
- Modify: `model/contract.go`
- Modify: `handler/contract_call.go`
- Modify: `handler/contract.go`
- Modify: `handler/approval.go`
- Modify: `handler/contract_call_test.go`
- Modify: `handler/contract_test.go`
- Modify: `handler/policy_usd_test.go`

- [ ] **Step 1: Remove model fields**

In `model/contract.go`, remove from `AllowedContract`:
```go
	AllowedMethods  string    `json:"allowed_methods"`
	AutoApprove     bool      `json:"auto_approve" gorm:"default:false"`
```

- [ ] **Step 2: Remove from `handler/contract.go` AddContract**

In `AddContract` request struct, remove `AllowedMethods` and `AutoApprove` fields.
In the `proposed` construction, remove `AllowedMethods` and `AutoApprove` assignments.

- [ ] **Step 3: Remove from `handler/contract.go` UpdateContract**

In `UpdateContract` request struct, remove `AllowedMethods` and `AutoApprove` pointer fields.
Remove the merge logic blocks for these fields.
Remove the update map entries for these fields.

- [ ] **Step 4: Remove from `handler/approval.go:141-142`**

Delete these two lines from the contract_update apply block:
```go
				"allowed_methods": proposed.AllowedMethods,
				"auto_approve":    proposed.AutoApprove,
```

- [ ] **Step 5: Simplify `handler/contract_call.go`**

**Delete** the `highRiskMethods` map (lines 21-28) and `highRiskSOLDiscriminators` map (lines 30-37).

**Delete** `AmountUSD` field from `ContractCallRequest` (line 68).

**In `contractCallEVM`**, replace the security decision block (lines ~140-260):
- Delete the `allowed.AllowedMethods` method restriction check (lines 140-152)
- Replace the high-risk + AutoApprove + amount_usd + daily-limit + threshold logic (lines 194-260) with:

```go
	// All contract operations via API Key require Passkey approval.
	needsApproval := false
	var approvalReason string

	if !isPasskeyAuth(c) {
		needsApproval = true
		approvalReason = "contract operations require passkey approval"
	}
```

Keep the `effectiveUSD` computation for payable ETH value (for display only in txContext), but remove the `amount_usd` block and the daily-limit/threshold checks.

**In `contractCallSolana`**, same simplification:
- Delete the discriminator restriction check (lines ~390-402)
- Replace the security decision block (lines ~425-470) with:

```go
	needsApproval := false
	var approvalReason string
	if !isPasskeyAuth(c) {
		needsApproval = true
		approvalReason = "contract operations require passkey approval"
	}
```

Remove the `amount_usd`, daily-limit, and threshold blocks for Solana.

**Update comments** on `ApproveToken` (line 559) and `RevokeApproval` (line 572):
```go
// API Key auth: always requires Passkey approval (contract operations are not threshold-based).
// Passkey auth: executes directly.
```

- [ ] **Step 6: Update `seedWalletWithContract` in `handler/contract_call_test.go`**

Change signature to remove `allowedMethods` and `autoApprove` params:

```go
func seedWalletWithContract(t *testing.T, db *gorm.DB) (model.User, model.Wallet, model.AllowedContract) {
	t.Helper()
	user, wallet := seedWallet(t, db)
	contract := model.AllowedContract{
		WalletID:        wallet.ID,
		ContractAddress: "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		Symbol:          "USDC",
		Decimals:        6,
	}
	if err := db.Create(&contract).Error; err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	return user, wallet, contract
}
```

- [ ] **Step 7: Fix all callers of `seedWalletWithContract`**

These all need to drop the extra args and use the new signature `seedWalletWithContract(t, db)`:

| Test | Old call | Line |
|------|----------|------|
| `TestContractCall_MethodNotAllowed` | `seedWalletWithContract(t, db, "transfer", true)` | 160 |
| `TestContractCall_HighRiskForceApproval_APIKey` | `seedWalletWithContract(t, db, "", true)` | 188 |
| `TestContractCall_ApprovalStoresETHTxParams` | `seedWalletWithContract(t, db, "", false)` | 249 |
| `TestContractCall_AutoApproveFalse_APIKey` | `seedWalletWithContract(t, db, "", false)` | 304 |
| `TestContractCall_EVMRequiresFuncSig` | `seedWalletWithContract(t, db, "", true)` | 553 |
| `TestApproveToken_APIKey_PendingApproval` | `seedWalletWithContract(t, db, "", true)` | 628 |
| `TestRevokeApproval_APIKey_PendingApproval` | `seedWalletWithContract(t, db, "", true)` | 689 |
| `TestContractCall_InvalidFuncSig` | `seedWalletWithContract(t, db, "", true)` | 749 |

- [ ] **Step 8: Delete or rename tests**

**Delete entirely:**
- `TestContractCall_MethodNotAllowed` (tests method restriction — removed)
- `TestContractCall_AutoApproveFalse_APIKey` (AutoApprove removed — all API Key → 202)
- `TestContractCall_SolanaDiscriminatorNotAllowed` (discriminator restriction — removed)

**Rename/simplify:**
- `TestContractCall_HighRiskForceApproval_APIKey` → rename to `TestContractCall_APIKey_RequiresApproval`, remove "high-risk" references
- `TestContractCall_SolanaHighRisk_APIKey` → rename to `TestContractCall_Solana_APIKey_RequiresApproval`, remove high-risk references

**Fix Solana tests that create AllowedContract with AutoApprove/AllowedMethods:**
- `TestContractCall_SolanaNoAccounts` (line 463): remove `AutoApprove: true`
- `TestContractCall_SolanaHighRisk_APIKey` (line 504): remove `AutoApprove: true`
- `TestContractCall_SolanaDiscriminatorNotAllowed` (line 419-420): delete entire test

**Add new test:**
```go
func TestContractCall_PasskeyAuth_DirectExecution(t *testing.T) {
	db := testDB(t)
	user, wallet, _ := seedWalletWithContract(t, db)
	rpc := mockETHRPCServer(t)
	r := contractCallRouter(db, user.ID, "passkey", rpc.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "approve(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusAccepted {
		t.Fatalf("Passkey should skip approval, got 202: %s", w.Body.String())
	}
}
```

- [ ] **Step 9: Fix `handler/policy_usd_test.go`**

**Delete** these tests (they reference `amount_usd` or `AutoApprove`):
- `TestContractCall_AmountUSD_AboveThreshold_PendingApproval` (line 269)
- `TestContractCall_AmountUSD_DailyLimitExceeded` (line 306)
- `TestContractCall_NativeValueVsAmountUSD_UsesLarger` (line 339)
- `TestContractCall_Solana_AmountUSD_AboveThreshold` (line 374)

**Fix** `TestContractCall_NoAmountUSD_NoPolicy_PassesThrough` (line 657): remove `AutoApprove: true` from the AllowedContract. This test now verifies that API Key contract calls go to 202 (since all contract ops require approval), so update the assertion too — it should expect 202 instead of direct execution.

- [ ] **Step 10: Fix `handler/contract_test.go`**

Remove `allowed_methods` and `auto_approve` from request bodies and assertions. Remove `AllowedMethods`/`AutoApprove` from any AllowedContract assertions.

- [ ] **Step 11: Run all tests**

Run: `go test ./handler/ -count=1`
Expected: ALL PASS (iterate until green — this is a large change)

- [ ] **Step 12: Commit**

```bash
git add model/contract.go handler/contract_call.go handler/contract.go handler/approval.go \
  handler/contract_call_test.go handler/contract_test.go handler/policy_usd_test.go
git commit -m "refactor: simplify contract security — all contract ops require approval, remove allowed_methods/auto_approve/high-risk/amount_usd"
```

---

### Task 3: Transfer — wire `GetTokenUSDPrice` + unknown token → approval

**Files:**
- Modify: `handler/wallet.go` (lines 699-708)
- Test: `handler/policy_usd_test.go`

- [ ] **Step 1: Write failing test**

Add a helper `policyUSDRouterWithAuth` to `handler/policy_usd_test.go` (the existing `policyUSDRouter` hardcodes authMode="passkey"):

```go
func policyUSDRouterWithAuth(t *testing.T, db *gorm.DB, userID uint, ps *handler.PriceService, authMode string) *gin.Engine {
	t.Helper()
	rpc := mockETHRPCServer(t)
	if cfg, ok := model.Chains["ethereum"]; ok {
		cfg.RPCURL = rpc.URL
		model.Chains["ethereum"] = cfg
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := handler.NewWalletHandler(db, nil, "http://localhost")
	if ps != nil {
		wh.SetPriceService(ps)
	}
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", authMode)
		c.Next()
	})
	r.POST("/wallets/:id/transfer", wh.Transfer)
	r.GET("/wallets/:id/daily-spent", wh.DailySpent)
	return r
}
```

Add test:

```go
func TestTransfer_UnknownTokenPrice_RequiresApproval(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	db.Create(&model.AllowedContract{
		WalletID:        wallet.ID,
		ContractAddress: "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Symbol:          "UNKNOWN",
		Decimals:        18,
	})
	db.Create(&model.ApprovalPolicy{WalletID: wallet.ID, ThresholdUSD: "100", Enabled: true})

	ps := makePS(3500, 150) // knows ETH/SOL but not UNKNOWN
	r := policyUSDRouterWithAuth(t, db, user.ID, ps, "apikey")
	body := jsonBody(map[string]interface{}{
		"to":     "0x0000000000000000000000000000000000005678",
		"amount": "10",
		"token":  map[string]interface{}{"contract": "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "symbol": "UNKNOWN", "decimals": 18},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for unknown token price, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./handler/ -run TestTransfer_UnknownTokenPrice -v`
Expected: FAIL (currently unknown tokens skip threshold and proceed directly)

- [ ] **Step 3: Modify transfer flow in `handler/wallet.go`**

In the USD calculation block (lines 699-708), add fallback to `GetTokenUSDPrice` when `GetUSDPrice` fails:

```go
	var amountUSD float64
	if policyFound && h.prices != nil {
		if usdPrice, priceErr := h.prices.GetUSDPrice(currency); priceErr == nil && usdPrice > 0 {
			if a, ok := new(big.Float).SetString(req.Amount); ok {
				f, _ := a.Float64()
				amountUSD = f * usdPrice
			}
		} else if tokenContractAddr != "" {
			// Fallback: try pricing by contract address (ERC-20 tokens not in symbol list).
			if usdPrice, priceErr := h.prices.GetTokenUSDPrice(chainCfg.Name, tokenContractAddr); priceErr == nil && usdPrice > 0 {
				if a, ok := new(big.Float).SetString(req.Amount); ok {
					f, _ := a.Float64()
					amountUSD = f * usdPrice
				}
			}
		}
	}
```

After this block, add the unknown-token fail-closed check (before the daily limit check):

```go
	// Unknown token price with active policy → require approval (fail-closed).
	if policyFound && amountUSD == 0 && tokenContractAddr != "" && !isPasskeyAuth(c) {
		signReq := SignRequest{
			Message:   hex.EncodeToString(signingMsg),
			Encoding:  "hex",
			TxContext: txContext,
		}
		h.createApprovalRequest(c, wallet, signReq, signingMsg, &policy, txParamsJSON)
		return
	}
```

- [ ] **Step 4: Run tests**

Run: `go test ./handler/ -run TestTransfer -v`
Expected: ALL PASS

- [ ] **Step 5: Run full regression**

Run: `go test ./handler/ -count=1`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add handler/wallet.go handler/policy_usd_test.go
git commit -m "feat: transfer with unknown token price requires approval; fallback to GetTokenUSDPrice for ERC-20"
```

---

### Task 4: Add `GET /api/wallets/:id/daily-spent` endpoint

**Files:**
- Modify: `handler/wallet.go`
- Modify: `main.go`
- Test: `handler/policy_usd_test.go`

- [ ] **Step 1: Write failing tests**

Add to `handler/policy_usd_test.go` (using the `policyUSDRouterWithAuth` helper from Task 3):

```go
func TestDailySpent_WithPolicy(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	db.Create(&model.ApprovalPolicy{
		WalletID:      wallet.ID,
		ThresholdUSD:  "100",
		DailyLimitUSD: "1000",
		DailySpentUSD: "235.50",
		DailyResetAt:  time.Now().UTC().Truncate(24 * time.Hour),
		Enabled:       true,
	})

	r := policyUSDRouterWithAuth(t, db, user.ID, nil, "apikey")
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/daily-spent", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["daily_spent_usd"] != "235.50" {
		t.Errorf("expected daily_spent_usd=235.50, got %v", resp["daily_spent_usd"])
	}
	if resp["daily_limit_usd"] != "1000" {
		t.Errorf("expected daily_limit_usd=1000, got %v", resp["daily_limit_usd"])
	}
	if resp["remaining_usd"] == nil || resp["remaining_usd"] == "" {
		t.Error("expected remaining_usd to be set")
	}
}

func TestDailySpent_NoPolicy(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)

	r := policyUSDRouterWithAuth(t, db, user.ID, nil, "apikey")
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/daily-spent", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["daily_spent_usd"] != "0" {
		t.Errorf("expected daily_spent_usd=0, got %v", resp["daily_spent_usd"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./handler/ -run TestDailySpent -v`
Expected: FAIL — `DailySpent` method does not exist

- [ ] **Step 3: Implement `DailySpent` handler in `handler/wallet.go`**

```go
// DailySpent returns the current day's USD spend and limit info.
// GET /api/wallets/:id/daily-spent
func (h *WalletHandler) DailySpent(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}

	var policy model.ApprovalPolicy
	if err := h.db.Where("wallet_id = ? AND enabled = ?", wallet.ID, true).First(&policy).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"daily_spent_usd": "0",
			"daily_limit_usd": "",
			"remaining_usd":   "",
			"reset_at":        "",
		})
		return
	}

	startOfDay := utcStartOfDay()
	spent := policy.DailySpentUSD
	if policy.DailyResetAt.Before(startOfDay) {
		spent = "0"
	}

	var remaining string
	if policy.DailyLimitUSD != "" {
		limit, _ := new(big.Float).SetString(policy.DailyLimitUSD)
		spentF, _ := new(big.Float).SetString(spent)
		if limit != nil && spentF != nil {
			rem := new(big.Float).Sub(limit, spentF)
			if rem.Sign() < 0 {
				rem = new(big.Float)
			}
			remaining = rem.Text('f', 2)
		}
	}

	resetAt := ""
	if policy.DailyLimitUSD != "" {
		nextReset := startOfDay.Add(24 * time.Hour)
		resetAt = nextReset.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"daily_spent_usd": spent,
		"daily_limit_usd": policy.DailyLimitUSD,
		"remaining_usd":   remaining,
		"reset_at":        resetAt,
	})
}
```

- [ ] **Step 4: Register route in `main.go`**

Add after the existing policy routes (around line 331):

```go
auth.GET("/wallets/:id/daily-spent", walletH.DailySpent)
```

- [ ] **Step 5: Run tests**

Run: `go test ./handler/ -run TestDailySpent -v`
Expected: ALL PASS

- [ ] **Step 6: Run full regression**

Run: `go test ./handler/ -count=1`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add handler/wallet.go main.go handler/policy_usd_test.go
git commit -m "feat: GET /api/wallets/:id/daily-spent — query daily USD spend and remaining limit"
```

---

### Task 5: Update README.md and SKILL.md

**Files:**
- Modify: `README.md`
- Modify: `skill/tee-wallet/SKILL.md`

- [ ] **Step 1: Update README.md**

1. **Smart Contract Security section** (lines 24-29): Replace "3-Layer Model" with "2-Layer Model":
   - (1) Contract whitelist — address admission only
   - (2) All contract operations require Passkey approval via API Key
   - Remove mentions of `allowed_methods`, `auto_approve`, high-risk methods

2. **Contract Calls table** (line 210): Remove `amount_usd` reference. Update descriptions.

3. **Add `/daily-spent` to API Reference** in Approval Policies section.

4. **Security Model section** (lines 240-248): Remove "Contract Whitelist with Method Gates" and "Auto-Approve Mode". Replace with simplified description.

- [ ] **Step 2: Update SKILL.md**

1. Remove all references to `amount_usd` in request examples and rules
2. Remove `allowed_methods` and `auto_approve` from contract whitelist examples
3. Update rules: "all contract operations require approval" instead of high-risk method rules
4. Add `GET /api/wallets/:id/daily-spent` endpoint docs
5. Update whitelist section: include contract address, label, symbol, decimals

- [ ] **Step 3: Verify**

Run: `go vet ./...`
Expected: No issues

- [ ] **Step 4: Commit**

```bash
git add README.md skill/tee-wallet/SKILL.md
git commit -m "docs: update README and SKILL.md for simplified contract security model and daily-spent endpoint"
```

---

### Task 6: Final Verification

- [ ] **Step 1: Run all tests with race detector**

Run: `go test ./handler/ -race -count=1`
Expected: ALL PASS, no races

- [ ] **Step 2: Run full project tests**

Run: `go test ./... -count=1`
Expected: ALL PASS

- [ ] **Step 3: Run go vet**

Run: `go vet ./...`
Expected: No issues
