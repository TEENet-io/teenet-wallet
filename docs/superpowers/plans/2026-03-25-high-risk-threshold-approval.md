# High-Risk Method Threshold Approval Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace unconditional Passkey approval for high-risk methods with USD threshold-based approval. The `/approve-token` endpoint (structured, known amount) uses token pricing + threshold. The `/contract-call` endpoint (unstructured, can't extract token amount) stays fail-closed for high-risk methods. Remove untrusted `amount_usd` self-reporting.

**Architecture:** Add `GetTokenUSDPrice(chainName, contractAddr)` to PriceService for on-demand ERC-20 price lookups via CoinGecko Token Price API. Modify three code paths in `contract_call.go`: `contractCallEVM` (always fail-closed for high-risk), `contractCallSolana` (always fail-closed), and `executeApprove` (threshold-based with token pricing). Security: fail-closed when system can't determine USD price.

**Tech Stack:** Go, CoinGecko API, Gin, GORM, httptest

**Design doc:** `docs/approval-threshold-design.md`

---

### Task 1: PriceService — Add Token Price Lookup by Contract Address

**Files:**
- Modify: `handler/price.go`
- Test: `handler/price_test.go`

- [ ] **Step 1: Write failing tests for `GetTokenUSDPrice`**

Add to `handler/price_test.go`:

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
			resp := map[string]map[string]float64{"0xaddr": {"usd": 5.0}}
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
	ps.GetTokenUSDPrice("ethereum", "0xaddr")
	ps.GetTokenUSDPrice("ethereum", "0xaddr")
	if callCount != 1 {
		t.Fatalf("expected 1 API call (cached), got %d", callCount)
	}
}

func TestGetTokenUSDPrice_TestnetFails(t *testing.T) {
	srv := fakeCoinGeckoTokenServer(nil)
	defer srv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)
	_, err := ps.GetTokenUSDPrice("sepolia", "0xaddr")
	if err == nil {
		t.Fatal("expected error for testnet chain")
	}
}

func TestGetTokenUSDPrice_UnknownToken(t *testing.T) {
	srv := fakeCoinGeckoTokenServer(map[string]float64{})
	defer srv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)
	_, err := ps.GetTokenUSDPrice("ethereum", "0xdeadbeef")
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./handler/ -run TestGetTokenUSDPrice -v`
Expected: FAIL — `GetTokenUSDPrice` method does not exist

- [ ] **Step 3: Implement `GetTokenUSDPrice` in `handler/price.go`**

Add platform ID mapping (after `coinGeckoIDs`):

```go
var coinGeckoPlatformIDs = map[string]string{
	"ethereum": "ethereum",
	"optimism": "optimistic-ethereum",
}
```

Add fields to `PriceService` struct (after `baseURL`):

```go
	tokenPrices map[string]float64
	tokenExpiry map[string]time.Time
```

Initialize in `NewPriceServiceWithBaseURL` (add to struct literal):

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

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./handler/ -run TestGetTokenUSDPrice -v`
Expected: ALL PASS

- [ ] **Step 5: Run full price test suite for regression**

Run: `go test ./handler/ -run TestPriceService -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add handler/price.go handler/price_test.go
git commit -m "feat: PriceService.GetTokenUSDPrice — ERC-20 price lookup by contract address via CoinGecko"
```

---

### Task 2: Remove `amount_usd` from ContractCallRequest

**Files:**
- Modify: `handler/contract_call.go`
- Modify: `handler/policy_usd_test.go`

- [ ] **Step 1: Remove `AmountUSD` field from struct**

In `handler/contract_call.go`, delete from `ContractCallRequest`:

```go
	AmountUSD string                 `json:"amount_usd"`                  // optional: caller-reported USD value for threshold/daily-limit
```

- [ ] **Step 2: Remove `amount_usd` usage from `contractCallEVM`**

Delete the `req.AmountUSD` block (around lines 224-231):

```go
		if req.AmountUSD != "" {
			if reported, ok := new(big.Float).SetString(req.AmountUSD); ok && reported.Sign() > 0 {
				f, _ := reported.Float64()
				if f > effectiveUSD {
					effectiveUSD = f
				}
			}
		}
```

- [ ] **Step 3: Remove `amount_usd` usage from `contractCallSolana`**

Replace the Solana effectiveUSD block (around lines 438-444):

```go
	var effectiveUSD float64
	if h.prices != nil && req.AmountUSD != "" {
		if reported, ok := new(big.Float).SetString(req.AmountUSD); ok && reported.Sign() > 0 {
			effectiveUSD, _ = reported.Float64()
		}
	}
```

With:

```go
	var effectiveUSD float64
```

- [ ] **Step 4: Remove tests that only test `amount_usd` behavior**

In `handler/policy_usd_test.go`, remove these test functions entirely:
- `TestContractCall_AmountUSD_AboveThreshold_PendingApproval` (line 269)
- `TestContractCall_AmountUSD_DailyLimitExceeded` (line 306)
- `TestContractCall_NativeValueVsAmountUSD_UsesLarger` (line 339)
- `TestContractCall_Solana_AmountUSD_AboveThreshold` (line 374)

Keep `TestContractCall_NoAmountUSD_NoPolicy_PassesThrough` (line 657) — it doesn't use `amount_usd` in the request.

- [ ] **Step 5: Run all handler tests**

Run: `go test ./handler/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add handler/contract_call.go handler/policy_usd_test.go
git commit -m "refactor: remove untrusted amount_usd self-reporting from contract-call"
```

---

### Task 3: Modify `contractCallEVM` — High-Risk Always Fail-Closed via `/contract-call`

The `/contract-call` endpoint cannot reliably extract token amounts from `args` (ABI only describes types, not semantics). High-risk methods via this endpoint are **always fail-closed** — they require approval regardless of price availability. Users who want threshold-based auto-approval for approve operations should use `/approve-token` instead.

**Files:**
- Modify: `handler/contract_call.go` (security decision block, around lines 194-260)
- Test: `handler/contract_call_test.go`

- [ ] **Step 1: Write tests**

Add helper to `handler/contract_call_test.go`:

```go
func contractCallRouterWithPrices(db *gorm.DB, userID uint, authMode, rpcURL, priceURL string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	if rpcURL != "" {
		if cfg, ok := model.Chains["ethereum"]; ok {
			cfg.RPCURL = rpcURL
			model.Chains["ethereum"] = cfg
		}
	}
	r := gin.New()
	h := handler.NewContractCallHandler(db, nil, "http://localhost")
	if priceURL != "" {
		ps := handler.NewPriceServiceWithBaseURL(60*time.Second, priceURL)
		h.SetPriceService(ps)
	}
	injectUser := func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", authMode)
		c.Next()
	}
	r.Use(injectUser)
	r.POST("/wallets/:id/contract-call", h.ContractCall)
	r.POST("/wallets/:id/approve-token", h.ApproveToken)
	r.POST("/wallets/:id/revoke-approval", h.RevokeApproval)
	return r
}
```

Add tests:

```go
// High-risk via /contract-call is ALWAYS fail-closed (can't extract token amount from args)
func TestContractCall_HighRisk_AlwaysFailClosed(t *testing.T) {
	db := testDB(t)
	user, wallet, _ := seedWalletWithContract(t, db, "", true) // AutoApprove ON
	db.Create(&model.ApprovalPolicy{WalletID: wallet.ID, ThresholdUSD: "100", Enabled: true})
	rpc := mockETHRPCServer(t)
	priceSrv := fakeCoinGeckoServer(3500, 150)
	defer priceSrv.Close()

	r := contractCallRouterWithPrices(db, user.ID, "apikey", rpc.URL, priceSrv.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "approve(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "5000000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Even with AutoApprove + PriceService, /contract-call can't extract token amount → fail-closed
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 (fail-closed for high-risk via /contract-call), got %d: %s", w.Code, w.Body.String())
	}
}

// High-risk + AutoApprove=false → 202
func TestContractCall_HighRisk_AutoApproveFalse(t *testing.T) {
	db := testDB(t)
	user, wallet, _ := seedWalletWithContract(t, db, "", false)
	rpc := mockETHRPCServer(t)

	r := contractCallRouter(db, user.ID, "apikey", rpc.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "approve(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 (AutoApprove=false), got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./handler/ -run "TestContractCall_HighRisk_(AlwaysFailClosed|AutoApproveFalse)" -v`
Expected: Both pass (existing behavior already returns 202 for high-risk)

- [ ] **Step 3: Modify `contractCallEVM` high-risk logic**

Update comment on `highRiskMethods` (line 21):

```go
// highRiskMethods are methods that require approval via /contract-call (fail-closed: can't
// extract token amount from args). Use /approve-token for threshold-based auto-approval.
```

Replace the security decision block (lines 200-209):

```go
	if !isPasskeyAuth(c) {
		if highRiskMethods[methodNameLower] {
			needsApproval = true
			approvalReason = fmt.Sprintf("method %q is high-risk; use /approve-token for threshold-based auto-approval", methodName)
		} else if !allowed.AutoApprove {
			needsApproval = true
			approvalReason = "contract does not have auto-approve enabled; passkey approval required"
		}
	}
```

Note: This keeps the same behavior (high-risk → approval) but with an improved message guiding users to `/approve-token`.

- [ ] **Step 4: Run full test suite**

Run: `go test ./handler/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add handler/contract_call.go handler/contract_call_test.go
git commit -m "refactor: high-risk via /contract-call stays fail-closed, improved message points to /approve-token"
```

---

### Task 4: Modify `contractCallSolana` — Fail-Closed (Behavior Unchanged)

**Files:**
- Modify: `handler/contract_call.go` (around lines 425-436)

- [ ] **Step 1: Update comment on `highRiskSOLDiscriminators` (line 30)**

```go
// highRiskSOLDiscriminators maps first-byte hex discriminators for Solana
// SPL Token instructions that always require approval (fail-closed: system cannot
// auto-price Solana tokens yet).
```

- [ ] **Step 2: Run existing Solana tests**

Run: `go test ./handler/ -run "Solana" -v`
Expected: ALL PASS (behavior unchanged)

- [ ] **Step 3: Commit**

```bash
git add handler/contract_call.go
git commit -m "docs: update Solana high-risk comment to clarify fail-closed rationale"
```

---

### Task 5: Modify `executeApprove` — Add Token Pricing and Threshold

This is the key change. `/approve-token` has a structured `amount` field, so the system can compute USD = amount × token_price. If below threshold + AutoApprove → auto-approve. Revoke (amount=0) always auto-approves.

**Files:**
- Modify: `handler/contract_call.go` (lines 599-755)
- Test: `handler/contract_call_test.go`

- [ ] **Step 1: Write failing tests**

```go
// approve-token: AutoApprove + stablecoin below threshold → bypass approval
func TestApproveToken_AutoApprove_Stablecoin_BelowThreshold(t *testing.T) {
	db := testDB(t)
	user, wallet, _ := seedWalletWithContract(t, db, "", true)
	db.Create(&model.ApprovalPolicy{WalletID: wallet.ID, ThresholdUSD: "100", Enabled: true})
	rpc := mockETHRPCServer(t)
	priceSrv := fakeCoinGeckoServer(3500, 150)
	defer priceSrv.Close()

	r := contractCallRouterWithPrices(db, user.ID, "apikey", rpc.URL, priceSrv.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"spender":  "0x1234567890123456789012345678901234567890",
		"amount":   "10", // 10 USDC = $10 < threshold $100
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/approve-token", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should bypass approval. SDK is nil → 502 (signing failed), NOT 202.
	if w.Code == http.StatusAccepted {
		t.Fatalf("expected approval bypass (non-202), got 202: %s", w.Body.String())
	}
}

// approve-token: above threshold → 202
func TestApproveToken_AutoApprove_AboveThreshold(t *testing.T) {
	db := testDB(t)
	user, wallet, _ := seedWalletWithContract(t, db, "", true)
	db.Create(&model.ApprovalPolicy{WalletID: wallet.ID, ThresholdUSD: "100", Enabled: true})
	rpc := mockETHRPCServer(t)
	priceSrv := fakeCoinGeckoServer(3500, 150)
	defer priceSrv.Close()

	r := contractCallRouterWithPrices(db, user.ID, "apikey", rpc.URL, priceSrv.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"spender":  "0x1234567890123456789012345678901234567890",
		"amount":   "500", // 500 USDC = $500 > threshold $100
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/approve-token", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 (above threshold), got %d: %s", w.Code, w.Body.String())
	}
}

// approve-token: unknown token (no price) → fail-closed → 202
func TestApproveToken_UnknownToken_FailClosed(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	// Whitelist a contract with unknown symbol
	db.Create(&model.AllowedContract{
		WalletID: wallet.ID, ContractAddress: "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Symbol: "UNKNOWN", Decimals: 18, AutoApprove: true,
	})
	db.Create(&model.ApprovalPolicy{WalletID: wallet.ID, ThresholdUSD: "100", Enabled: true})
	rpc := mockETHRPCServer(t)
	// Token server returns nothing for unknown address
	priceSrv := fakeCoinGeckoTokenServer(map[string]float64{})
	defer priceSrv.Close()

	r := contractCallRouterWithPrices(db, user.ID, "apikey", rpc.URL, priceSrv.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"spender":  "0x1234567890123456789012345678901234567890",
		"amount":   "10",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/approve-token", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 (fail-closed for unknown token), got %d: %s", w.Code, w.Body.String())
	}
}

// revoke-approval: AutoApprove + amount=0 → always bypass (even without PriceService)
func TestRevokeApproval_AutoApprove_AlwaysBypasses(t *testing.T) {
	db := testDB(t)
	user, wallet, _ := seedWalletWithContract(t, db, "", true)
	rpc := mockETHRPCServer(t)

	r := contractCallRouterWithPrices(db, user.ID, "apikey", rpc.URL, "")
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"spender":  "0x1234567890123456789012345678901234567890",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/revoke-approval", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusAccepted {
		t.Fatalf("expected approval bypass for revoke (non-202), got 202")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./handler/ -run "TestApproveToken_AutoApprove|TestRevokeApproval_AutoApprove|TestApproveToken_UnknownToken" -v`
Expected: FAIL (currently always returns 202 for API key)

- [ ] **Step 3: Implement threshold logic in `executeApprove`**

In `handler/contract_call.go`, restructure `executeApprove` (lines 677-722). Replace the unconditional API key approval with threshold-based logic.

Update doc comments:

```go
// ApproveToken approves ERC-20 token spending.
// API Key: auto-approves if AutoApprove=true AND system can price the token AND amount
// is below threshold. Revoke (amount=0) auto-approves if AutoApprove=true.
// Fail-closed: unknown token price → requires approval.
// Passkey: always executes directly.
```

After building `txContext` (line 686) and before the current unconditional approval block (line 688), insert new logic:

```go
	needsApproval := false
	var approvalReason string
	var effectiveUSD float64

	if !isPasskeyAuth(c) {
		if !allowed.AutoApprove {
			needsApproval = true
			approvalReason = "contract does not have auto-approve enabled"
		} else {
			systemPriceKnown := false
			if amount == "0" {
				systemPriceKnown = true // revoke = $0
			} else if h.prices != nil {
				if amtFloat, ok2 := new(big.Float).SetString(amount); ok2 && amtFloat.Sign() > 0 {
					f, _ := amtFloat.Float64()
					if usdPrice, err := h.prices.GetUSDPrice(allowed.Symbol); err == nil && usdPrice > 0 {
						effectiveUSD = f * usdPrice
						systemPriceKnown = true
					} else if usdPrice, err := h.prices.GetTokenUSDPrice(chainCfg.Name, contractAddr); err == nil && usdPrice > 0 {
						effectiveUSD = f * usdPrice
						systemPriceKnown = true
					}
				}
			}

			if !systemPriceKnown {
				needsApproval = true
				approvalReason = "token price unavailable; approval required (fail-closed)"
			} else {
				var policy model.ApprovalPolicy
				if h.db.Where("wallet_id = ? AND enabled = ?", wallet.ID, true).First(&policy).Error == nil {
					if exceedsUSDThreshold(effectiveUSD, policy.ThresholdUSD) {
						needsApproval = true
						approvalReason = fmt.Sprintf("amount ~$%.2f USD exceeds threshold $%s USD", effectiveUSD, policy.ThresholdUSD)
					}
				}
			}
		}
	}

	if effectiveUSD > 0 {
		txContext["amount_usd"] = fmt.Sprintf("%.2f", effectiveUSD)
	}

	if needsApproval {
		txContextJSON, _ := json.Marshal(txContext)
		userID := mustUserID(c)
		if c.IsAborted() {
			return
		}
		approval := model.ApprovalRequest{
			WalletID:     wallet.ID,
			UserID:       userID,
			ApprovalType: "contract_call",
			Message:      hex.EncodeToString(signingMsg),
			TxContext:    string(txContextJSON),
			TxParams:     string(txParamsJSON),
			Status:       "pending",
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(h.approvalExpiry),
		}
		if err := h.db.Create(&approval).Error; err != nil {
			jsonError(c, http.StatusInternalServerError, "create approval request failed")
			return
		}
		approvalURL := fmt.Sprintf("%s/#/approve/%d", requestBaseURL(c, h.baseURL), approval.ID)
		writeAuditCtx(h.db, c, auditAction, "pending", &wallet.ID, map[string]interface{}{
			"approval_id": approval.ID, "tx_context": txContext,
		})
		c.JSON(http.StatusAccepted, gin.H{
			"status":       "pending_approval",
			"approval_id":  approval.ID,
			"message":      approvalReason,
			"tx_context":   txContext,
			"approval_url": approvalURL,
		})
		return
	}

	// Direct sign + broadcast (Passkey auth, or API key that passed threshold checks).
```

Keep the existing Passkey direct-sign path (current lines 724-755) as-is — it now also serves API keys that passed the threshold check.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./handler/ -run "TestApproveToken|TestRevokeApproval" -v`
Expected: ALL PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./handler/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add handler/contract_call.go handler/contract_call_test.go
git commit -m "feat: approve-token/revoke-approval use threshold-based approval with token USD pricing"
```

---

### Task 6: Final Verification

- [ ] **Step 1: Run all tests with race detector**

Run: `go test ./handler/ -race -v`
Expected: ALL PASS, no races

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: No issues

- [ ] **Step 3: Run full project tests**

Run: `go test ./... -v`
Expected: ALL PASS

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "docs: update high-risk method comments to reflect threshold-based approval design"
```
