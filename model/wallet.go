package model

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CustomChain persists user-added EVM chains across restarts.
type CustomChain struct {
	ID       uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Name     string `json:"name" gorm:"size:50;uniqueIndex"`
	Label    string `json:"label" gorm:"size:100"`
	Currency string `json:"currency" gorm:"size:10"`
	Family   string `json:"family" gorm:"size:10"` // "evm" only for now
	RPCURL   string `json:"rpc_url" gorm:"size:500"`
	ChainID  uint64 `json:"chain_id"` // EVM chain ID for replay protection
}

// Wallet represents a chain wallet backed by a TEE-DAO key pair.
// The private key never exists outside TEE hardware.
type Wallet struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	UserID    uint      `json:"user_id" gorm:"not null;index"`
	Chain     string    `json:"chain" gorm:"size:32;not null"`
	KeyName   string    `json:"key_name" gorm:"not null;uniqueIndex"` // TEE-DAO key name
	PublicKey string    `json:"public_key"`                           // hex-encoded compressed pubkey
	Address   string    `json:"address" gorm:"size:100;index"`        // chain address
	Label     string    `json:"label" gorm:"size:100"`
	Curve     string    `json:"curve"`    // secp256k1, ed25519
	Protocol  string    `json:"protocol"` // ecdsa, schnorr
	Status    string    `json:"status" gorm:"default:'creating'"` // creating, ready, error
	CreatedAt time.Time `json:"created_at"`
}

// BeforeCreate generates a UUID v4 for the wallet ID so IDs are not sequential.
func (w *Wallet) BeforeCreate(tx *gorm.DB) error {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	return nil
}

// ChainConfig describes one blockchain network.
type ChainConfig struct {
	Name     string `json:"name"`     // unique key, e.g. "sepolia"
	Label    string `json:"label"`    // display name, e.g. "Sepolia Testnet"
	Protocol string `json:"protocol"` // "ecdsa" | "schnorr"
	Curve    string `json:"curve"`    // "secp256k1" | "ed25519"
	Currency string `json:"currency"` // e.g. "ETH", "SOL"
	Family   string `json:"family"`   // "evm" | "solana"
	RPCURL   string `json:"rpc_url"`  // JSON-RPC endpoint
	ChainID  uint64 `json:"chain_id"` // EVM chain ID (0 for non-EVM)
	Custom   bool   `json:"custom"`   // true if user-added at runtime
}

// Chains is the active chain registry, loaded at startup.
// Use GetChain/SetChain/DeleteChain/GetAllChains for concurrent access.
var (
	chainsMu sync.RWMutex
	Chains   map[string]ChainConfig
)

// GetChain returns the ChainConfig for the given name and whether it exists.
func GetChain(name string) (ChainConfig, bool) {
	chainsMu.RLock()
	defer chainsMu.RUnlock()
	c, ok := Chains[name]
	return c, ok
}

// SetChain sets a ChainConfig in the registry.
func SetChain(name string, cfg ChainConfig) {
	chainsMu.Lock()
	defer chainsMu.Unlock()
	Chains[name] = cfg
}

// DeleteChain removes a chain from the registry.
func DeleteChain(name string) {
	chainsMu.Lock()
	defer chainsMu.Unlock()
	delete(Chains, name)
}

// GetAllChains returns a snapshot copy of the chain registry.
func GetAllChains() map[string]ChainConfig {
	chainsMu.RLock()
	defer chainsMu.RUnlock()
	cp := make(map[string]ChainConfig, len(Chains))
	for k, v := range Chains {
		cp[k] = v
	}
	return cp
}

// ChainsLen returns the number of chains in the registry.
func ChainsLen() int {
	chainsMu.RLock()
	defer chainsMu.RUnlock()
	return len(Chains)
}

// defaultChains is the fallback when no chains.json is present.
var defaultChains = []ChainConfig{
	{Name: "ethereum", Label: "Ethereum Mainnet", Protocol: "ecdsa", Curve: "secp256k1", Currency: "ETH", Family: "evm", RPCURL: "https://ethereum-rpc.publicnode.com"},
	{Name: "solana", Label: "Solana Mainnet", Protocol: "schnorr", Curve: "ed25519", Currency: "SOL", Family: "solana", RPCURL: "https://api.mainnet-beta.solana.com"},
	{Name: "sepolia", Label: "Sepolia Testnet", Protocol: "ecdsa", Curve: "secp256k1", Currency: "ETH", Family: "evm", RPCURL: "https://ethereum-sepolia-rpc.publicnode.com"},
	{Name: "holesky", Label: "Holesky Testnet", Protocol: "ecdsa", Curve: "secp256k1", Currency: "ETH", Family: "evm", RPCURL: "https://ethereum-holesky-rpc.publicnode.com"},
	{Name: "bsc-testnet", Label: "BSC Testnet", Protocol: "ecdsa", Curve: "secp256k1", Currency: "tBNB", Family: "evm", RPCURL: "https://bsc-testnet-rpc.publicnode.com"},
	{Name: "solana-devnet", Label: "Solana Devnet", Protocol: "schnorr", Curve: "ed25519", Currency: "SOL", Family: "solana", RPCURL: "https://api.devnet.solana.com"},
	{Name: "optimism", Label: "Optimism Mainnet", Protocol: "ecdsa", Curve: "secp256k1", Currency: "ETH", Family: "evm", RPCURL: "https://optimism-rpc.publicnode.com"},
	{Name: "base-sepolia", Label: "Base Sepolia Testnet", Protocol: "ecdsa", Curve: "secp256k1", Currency: "ETH", Family: "evm", RPCURL: "https://base-sepolia-rpc.publicnode.com"},
}

// LoadChains loads chain configuration from a JSON file.
// Falls back to built-in defaults if the file does not exist or cannot be parsed.
func LoadChains(path string) {
	Chains = make(map[string]ChainConfig)
	data, err := os.ReadFile(path)
	if err != nil {
		useDefaultChains()
		slog.Info("using built-in default chains", "count", len(Chains))
		return
	}
	var list []ChainConfig
	if err := json.Unmarshal(data, &list); err != nil {
		slog.Warn("chains file parse failed, using defaults", "path", path, "error", err)
		useDefaultChains()
		return
	}
	for _, c := range list {
		Chains[c.Name] = c
	}
	slog.Info("chains loaded", "count", len(Chains), "path", path)
}

func useDefaultChains() {
	for _, c := range defaultChains {
		Chains[c.Name] = c
	}
}

// LoadCustomChains reads all CustomChain rows from the DB and merges them into
// the Chains registry. Built-in chain names are never overwritten.
func LoadCustomChains(db *gorm.DB) {
	var rows []CustomChain
	if err := db.Find(&rows).Error; err != nil {
		slog.Warn("failed to load custom chains from db", "error", err)
		return
	}
	added := 0
	for _, row := range rows {
		if _, exists := Chains[row.Name]; exists {
			// Built-in chain — skip silently.
			continue
		}
		Chains[row.Name] = ChainConfig{
			Name:     row.Name,
			Label:    row.Label,
			Protocol: "ecdsa",
			Curve:    "secp256k1",
			Currency: row.Currency,
			Family:   row.Family,
			RPCURL:   row.RPCURL,
			ChainID:  row.ChainID,
			Custom:   true,
		}
		added++
	}
	if added > 0 {
		slog.Info("custom chains loaded from db", "count", added)
	}
}
