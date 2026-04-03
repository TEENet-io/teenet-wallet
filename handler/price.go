package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// coinGeckoIDs maps uppercase currency symbols to CoinGecko API identifiers.
var coinGeckoIDs = map[string]string{
	"ETH":  "ethereum",
	"SOL":  "solana",
	"BNB":  "binancecoin",
	"POL":  "matic-network",
	"AVAX": "avalanche-2",
}

// coinGeckoPlatformIDs maps chain names (lowercase) to CoinGecko asset platform IDs
// used by the /simple/token_price/{platform} endpoint.
// coinGeckoPlatformIDs maps chain names (lowercase) to CoinGecko asset platform IDs.
// Used by GetTokenUSDPrice for the /simple/token_price/{platform} endpoint.
// Testnets are intentionally omitted — fail-closed (unknown token → approval).
// Full list: https://api.coingecko.com/api/v3/asset_platforms
var coinGeckoPlatformIDs = map[string]string{
	"ethereum":  "ethereum",
	"optimism":  "optimistic-ethereum",
	"arbitrum":  "arbitrum-one",
	"base":      "base",
	"polygon":   "polygon-pos",
	"bsc":       "binance-smart-chain",
	"avalanche": "avalanche",
	"fantom":    "fantom",
	"linea":     "linea",
	"zksync":    "zksync",
	"scroll":    "scroll",
	"mantle":    "mantle",
	"celo":      "celo",
	"gnosis":    "xdai",
	"cronos":    "cronos",
	"moonbeam":  "moonbeam",
	"blast":     "blast",
	"solana":    "solana",
}

// evmPlatforms tracks which CoinGecko platform IDs use hex addresses (case-insensitive).
// Non-EVM platforms (e.g. Solana) use case-sensitive addresses (base58).
var evmPlatforms = map[string]bool{
	"ethereum":            true,
	"optimistic-ethereum": true,
	"arbitrum-one":        true,
	"base":                true,
	"polygon-pos":         true,
	"binance-smart-chain": true,
	"avalanche":           true,
	"fantom":              true,
	"linea":               true,
	"zksync":              true,
	"scroll":              true,
	"mantle":              true,
	"celo":                true,
	"xdai":                true,
	"cronos":              true,
	"moonbeam":            true,
	"blast":               true,
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
// ERC-20 tokens are priced via CoinGecko Token Price API.
// Solana SPL tokens fall back to Jupiter Price API when CoinGecko has no data.
// Unknown tokens fall back to symbol-based lookup via CoinGecko coin list.
type PriceService struct {
	mu             sync.RWMutex
	prices         map[string]float64 // "ETH" -> 3500.0
	lastUpdate     time.Time
	ttl            time.Duration
	client         *http.Client
	baseURL        string             // CoinGecko API base URL (overridable for testing)
	jupiterBaseURL string             // Jupiter Price API base URL (overridable for testing)
	tokenPrices    map[string]float64 // "ethereum:0xabc..." or "jupiter:MintAddr" -> price
	tokenExpiry    map[string]time.Time

	// Symbol-based fallback: symbol (uppercase) → CoinGecko coin ID.
	// Built from /coins/list, cached for 24 hours.
	symbolMap       map[string]string // "LINK" -> "chainlink"
	symbolMapExpiry time.Time

	refreshMu   sync.Mutex // serialises refresh() calls
	symbolMapMu sync.Mutex // serialises ensureSymbolMap() calls
}

// NewPriceService creates a PriceService with the given cache TTL.
func NewPriceService(ttl time.Duration) *PriceService {
	return NewPriceServiceWithBaseURL(ttl, "https://api.coingecko.com")
}

// NewPriceServiceWithBaseURL creates a PriceService with a custom API base URL (for testing).
func NewPriceServiceWithBaseURL(ttl time.Duration, baseURL string) *PriceService {
	ps := &PriceService{
		prices:         make(map[string]float64),
		ttl:            ttl,
		client:         &http.Client{Timeout: 10 * time.Second},
		baseURL:        baseURL,
		jupiterBaseURL: "https://api.jup.ag",
		tokenPrices:    make(map[string]float64),
		tokenExpiry:    make(map[string]time.Time),
	}
	ps.refresh()
	return ps
}

// SetJupiterBaseURL overrides the Jupiter API base URL (for testing).
func (ps *PriceService) SetJupiterBaseURL(url string) { ps.jupiterBaseURL = url }

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

// GetTokenUSDPrice fetches the USD price for a token identified by its
// contract/mint address on the given chain. Results are cached for the service TTL.
// Supported chains: "ethereum", "optimism", "solana".
// EVM addresses are lowercased; Solana mint addresses preserve original case (base58).
func (ps *PriceService) GetTokenUSDPrice(chainName, contractAddress string) (float64, error) {
	platform, ok := coinGeckoPlatformIDs[strings.ToLower(chainName)]
	if !ok {
		return 0, fmt.Errorf("no token price support for chain %q", chainName)
	}
	addr := strings.TrimSpace(contractAddress)
	if evmPlatforms[platform] {
		addr = strings.ToLower(addr)
	}
	cacheKey := platform + ":" + addr

	ps.mu.RLock()
	if price, ok := ps.tokenPrices[cacheKey]; ok {
		if time.Now().Before(ps.tokenExpiry[cacheKey]) {
			ps.mu.RUnlock()
			return price, nil
		}
	}
	ps.mu.RUnlock()

	url := fmt.Sprintf("%s/api/v3/simple/token_price/%s?contract_addresses=%s&vs_currencies=usd", ps.baseURL, platform, addr)
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
	ps.evictExpiredTokenPrices()
	ps.tokenPrices[cacheKey] = usd
	ps.tokenExpiry[cacheKey] = time.Now().Add(ps.ttl)
	ps.mu.Unlock()

	return usd, nil
}

// GetJupiterPrice fetches the USD price of a Solana SPL token via the Jupiter Price API.
// Uses the same per-entry TTL cache as GetTokenUSDPrice (keyed as "jupiter:{mintAddr}").
// Jupiter covers virtually all SPL tokens with liquidity on Solana DEXes.
func (ps *PriceService) GetJupiterPrice(mintAddress string) (float64, error) {
	addr := strings.TrimSpace(mintAddress)
	cacheKey := "jupiter:" + addr

	ps.mu.RLock()
	if price, ok := ps.tokenPrices[cacheKey]; ok {
		if time.Now().Before(ps.tokenExpiry[cacheKey]) {
			ps.mu.RUnlock()
			return price, nil
		}
	}
	ps.mu.RUnlock()

	url := ps.jupiterBaseURL + "/price/v2?ids=" + addr
	resp, err := ps.client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("jupiter price fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("jupiter price API returned %d", resp.StatusCode)
	}

	// Response: {"data":{"MintAddr":{"id":"MintAddr","price":"1.23"}}}
	var result struct {
		Data map[string]struct {
			Price string `json:"price"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("jupiter price parse failed: %w", err)
	}

	entry, exists := result.Data[addr]
	if !exists || entry.Price == "" {
		return 0, fmt.Errorf("no jupiter price data for %s", addr)
	}
	usd, err := strconv.ParseFloat(entry.Price, 64)
	if err != nil || usd <= 0 {
		return 0, fmt.Errorf("invalid jupiter price for %s: %q", addr, entry.Price)
	}

	ps.mu.Lock()
	ps.tokenPrices[cacheKey] = usd
	ps.tokenExpiry[cacheKey] = time.Now().Add(ps.ttl)
	ps.mu.Unlock()

	return usd, nil
}

// GetPriceBySymbol looks up a token's USD price by its symbol (e.g., "LINK", "UNI").
// This is the last-resort fallback for testnet tokens whose contract addresses aren't
// recognized by CoinGecko. It uses a cached symbol → CoinGecko coin ID map built from
// the /coins/list endpoint (refreshed every 24 hours).
func (ps *PriceService) GetPriceBySymbol(symbol string) (float64, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return 0, fmt.Errorf("empty symbol")
	}
	if stablecoins[symbol] {
		return 1.0, nil
	}
	// Check native currencies first.
	if _, native := coinGeckoIDs[symbol]; native {
		return ps.GetUSDPrice(symbol)
	}

	// Ensure the symbol map is loaded.
	ps.ensureSymbolMap()

	ps.mu.RLock()
	coinID, ok := ps.symbolMap[symbol]
	ps.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("no CoinGecko coin ID for symbol %q", symbol)
	}

	// Check cache.
	cacheKey := "symbol:" + coinID
	ps.mu.RLock()
	if price, cached := ps.tokenPrices[cacheKey]; cached {
		if time.Now().Before(ps.tokenExpiry[cacheKey]) {
			ps.mu.RUnlock()
			return price, nil
		}
	}
	ps.mu.RUnlock()

	// Fetch price by coin ID.
	url := ps.baseURL + "/api/v3/simple/price?ids=" + coinID + "&vs_currencies=usd"
	resp, err := ps.client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("symbol price fetch failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("symbol price API returned %d", resp.StatusCode)
	}
	var result map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("symbol price parse failed: %w", err)
	}
	data, exists := result[coinID]
	if !exists {
		return 0, fmt.Errorf("no price data for coin %s", coinID)
	}
	usd, ok := data["usd"]
	if !ok || usd <= 0 {
		return 0, fmt.Errorf("invalid USD price for coin %s", coinID)
	}

	ps.mu.Lock()
	ps.tokenPrices[cacheKey] = usd
	ps.tokenExpiry[cacheKey] = time.Now().Add(ps.ttl)
	ps.mu.Unlock()

	return usd, nil
}

// ensureSymbolMap loads the CoinGecko coin list if the cache is expired or empty.
// The list maps uppercase token symbols to CoinGecko coin IDs (e.g., "LINK" → "chainlink").
// When multiple coins share a symbol, the first result (typically highest market cap) wins.
func (ps *PriceService) ensureSymbolMap() {
	ps.symbolMapMu.Lock()
	defer ps.symbolMapMu.Unlock()

	ps.mu.RLock()
	if ps.symbolMap != nil && time.Now().Before(ps.symbolMapExpiry) {
		ps.mu.RUnlock()
		return
	}
	ps.mu.RUnlock()

	url := ps.baseURL + "/api/v3/coins/list"
	resp, err := ps.client.Get(url)
	if err != nil {
		slog.Warn("CoinGecko coins/list fetch failed", "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("CoinGecko coins/list returned non-200", "status", resp.StatusCode)
		return
	}

	var coins []struct {
		ID     string `json:"id"`
		Symbol string `json:"symbol"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&coins); err != nil {
		slog.Warn("CoinGecko coins/list parse failed", "error", err)
		return
	}

	m := make(map[string]string, len(coins))
	for _, c := range coins {
		sym := strings.ToUpper(c.Symbol)
		if _, exists := m[sym]; !exists {
			m[sym] = c.ID
		}
	}

	ps.mu.Lock()
	ps.symbolMap = m
	ps.symbolMapExpiry = time.Now().Add(24 * time.Hour)
	ps.mu.Unlock()

	slog.Info("symbol map loaded", "coins", len(m))
}

// evictExpiredTokenPrices removes expired entries from tokenPrices/tokenExpiry when
// the map exceeds 10 000 entries. Caller must hold ps.mu (write lock).
func (ps *PriceService) evictExpiredTokenPrices() {
	if len(ps.tokenPrices) <= 10000 {
		return
	}
	now := time.Now()
	for k, exp := range ps.tokenExpiry {
		if now.After(exp) {
			delete(ps.tokenPrices, k)
			delete(ps.tokenExpiry, k)
		}
	}
}

// refresh fetches fresh prices from CoinGecko.
// Only one goroutine refreshes at a time; concurrent callers block and share the result.
func (ps *PriceService) refresh() {
	ps.refreshMu.Lock()
	defer ps.refreshMu.Unlock()
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
