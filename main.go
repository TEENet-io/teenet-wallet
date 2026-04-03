package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	sdk "github.com/TEENet-io/teenet-sdk/go"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/TEENet-io/teenet-wallet/handler"
	"github.com/TEENet-io/teenet-wallet/model"
)

func raiseFileLimit() {
	var rlimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
		return
	}
	if rlimit.Cur < 65535 {
		rlimit.Cur = 65535
		if rlimit.Max < 65535 {
			rlimit.Max = 65535
		}
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
			slog.Warn("failed to raise file descriptor limit", "error", err, "current", rlimit.Cur)
		} else {
			slog.Info("raised file descriptor limit", "nofile", rlimit.Cur)
		}
	}
}

func main() {
	slogLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(slogLogger)

	raiseFileLimit()

	consensusURL := envOrDefault("CONSENSUS_URL", "http://localhost:8089")
	host := envOrDefault("HOST", "0.0.0.0")
	port := envOrDefault("PORT", "8080")
	dataDir := envOrDefault("DATA_DIR", "/data")
	baseURL := envOrDefault("BASE_URL", "http://localhost:"+port)
	frontendURL := envOrDefault("FRONTEND_URL", "")
	chainsFile := envOrDefault("CHAINS_FILE", "./chains.json")
	apiKeyRateLimit        := envOrDefaultInt("API_KEY_RATE_LIMIT", 200)       // general: requests per minute per API key
	walletCreateRateLimit  := envOrDefaultInt("WALLET_CREATE_RATE_LIMIT", 5)  // wallet creation is TEE-DKG-bound
	registrationRateLimit  := envOrDefaultInt("REGISTRATION_RATE_LIMIT", 10)  // public auth: prevent TEE DKG abuse
	approvalExpiryMinutes  := envOrDefaultInt("APPROVAL_EXPIRY_MINUTES", 1440)
	maxWalletsPerUser      := envOrDefaultInt("MAX_WALLETS_PER_USER", 10)
	maxAPIKeysPerUser      := envOrDefaultInt("MAX_API_KEYS_PER_USER", 10)
	maxUsers               := envOrDefaultInt("MAX_USERS", 500)
	approvalExpiry         := time.Duration(approvalExpiryMinutes) * time.Minute

	// Load chain configuration.
	model.LoadChains(chainsFile)

	// Init SQLite DB.
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("mkdir data dir: %v", err)
	}
	dbPath := filepath.Join(dataDir, "wallet.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.Exec("PRAGMA journal_mode=WAL")
	sqlDB.Exec("PRAGMA busy_timeout=5000")

	if err := db.AutoMigrate(
		&model.User{},
		&model.APIKey{},
		&model.Wallet{},
		&model.ApprovalPolicy{},
		&model.ApprovalRequest{},
		&model.AllowedContract{},
		&model.AuditLog{},
		&model.IdempotencyRecord{},
		&model.CustomChain{},
		&model.AddressBookEntry{},
	); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// Merge persisted custom chains into the registry now that the DB is ready.
	model.LoadCustomChains(db)

	// Init TEENet SDK.
	opts := &sdk.ClientOptions{
		RequestTimeout:     3 * time.Minute, // ECDSA DKG can take 1-2 min
		PendingWaitTimeout: 3 * time.Minute,
	}
	sdkClient := sdk.NewClientWithOptions(consensusURL, opts)
	if err := sdkClient.SetDefaultAppInstanceIDFromEnv(); err != nil {
		slog.Warn("APP_INSTANCE_ID not set — SDK signing will require explicit app instance ID")
	}
	defer sdkClient.Close()

	sessions := handler.NewSessionStore()
	defer sessions.Stop()
	sseHub := handler.NewSSEHub()

	// Price service for USD threshold conversion.
	priceTTL := time.Duration(envOrDefaultInt("PRICE_CACHE_TTL", 60)) * time.Second
	priceService := handler.NewPriceService(priceTTL)

	// Router.
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20) // 1 MB
		c.Next()
	})
	r.Use(requestIDMiddleware())
	r.Use(corsMiddleware(frontendURL))

	// Content-Security-Policy for non-API routes (web UI).
	r.Use(func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Header("Content-Security-Policy",
				"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		}
		c.Next()
	})

	// Serve frontend.
	r.Static("/assets", "./frontend/assets")
	r.StaticFile("/favicon.ico", "./frontend/favicon.ico")
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(404, gin.H{"error": "api endpoint not found"})
			return
		}
		c.File("./frontend/index.html")
	})

	// Health check (public).
	r.GET("/api/health", func(c *gin.Context) {
		sqlDB, err := db.DB()
		dbOK := err == nil && sqlDB.Ping() == nil
		status := "ok"
		if !dbOK {
			status = "degraded"
		}
		c.JSON(200, gin.H{"status": status, "service": "teenet-wallet", "db": dbOK})
	})

	// Chain list (public) — used by frontend to populate chain selector.
	// The "custom" field in each ChainConfig is true for user-added chains.
	r.GET("/api/chains", func(c *gin.Context) {
		all := model.GetAllChains()
		list := make([]model.ChainConfig, 0, len(all))
		for _, cfg := range all {
			list = append(list, cfg)
		}
		c.JSON(200, gin.H{"success": true, "chains": list})
	})

	// USD prices (public) — used by frontend to display reference prices.
	r.GET("/api/prices", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true, "prices": priceService.GetAllPrices()})
	})

	// Auth handlers (public: login + registration flows only).
	// Registration and login discovery endpoints are IP-rate-limited to prevent
	// an attacker from spamming TEE DKG operations (each takes 1-2 min of cluster compute).
	authH := handler.NewAuthHandler(db, sdkClient, sessions, baseURL)
	authH.SetMaxAPIKeys(maxAPIKeysPerUser)
	authH.SetMaxUsers(maxUsers)
	ipLimiter := handler.NewIPRateLimiter(registrationRateLimit, time.Minute)
	defer ipLimiter.Stop()
	ipRL := handler.IPRateLimitMiddleware(ipLimiter)
	r.GET("/api/auth/check-name", authH.CheckName)
	r.GET("/api/auth/passkey/options", authH.PasskeyOptions)
	r.POST("/api/auth/passkey/verify", authH.PasskeyVerify)
	r.POST("/api/auth/passkey/register/begin", ipRL, authH.PasskeyRegistrationBegin)   // registration only: rate-limited to prevent DKG abuse
	r.GET("/api/auth/passkey/register/options", authH.PasskeyRegistrationOptions)       // legacy: invite-token flow
	r.POST("/api/auth/passkey/register/verify", ipRL, authH.PasskeyRegistrationVerify) // registration only

	// Protected routes (dual auth: API Key or Passkey session).
	rateLimiter       := handler.NewRateLimiter(apiKeyRateLimit, time.Minute)
	defer rateLimiter.Stop()
	walletRateLimiter := handler.NewRateLimiter(walletCreateRateLimit, time.Minute)
	defer walletRateLimiter.Stop()
	auth := r.Group("/api")
	auth.Use(handler.AuthMiddleware(db, sessions))
	auth.Use(handler.CSRFMiddleware())
	auth.Use(handler.APIKeyRateLimitMiddleware(rateLimiter))

	// API Key management + session management (Passkey only).
	passkeyOnly := auth.Group("")
	passkeyOnly.Use(handler.PasskeyOnlyMiddleware())
	passkeyOnly.POST("/auth/invite", authH.InviteUser)          // admin action: Passkey only
	passkeyOnly.DELETE("/auth/session", authH.Logout)           // revoke current session
	passkeyOnly.DELETE("/auth/account", authH.DeleteAccount)    // delete account + all keys
	passkeyOnly.POST("/auth/apikey/generate", authH.GenerateAPIKey)
	passkeyOnly.GET("/auth/apikey/list", authH.ListAPIKeys)
	passkeyOnly.DELETE("/auth/apikey", authH.RevokeAPIKey)
	passkeyOnly.PATCH("/auth/apikey", authH.RenameAPIKey)

	// Custom chain management (Passkey only — structural change to the wallet service).
	passkeyOnly.POST("/chains", func(c *gin.Context) {
		var req struct {
			Name     string `json:"name"`
			Label    string `json:"label"`
			Currency string `json:"currency"`
			RPCURL   string `json:"rpc_url"`
			ChainID  uint64 `json:"chain_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request body"})
			return
		}

		// Validate required fields.
		if req.Name == "" {
			c.JSON(400, gin.H{"error": "name is required"})
			return
		}
		if req.RPCURL == "" {
			c.JSON(400, gin.H{"error": "rpc_url is required"})
			return
		}

		// SSRF protection: validate the RPC URL is not targeting internal networks.
		if err := validateExternalURL(req.RPCURL); err != nil {
			c.JSON(400, gin.H{"error": fmt.Sprintf("invalid rpc_url: %s", err.Error())})
			return
		}

		// Only EVM custom chains are supported for now.
		// (Solana requires different protocol/curve and additional validation.)
		family := "evm"

		// Reject if it would collide with a built-in chain.
		if existing, exists := model.GetChain(req.Name); exists && !existing.Custom {
			c.JSON(409, gin.H{"error": "cannot overwrite a built-in chain"})
			return
		}
		// Reject if a custom chain with this name already exists.
		if existing, exists := model.GetChain(req.Name); exists && existing.Custom {
			c.JSON(409, gin.H{"error": "custom chain already exists"})
			return
		}

		row := model.CustomChain{
			Name:     req.Name,
			Label:    req.Label,
			Currency: req.Currency,
			Family:   family,
			RPCURL:   req.RPCURL,
			ChainID:  req.ChainID,
		}
		if err := db.Create(&row).Error; err != nil {
			slog.Error("create custom chain", "error", err)
			c.JSON(500, gin.H{"error": "failed to persist custom chain"})
			return
		}

		cfg := model.ChainConfig{
			Name:     row.Name,
			Label:    row.Label,
			Protocol: "ecdsa",
			Curve:    "secp256k1",
			Currency: row.Currency,
			Family:   family,
			RPCURL:   row.RPCURL,
			ChainID:  row.ChainID,
			Custom:   true,
		}
		model.SetChain(cfg.Name, cfg)

		slog.Info("custom chain added", "name", cfg.Name, "chain_id", cfg.ChainID)
		c.JSON(201, gin.H{"success": true, "chain": cfg})
	})

	passkeyOnly.DELETE("/chains/:name", func(c *gin.Context) {
		name := c.Param("name")

		existing, exists := model.GetChain(name)
		if !exists {
			c.JSON(404, gin.H{"error": "chain not found"})
			return
		}
		if !existing.Custom {
			c.JSON(403, gin.H{"error": "cannot delete a built-in chain"})
			return
		}

		// Refuse if any wallet is using this chain.
		var count int64
		if err := db.Model(&model.Wallet{}).Where("chain = ?", name).Count(&count).Error; err != nil {
			slog.Error("check wallets on chain", "error", err)
			c.JSON(500, gin.H{"error": "failed to check wallets"})
			return
		}
		if count > 0 {
			c.JSON(409, gin.H{"error": "chain has existing wallets; delete them first"})
			return
		}

		if err := db.Where("name = ?", name).Delete(&model.CustomChain{}).Error; err != nil {
			slog.Error("delete custom chain", "error", err)
			c.JSON(500, gin.H{"error": "failed to delete custom chain"})
			return
		}
		model.DeleteChain(name)

		slog.Info("custom chain deleted", "name", name)
		c.JSON(200, gin.H{"success": true})
	})

	// Contract whitelist (dual-auth for read, Passkey-only for write).
	contractH := handler.NewContractHandler(db, sdkClient, approvalExpiry)
	auth.GET("/wallets/:id/contracts", contractH.ListContracts)
	auth.POST("/wallets/:id/contracts", contractH.AddContract)           // passkey: direct; apikey: pending approval
	auth.PUT("/wallets/:id/contracts/:cid", contractH.UpdateContract)    // passkey: direct; apikey: pending approval
	passkeyOnly.DELETE("/wallets/:id/contracts/:cid", contractH.DeleteContract)

	// Address book (dual-auth for read/add/update, Passkey-only for delete).
	abH := handler.NewAddressBookHandler(db, sdkClient, approvalExpiry)
	auth.GET("/addressbook", abH.ListEntries)
	auth.POST("/addressbook", abH.AddEntry)
	auth.PUT("/addressbook/:id", abH.UpdateEntry)
	passkeyOnly.DELETE("/addressbook/:id", abH.DeleteEntry)

	// Idempotency store for transfer requests.
	idempotencyStore := handler.NewIdempotencyStore(db)
	defer idempotencyStore.Stop()

	// Wallet routes (API Key or Passkey).
	walletH := handler.NewWalletHandler(db, sdkClient, baseURL, approvalExpiry)
	walletH.SetPriceService(priceService)
	walletH.SetMaxWallets(maxWalletsPerUser)
	walletH.SetIdempotencyStore(idempotencyStore)
	auth.POST("/wallets", handler.UserRateLimitMiddleware(walletRateLimiter), walletH.CreateWallet)
	auth.GET("/wallets", walletH.ListWallets)
	auth.GET("/wallets/:id", walletH.GetWallet)
	auth.PATCH("/wallets/:id", walletH.RenameWallet)         // rename: API Key or Passkey
	passkeyOnly.DELETE("/wallets/:id", walletH.DeleteWallet) // irreversible: Passkey only

	// General contract call (API Key or Passkey, with security layers).
	contractCallH := handler.NewContractCallHandler(db, sdkClient, baseURL, approvalExpiry)
	contractCallH.SetPriceService(priceService)
	auth.POST("/wallets/:id/contract-call", contractCallH.ContractCall)
	auth.POST("/wallets/:id/approve-token", contractCallH.ApproveToken)
	auth.POST("/wallets/:id/revoke-approval", contractCallH.RevokeApproval)
	auth.POST("/wallets/:id/transfer", walletH.Transfer) // backend builds+broadcasts tx
	auth.POST("/wallets/:id/wrap-sol", walletH.WrapSOL)
	auth.POST("/wallets/:id/unwrap-sol", walletH.UnwrapSOL)
	auth.GET("/wallets/:id/pubkey", walletH.GetPubkey)
	auth.GET("/wallets/:id/policy", walletH.GetPolicy)        // read: API Key or Passkey
	auth.PUT("/wallets/:id/policy", walletH.SetPolicy)        // passkey: apply directly; API key: creates approval
	passkeyOnly.DELETE("/wallets/:id/policy", walletH.DeletePolicy) // irreversible: Passkey only
	auth.GET("/wallets/:id/daily-spent", walletH.DailySpent)

	// Balance (API Key or Passkey).
	balanceH := handler.NewBalanceHandler(db)
	auth.GET("/wallets/:id/balance", balanceH.GetBalance)

	// Faucet proxy (dual-auth, testnet only).
	faucetURL := envOrDefault("FAUCET_URL", "")
	faucetH := handler.NewFaucetHandler(db, faucetURL)
	auth.POST("/faucet", faucetH.Claim)

	// Audit log routes (dual-auth).
	auditH := handler.NewAuditHandler(db)
	auth.GET("/audit/logs", auditH.ListLogs)

	// Approval routes.
	approvalH := handler.NewApprovalHandler(db, sdkClient, sseHub)
	approvalH.SetPriceService(priceService)
	auth.GET("/approvals/pending", approvalH.ListPending)
	auth.GET("/approvals/:id", approvalH.GetApproval)
	// Approve/reject: Passkey only.
	approveOnly := auth.Group("")
	approveOnly.Use(handler.PasskeyOnlyMiddleware())
	approveOnly.POST("/approvals/:id/approve", approvalH.Approve)
	approveOnly.POST("/approvals/:id/reject", approvalH.Reject)

	// SSE event stream (API Key or Passkey).
	sseH := handler.NewSSEHandler(sseHub)
	auth.GET("/events/stream", sseH.Stream)

	addr := host + ":" + port
	slog.Info("server starting",
		"addr", addr,
		"consensus_url", consensusURL,
		"base_url", baseURL,
		"chains_file", chainsFile,
		"chains_loaded", model.ChainsLen(),
		"approval_expiry_minutes", approvalExpiryMinutes,
		"max_wallets_per_user", maxWalletsPerUser,
		"max_api_keys_per_user", maxAPIKeysPerUser,
		"max_users", maxUsers,
	)

	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("shutdown complete")
}

func envOrDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envOrDefaultInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			b := make([]byte, 8)
			rand.Read(b)
			id = hex.EncodeToString(b)
		}
		c.Set("requestID", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func corsMiddleware(allowedOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if allowedOrigin == "*" {
			c.Header("Access-Control-Allow-Origin", "*")
		} else if allowedOrigin != "" && origin == allowedOrigin {
			c.Header("Access-Control-Allow-Origin", allowedOrigin)
			c.Header("Vary", "Origin")
		}
		// If allowedOrigin == "" (default), no CORS header is set — secure by default.

		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-CSRF-Token,Idempotency-Key")
		if allowedOrigin != "*" {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// validateExternalURL checks that the URL uses http(s) and does not resolve to
// a private/loopback IP address, preventing SSRF attacks.
func validateExternalURL(rawURL string) error {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fmt.Errorf("URL must start with http:// or https://")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("malformed URL")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("localhost is not allowed")
	}
	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("cannot resolve host: %s", host)
	}
	privateRanges := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	var cidrs []*net.IPNet
	for _, cidr := range privateRanges {
		_, network, _ := net.ParseCIDR(cidr)
		cidrs = append(cidrs, network)
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		for _, cidr := range cidrs {
			if cidr.Contains(ip) {
				return fmt.Errorf("private/internal IP address is not allowed")
			}
		}
	}
	return nil
}
