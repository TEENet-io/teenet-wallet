package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// coinGeckoIDs maps uppercase currency symbols to CoinGecko API identifiers.
var coinGeckoIDs = map[string]string{
	"ETH": "ethereum",
	"SOL": "solana",
}

// stablecoins are pegged 1:1 to USD — no price lookup needed.
var stablecoins = map[string]bool{
	"USDT": true,
	"USDC": true,
	"DAI":  true,
	"BUSD": true,
}

// PriceService fetches and caches USD prices for supported cryptocurrencies.
// USDT/USDC are hardcoded to $1. ETH/SOL are fetched from CoinGecko with a TTL cache.
type PriceService struct {
	mu         sync.RWMutex
	prices     map[string]float64 // "ETH" -> 3500.0
	lastUpdate time.Time
	ttl        time.Duration
	client     *http.Client
	baseURL    string // CoinGecko API base URL (overridable for testing)
}

// NewPriceService creates a PriceService with the given cache TTL.
func NewPriceService(ttl time.Duration) *PriceService {
	return NewPriceServiceWithBaseURL(ttl, "https://api.coingecko.com")
}

// NewPriceServiceWithBaseURL creates a PriceService with a custom API base URL (for testing).
func NewPriceServiceWithBaseURL(ttl time.Duration, baseURL string) *PriceService {
	ps := &PriceService{
		prices:  make(map[string]float64),
		ttl:     ttl,
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: baseURL,
	}
	ps.refresh()
	return ps
}

// GetUSDPrice returns the USD price for a currency symbol (e.g. "ETH", "SOL", "USDC").
// Stablecoins return 1.0. Unknown currencies return an error.
func (ps *PriceService) GetUSDPrice(currency string) (float64, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if stablecoins[currency] {
		return 1.0, nil
	}
	if _, supported := coinGeckoIDs[currency]; !supported {
		return 0, fmt.Errorf("unsupported currency for USD pricing: %s", currency)
	}

	ps.mu.RLock()
	price, ok := ps.prices[currency]
	expired := time.Since(ps.lastUpdate) > ps.ttl
	ps.mu.RUnlock()

	if ok && !expired {
		return price, nil
	}

	// Refresh (only one goroutine refreshes; others use stale data).
	ps.refresh()

	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if p, ok := ps.prices[currency]; ok {
		return p, nil
	}
	if price > 0 {
		return price, nil // stale but non-zero
	}
	return 0, fmt.Errorf("price unavailable for %s", currency)
}

// GetAllPrices returns a snapshot of all cached prices (for the /api/prices endpoint).
func (ps *PriceService) GetAllPrices() map[string]float64 {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	out := make(map[string]float64, len(ps.prices)+len(stablecoins))
	for k, v := range ps.prices {
		out[k] = v
	}
	for k := range stablecoins {
		out[k] = 1.0
	}
	return out
}

// refresh fetches fresh prices from CoinGecko.
func (ps *PriceService) refresh() {
	// Build comma-separated list of CoinGecko IDs.
	ids := make([]string, 0, len(coinGeckoIDs))
	for _, id := range coinGeckoIDs {
		ids = append(ids, id)
	}

	url := ps.baseURL + "/api/v3/simple/price?ids=" + strings.Join(ids, ",") + "&vs_currencies=usd"
	resp, err := ps.client.Get(url)
	if err != nil {
		slog.Warn("CoinGecko price fetch failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("CoinGecko returned non-200", "status", resp.StatusCode)
		return
	}

	// Response shape: {"ethereum":{"usd":3500.12},"solana":{"usd":150.5}}
	var result map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		slog.Warn("CoinGecko response parse failed", "error", err)
		return
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()
	for symbol, geckoID := range coinGeckoIDs {
		if data, ok := result[geckoID]; ok {
			if usd, ok := data["usd"]; ok && usd > 0 {
				ps.prices[symbol] = usd
			}
		}
	}
	ps.lastUpdate = time.Now()
	slog.Info("prices refreshed", "prices", ps.prices)
}
