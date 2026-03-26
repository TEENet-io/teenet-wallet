package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TEENet-io/teenet-wallet/handler"
)

func fakeCoinGeckoServer(ethPrice, solPrice float64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]map[string]float64{
			"ethereum":     {"usd": ethPrice},
			"solana":       {"usd": solPrice},
			"binancecoin":  {"usd": 600},
			"matic-network": {"usd": 0.45},
			"avalanche-2":  {"usd": 35},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func newTestPriceService(serverURL string) *handler.PriceService {
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, serverURL)
	return ps
}

func TestPriceService_Stablecoins(t *testing.T) {
	srv := fakeCoinGeckoServer(3500, 150)
	defer srv.Close()
	ps := newTestPriceService(srv.URL)

	for _, coin := range []string{"USDT", "USDC", "DAI", "BUSD"} {
		price, err := ps.GetUSDPrice(coin)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", coin, err)
		}
		if price != 1.0 {
			t.Errorf("%s: expected 1.0, got %f", coin, price)
		}
	}
}

func TestPriceService_ETH(t *testing.T) {
	srv := fakeCoinGeckoServer(3500.50, 150)
	defer srv.Close()
	ps := newTestPriceService(srv.URL)

	price, err := ps.GetUSDPrice("ETH")
	if err != nil {
		t.Fatalf("ETH: unexpected error: %v", err)
	}
	if price != 3500.50 {
		t.Errorf("ETH: expected 3500.50, got %f", price)
	}
}

func TestPriceService_SOL(t *testing.T) {
	srv := fakeCoinGeckoServer(3500, 149.99)
	defer srv.Close()
	ps := newTestPriceService(srv.URL)

	price, err := ps.GetUSDPrice("SOL")
	if err != nil {
		t.Fatalf("SOL: unexpected error: %v", err)
	}
	if price != 149.99 {
		t.Errorf("SOL: expected 149.99, got %f", price)
	}
}

func TestPriceService_BNB(t *testing.T) {
	srv := fakeCoinGeckoServer(3500, 150)
	defer srv.Close()
	ps := newTestPriceService(srv.URL)

	price, err := ps.GetUSDPrice("BNB")
	if err != nil {
		t.Fatalf("BNB: unexpected error: %v", err)
	}
	if price != 600 {
		t.Errorf("BNB: expected 600, got %f", price)
	}
}

func TestPriceService_POL(t *testing.T) {
	srv := fakeCoinGeckoServer(3500, 150)
	defer srv.Close()
	ps := newTestPriceService(srv.URL)

	price, err := ps.GetUSDPrice("POL")
	if err != nil {
		t.Fatalf("POL: unexpected error: %v", err)
	}
	if price != 0.45 {
		t.Errorf("POL: expected 0.45, got %f", price)
	}
}

func TestPriceService_AVAX(t *testing.T) {
	srv := fakeCoinGeckoServer(3500, 150)
	defer srv.Close()
	ps := newTestPriceService(srv.URL)

	price, err := ps.GetUSDPrice("AVAX")
	if err != nil {
		t.Fatalf("AVAX: unexpected error: %v", err)
	}
	if price != 35 {
		t.Errorf("AVAX: expected 35, got %f", price)
	}
}

func TestPriceService_Unsupported(t *testing.T) {
	srv := fakeCoinGeckoServer(3500, 150)
	defer srv.Close()
	ps := newTestPriceService(srv.URL)

	_, err := ps.GetUSDPrice("DOGE")
	if err == nil {
		t.Error("expected error for unsupported currency, got nil")
	}
}

func TestPriceService_CaseInsensitive(t *testing.T) {
	srv := fakeCoinGeckoServer(3500, 150)
	defer srv.Close()
	ps := newTestPriceService(srv.URL)

	price, err := ps.GetUSDPrice("eth")
	if err != nil {
		t.Fatalf("lowercase eth: unexpected error: %v", err)
	}
	if price != 3500 {
		t.Errorf("expected 3500, got %f", price)
	}

	price, err = ps.GetUSDPrice("usdc")
	if err != nil {
		t.Fatalf("lowercase usdc: unexpected error: %v", err)
	}
	if price != 1.0 {
		t.Errorf("expected 1.0, got %f", price)
	}
}

func TestPriceService_GetAllPrices(t *testing.T) {
	srv := fakeCoinGeckoServer(3500, 150)
	defer srv.Close()
	ps := newTestPriceService(srv.URL)

	prices := ps.GetAllPrices()
	if prices["ETH"] != 3500 {
		t.Errorf("ETH: expected 3500, got %f", prices["ETH"])
	}
	if prices["SOL"] != 150 {
		t.Errorf("SOL: expected 150, got %f", prices["SOL"])
	}
	if prices["USDT"] != 1.0 {
		t.Errorf("USDT: expected 1.0, got %f", prices["USDT"])
	}
	if prices["USDC"] != 1.0 {
		t.Errorf("USDC: expected 1.0, got %f", prices["USDC"])
	}
	if prices["BNB"] != 600 {
		t.Errorf("BNB: expected 600, got %f", prices["BNB"])
	}
	if prices["POL"] != 0.45 {
		t.Errorf("POL: expected 0.45, got %f", prices["POL"])
	}
	if prices["AVAX"] != 35 {
		t.Errorf("AVAX: expected 35, got %f", prices["AVAX"])
	}
}

func TestPriceService_ServerDown_ReturnsStale(t *testing.T) {
	srv := fakeCoinGeckoServer(3500, 150)
	ps := newTestPriceService(srv.URL)

	// First fetch works
	price, err := ps.GetUSDPrice("ETH")
	if err != nil || price != 3500 {
		t.Fatalf("initial fetch: price=%f, err=%v", price, err)
	}

	// Shut down server
	srv.Close()

	// Should still return cached value
	price, err = ps.GetUSDPrice("ETH")
	if err != nil {
		t.Fatalf("after shutdown: unexpected error: %v", err)
	}
	if price != 3500 {
		t.Errorf("after shutdown: expected stale 3500, got %f", price)
	}
}

// fakeCoinGeckoTokenServer serves both /api/v3/simple/price (for constructor
// refresh) and /api/v3/simple/token_price/<platform> (for GetTokenUSDPrice).
// tokenResponses maps contract/mint address to its USD price.
// Keys should match the format sent by GetTokenUSDPrice: lowercase hex for EVM, original case for Solana.
func fakeCoinGeckoTokenServer(tokenResponses map[string]float64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/api/v3/simple/token_price/") {
			addr := r.URL.Query().Get("contract_addresses")
			// Try exact match first (Solana base58), then lowercase (EVM hex).
			price, ok := tokenResponses[addr]
			if !ok {
				price, ok = tokenResponses[strings.ToLower(addr)]
			}
			if !ok {
				json.NewEncoder(w).Encode(map[string]map[string]float64{})
				return
			}
			resp := map[string]map[string]float64{
				addr: {"usd": price},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Default: simple/price endpoint for ETH/SOL (used by constructor refresh).
		resp := map[string]map[string]float64{
			"ethereum": {"usd": 3500},
			"solana":   {"usd": 150},
		}
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestGetTokenUSDPrice_Success(t *testing.T) {
	const tokenAddr = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48" // USDC on Ethereum
	srv := fakeCoinGeckoTokenServer(map[string]float64{
		strings.ToLower(tokenAddr): 1.0002,
	})
	defer srv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)

	price, err := ps.GetTokenUSDPrice("ethereum", tokenAddr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 1.0002 {
		t.Errorf("expected 1.0002, got %f", price)
	}
}

func TestGetTokenUSDPrice_CacheHit(t *testing.T) {
	const tokenAddr = "0xdac17f958d2ee523a2206206994597c13d831ec7" // USDT
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/api/v3/simple/token_price/") {
			callCount++
			addr := strings.ToLower(r.URL.Query().Get("contract_addresses"))
			resp := map[string]map[string]float64{addr: {"usd": 0.9999}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		json.NewEncoder(w).Encode(map[string]map[string]float64{
			"ethereum": {"usd": 3500},
			"solana":   {"usd": 150},
		})
	}))
	defer srv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)

	// First call — should hit the server.
	price, err := ps.GetTokenUSDPrice("ethereum", tokenAddr)
	if err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}
	if price != 0.9999 {
		t.Errorf("first call: expected 0.9999, got %f", price)
	}
	if callCount != 1 {
		t.Errorf("expected 1 token API call after first fetch, got %d", callCount)
	}

	// Second call — should be served from cache (callCount stays at 1).
	price, err = ps.GetTokenUSDPrice("ethereum", tokenAddr)
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}
	if price != 0.9999 {
		t.Errorf("second call: expected 0.9999, got %f", price)
	}
	if callCount != 1 {
		t.Errorf("expected cache hit (still 1 token API call), got %d", callCount)
	}
}

func TestGetTokenUSDPrice_TestnetFails(t *testing.T) {
	srv := fakeCoinGeckoTokenServer(nil)
	defer srv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)

	_, err := ps.GetTokenUSDPrice("sepolia", "0xSomeAddr")
	if err == nil {
		t.Error("expected error for unsupported chain, got nil")
	}
}

func TestGetTokenUSDPrice_ArbitrumToken(t *testing.T) {
	const arbUSDC = "0xaf88d065e77c8cc2239327c5edb3a432268e5831" // USDC on Arbitrum
	srv := fakeCoinGeckoTokenServer(map[string]float64{
		arbUSDC: 1.0,
	})
	defer srv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)

	price, err := ps.GetTokenUSDPrice("arbitrum", "0xAF88d065e77c8cC2239327C5EDb3A432268e5831")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 1.0 {
		t.Fatalf("expected 1.0, got %f", price)
	}
}

func TestGetTokenUSDPrice_SolanaMintAddress(t *testing.T) {
	// Solana mint addresses are base58 (case-sensitive) — must NOT be lowercased.
	const mintAddr = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" // USDC on Solana
	srv := fakeCoinGeckoTokenServer(map[string]float64{
		mintAddr: 1.0,
	})
	defer srv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)

	price, err := ps.GetTokenUSDPrice("solana", mintAddr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 1.0 {
		t.Fatalf("expected 1.0, got %f", price)
	}
}

func TestGetTokenUSDPrice_SolanaDevnetFails(t *testing.T) {
	srv := fakeCoinGeckoTokenServer(nil)
	defer srv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)

	_, err := ps.GetTokenUSDPrice("solana-devnet", "SomeDevnetMint123")
	if err == nil {
		t.Fatal("expected error for solana-devnet (not in coinGeckoPlatformIDs)")
	}
}

func TestGetTokenUSDPrice_UnknownToken(t *testing.T) {
	srv := fakeCoinGeckoTokenServer(map[string]float64{}) // no token data
	defer srv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)

	_, err := ps.GetTokenUSDPrice("ethereum", "0xdeadbeef")
	if err == nil {
		t.Error("expected error for unknown token, got nil")
	}
}

// ─── Jupiter Price API tests ─────────────────────────────────────────────────

func fakeJupiterServer(prices map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mintID := r.URL.Query().Get("ids")
		w.Header().Set("Content-Type", "application/json")
		if p, ok := prices[mintID]; ok {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					mintID: map[string]string{"id": mintID, "price": p},
				},
			}
			json.NewEncoder(w).Encode(resp)
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{}})
		}
	}))
}

func TestGetJupiterPrice_Success(t *testing.T) {
	srv := fakeJupiterServer(map[string]string{
		"JUPyiwrYJFskUPiHa7hkeR8VUtAeFoSYbKedZNsDvCN": "0.85",
	})
	defer srv.Close()

	geckoSrv := fakeCoinGeckoServer(3500, 150)
	defer geckoSrv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, geckoSrv.URL)
	ps.SetJupiterBaseURL(srv.URL)

	price, err := ps.GetJupiterPrice("JUPyiwrYJFskUPiHa7hkeR8VUtAeFoSYbKedZNsDvCN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 0.85 {
		t.Fatalf("expected 0.85, got %f", price)
	}
}

func TestGetJupiterPrice_CacheHit(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		mintID := r.URL.Query().Get("ids")
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				mintID: map[string]string{"id": mintID, "price": "1.23"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	geckoSrv := fakeCoinGeckoServer(3500, 150)
	defer geckoSrv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, geckoSrv.URL)
	ps.SetJupiterBaseURL(srv.URL)

	ps.GetJupiterPrice("SomeMint123")
	ps.GetJupiterPrice("SomeMint123")
	if callCount != 1 {
		t.Fatalf("expected 1 Jupiter API call (cached), got %d", callCount)
	}
}

func TestGetJupiterPrice_UnknownMint(t *testing.T) {
	srv := fakeJupiterServer(map[string]string{}) // empty
	defer srv.Close()

	geckoSrv := fakeCoinGeckoServer(3500, 150)
	defer geckoSrv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, geckoSrv.URL)
	ps.SetJupiterBaseURL(srv.URL)

	_, err := ps.GetJupiterPrice("UnknownMintAddress")
	if err == nil {
		t.Fatal("expected error for unknown mint")
	}
}
