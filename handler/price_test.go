package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TEENet-io/teenet-wallet/handler"
)

func fakeCoinGeckoServer(ethPrice, solPrice float64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]map[string]float64{
			"ethereum": {"usd": ethPrice},
			"solana":   {"usd": solPrice},
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
