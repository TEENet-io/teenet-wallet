// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

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

	"github.com/TEENet-io/teenet-wallet/chain"
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

	host := envOrDefault("HOST", "0.0.0.0")
	port := envOrDefault("PORT", "18080")
	dataDir := envOrDefault("DATA_DIR", "/data")
	baseURL := envOrDefault("BASE_URL", "http://localhost:"+port)
	frontendURL := envOrDefault("FRONTEND_URL", "")
	if frontendURL == "*" {
		slog.Warn("CORS: FRONTEND_URL is set to wildcard '*' — any origin can make cross-origin requests. Restrict to a specific origin in production.")
		if os.Getenv("GIN_MODE") == "release" {
			slog.Error("CORS wildcard '*' is not allowed in production mode. Set FRONTEND_URL to a specific origin.")
			log.Fatal("refusing to start with FRONTEND_URL=* in release mode")
		}
	}
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
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("mkdir data dir: %v", err)
	}
	dbPath := filepath.Join(dataDir, "wallet.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("get underlying sql.DB: %v", err)
	}
	defer sqlDB.Close()
	defer chain.StopNonceCleanup()
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		slog.Warn("failed to set WAL mode", "error", err)
	}
	if _, err := sqlDB.Exec("PRAGMA busy_timeout=5000"); err != nil {
		slog.Warn("failed to set busy_timeout", "error", err)
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		slog.Warn("failed to set foreign_keys", "error", err)
	}
	if _, err := sqlDB.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		slog.Warn("failed to set synchronous", "error", err)
	}
	if _, err := sqlDB.Exec("PRAGMA cache_size=-64000"); err != nil { // 64 MB
		slog.Warn("failed to set cache_size", "error", err)
	}
	if _, err := sqlDB.Exec("PRAGMA temp_store=MEMORY"); err != nil {
		slog.Warn("failed to set temp_store", "error", err)
	}
	if _, err := sqlDB.Exec("PRAGMA mmap_size=268435456"); err != nil { // 256 MB
		slog.Warn("failed to set mmap_size", "error", err)
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.EmailVerification{},
		&model.APIKey{},
		&model.Wallet{},
		&model.ApprovalPolicy{},
		&model.ApprovalRequest{},
		&model.AllowedContract{},
		&model.AuditLog{},
		&model.IdempotencyRecord{},
		&model.AddressBookEntry{},
	); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// Init TEENet SDK.
	sdkClient := sdk.NewClient()
	defer sdkClient.Close()

	// QuickNode RPC overrides.
	//
	// For each chain with a non-empty `quicknode_network` field, rewrite its
	// RPCURL to `https://{endpoint}.{network}.quiknode.pro/{token}/` so we use
	// QuickNode instead of the public fallback. Token source precedence:
	//   1. QUICKNODE_TOKEN_KEY — TEE-backed API key (preferred; mirrors SMTP_PASSWORD_KEY).
	//   2. QUICKNODE_TOKEN    — plain env.
	// When QUICKNODE_ENDPOINT is unset, no overrides run and publicnode defaults stand.
	if qnEndpoint := strings.TrimSpace(os.Getenv("QUICKNODE_ENDPOINT")); qnEndpoint != "" {
		qnToken := os.Getenv("QUICKNODE_TOKEN")
		if keyName := strings.TrimSpace(os.Getenv("QUICKNODE_TOKEN_KEY")); keyName != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			res, err := sdkClient.GetAPIKey(ctx, keyName)
			cancel()
			if err != nil {
				log.Fatalf("QUICKNODE_TOKEN_KEY=%q: fetch from TEE failed: %v", keyName, err)
			}
			if !res.Success {
				log.Fatalf("QUICKNODE_TOKEN_KEY=%q: TEE returned error: %s", keyName, res.Error)
			}
			qnToken = res.APIKey
			slog.Info("QuickNode token loaded from TEE-backed API key store", "key_name", keyName)
		}
		if qnToken == "" {
			slog.Warn("QUICKNODE_ENDPOINT set without QUICKNODE_TOKEN / QUICKNODE_TOKEN_KEY — skipping RPC override")
		} else {
			overridden := 0
			for name, cfg := range model.GetAllChains() {
				if cfg.QuickNodeNetwork == "" {
					continue
				}
				// "-" sentinel = no network subdomain (ethereum mainnet).
				host := qnEndpoint + "." + cfg.QuickNodeNetwork + ".quiknode.pro"
				if cfg.QuickNodeNetwork == "-" {
					host = qnEndpoint + ".quiknode.pro"
				}
				path := strings.TrimPrefix(cfg.QuickNodePath, "/")
				originalURL := cfg.RPCURL
				cfg.RPCURL = fmt.Sprintf("https://%s/%s/%s", host, qnToken, path)
				// Register the original (public) URL as fallback — chain/rpc.go
				// auto-switches to it on transport/HTTP failure.
				chain.SetRPCFallback(cfg.RPCURL, originalURL)
				model.SetChain(name, cfg)
				overridden++
			}
			slog.Info("QuickNode RPC overrides applied", "endpoint", qnEndpoint, "chains", overridden)
		}
	}

	sessions := handler.NewSessionStore()
	defer sessions.Stop()
	sseHub := handler.NewSSEHub()
	defer sseHub.Stop()

	// Price service for USD threshold conversion.
	priceTTL := time.Duration(envOrDefaultInt("PRICE_CACHE_TTL", 60)) * time.Second
	priceService := handler.NewPriceService(priceTTL)
	defer priceService.Stop()

	// Router.
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	// Only trust X-Forwarded-* headers from a local reverse proxy (e.g. nginx on the same host).
	r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20) // 1 MB
		c.Next()
	})
	r.Use(requestIDMiddleware())
	r.Use(corsMiddleware(frontendURL))

	// Cache index.html so we can inject a per-request CSP nonce without
	// hitting disk on every page load. The SPA needs an inline <script> in
	// <head> to redirect bare paths like /instance/abc → /instance/abc/
	// (Vite builds with `base: './'`, so relative asset paths only resolve
	// when the page URL ends with a slash). To keep CSP strict, that inline
	// script is gated by a per-request nonce instead of 'unsafe-inline'.
	var indexHTMLTemplate string
	if data, err := os.ReadFile("./frontend/index.html"); err != nil {
		slog.Warn("failed to read frontend index.html at startup", "error", err)
	} else {
		indexHTMLTemplate = string(data)
	}

	// Content-Security-Policy for non-API routes (web UI). Generates a
	// per-request nonce and exposes it via gin.Context for the SPA handler.
	r.Use(func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			nonceBytes := make([]byte, 16)
			if _, err := rand.Read(nonceBytes); err == nil {
				nonce := hex.EncodeToString(nonceBytes)
				c.Set("cspNonce", nonce)
				c.Header("Content-Security-Policy",
					"default-src 'self'; script-src 'self' 'nonce-"+nonce+"'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
			}
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
		if indexHTMLTemplate == "" {
			c.File("./frontend/index.html")
			return
		}
		nonce, _ := c.Get("cspNonce")
		nonceStr, _ := nonce.(string)
		body := strings.ReplaceAll(indexHTMLTemplate, "__CSP_NONCE__", nonceStr)
		c.Data(200, "text/html; charset=utf-8", []byte(body))
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
	// Strip rpc_url and the quicknode_* fields: when QuickNode overrides are
	// applied, rpc_url embeds the QN token in its path, and this endpoint is
	// unauthenticated. All RPC calls happen server-side; the browser never
	// needs the URL.
	r.GET("/api/chains", func(c *gin.Context) {
		all := model.GetAllChains()
		list := make([]model.ChainConfig, 0, len(all))
		for _, cfg := range all {
			cfg.RPCURL = ""
			cfg.QuickNodeNetwork = ""
			cfg.QuickNodePath = ""
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

	// Email verification service: SMTP if SMTP_HOST is set, otherwise mock mode.
	//
	// Password source precedence:
	//   1. SMTP_PASSWORD_KEY — name of a TEE-backed API key; value fetched via SDK
	//      at startup. Preferred for production; keeps the secret out of docker env.
	//   2. SMTP_PASSWORD — plain env. Useful for local dev; visible to anyone who
	//      can docker inspect or read process env.
	var emailSender handler.EmailSender
	var devFixedCode string
	if smtpHost := strings.TrimSpace(os.Getenv("SMTP_HOST")); smtpHost != "" {
		smtpPassword := os.Getenv("SMTP_PASSWORD")
		if keyName := strings.TrimSpace(os.Getenv("SMTP_PASSWORD_KEY")); keyName != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			res, err := sdkClient.GetAPIKey(ctx, keyName)
			cancel()
			if err != nil {
				log.Fatalf("SMTP_PASSWORD_KEY=%q: fetch from TEE failed: %v", keyName, err)
			}
			if !res.Success {
				log.Fatalf("SMTP_PASSWORD_KEY=%q: TEE returned error: %s", keyName, res.Error)
			}
			smtpPassword = res.APIKey
			slog.Info("SMTP password loaded from TEE-backed API key store", "key_name", keyName)
		}
		emailSender = &handler.SmtpEmailSender{
			Host:     smtpHost,
			Port:     envOrDefault("SMTP_PORT", "587"),
			Username: os.Getenv("SMTP_USERNAME"),
			Password: smtpPassword,
			From:     os.Getenv("SMTP_FROM"),
		}
	} else {
		emailSender = &handler.MockEmailSender{}
		devFixedCode = strings.TrimSpace(os.Getenv("DEV_FIXED_CODE"))
		if devFixedCode == "" {
			devFixedCode = "999999"
		}
	}
	emailSvc := handler.NewEmailVerificationService(db, emailSender, handler.EmailVerificationConfig{
		CodeTTL:        time.Duration(envOrDefaultInt("EMAIL_CODE_TTL", 600)) * time.Second,
		ResendCooldown: time.Duration(envOrDefaultInt("EMAIL_CODE_RESEND_COOLDOWN", 60)) * time.Second,
		MaxAttempts:    envOrDefaultInt("EMAIL_CODE_MAX_ATTEMPTS", 5),
		FixedCode:      devFixedCode,
	})
	authH.SetEmailVerificationService(emailSvc)
	emailH := handler.NewEmailVerificationHandler(emailSvc)
	slog.Info("email verification configured", "mode", emailSender.Mode())
	if devFixedCode != "" {
		slog.Warn("DEV_FIXED_CODE is set: all email verification codes will equal this value — do not set in production", "code", devFixedCode)
	}

	ipLimiter := handler.NewIPRateLimiter(registrationRateLimit, time.Minute)
	defer ipLimiter.Stop()
	ipRL := handler.IPRateLimitMiddleware(ipLimiter)

	// Email verification routes (unauthenticated, IP rate limited).
	r.POST("/api/auth/email/send-code", ipRL, emailH.SendCode)
	r.POST("/api/auth/email/verify-code", ipRL, emailH.VerifyCode)

	r.GET("/api/auth/check-name", authH.CheckName)
	r.GET("/api/auth/passkey/options", authH.PasskeyOptions)
	r.POST("/api/auth/passkey/verify", authH.PasskeyVerify)
	r.POST("/api/auth/passkey/register/begin", ipRL, authH.PasskeyRegistrationBegin)   // registration only: rate-limited to prevent DKG abuse
	r.GET("/api/auth/passkey/register/options", authH.PasskeyRegistrationOptions)       // legacy: invite-token flow
	r.POST("/api/auth/passkey/register/verify", ipRL, authH.PasskeyRegistrationVerify) // registration only

	// Protected routes (dual auth: API Key or Passkey session).
	rateLimiter          := handler.NewRateLimiter(apiKeyRateLimit, time.Minute)
	defer rateLimiter.Stop()
	walletRateLimiter    := handler.NewRateLimiter(walletCreateRateLimit, time.Minute)
	defer walletRateLimiter.Stop()
	transferRateLimiter  := handler.NewRateLimiter(20, time.Minute) // 20 fund-moving ops/min per user
	defer transferRateLimiter.Stop()
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

	// Contract whitelist (dual-auth for read, Passkey-only for write).
	contractH := handler.NewContractHandler(db, sdkClient, baseURL, approvalExpiry)
	auth.GET("/wallets/:id/contracts", contractH.ListContracts)
	auth.POST("/wallets/:id/contracts", contractH.AddContract)           // passkey: direct; apikey: pending approval
	auth.PUT("/wallets/:id/contracts/:cid", contractH.UpdateContract)    // passkey: direct; apikey: pending approval
	passkeyOnly.DELETE("/wallets/:id/contracts/:cid", contractH.DeleteContract)

	// Address book (dual-auth for read/add/update, Passkey-only for delete).
	abH := handler.NewAddressBookHandler(db, sdkClient, baseURL, approvalExpiry)
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
	reaperCtx, reaperCancel := context.WithCancel(context.Background())
	defer reaperCancel()
	walletH.StartReaper(reaperCtx)
	auth.POST("/wallets", handler.UserRateLimitMiddleware(walletRateLimiter), walletH.CreateWallet)
	auth.GET("/wallets", walletH.ListWallets)
	auth.GET("/wallets/:id", walletH.GetWallet)
	auth.PATCH("/wallets/:id", walletH.RenameWallet)         // rename: API Key or Passkey
	passkeyOnly.DELETE("/wallets/:id", walletH.DeleteWallet) // irreversible: Passkey only

	// General contract call (API Key or Passkey, with security layers).
	contractCallH := handler.NewContractCallHandler(db, sdkClient, baseURL, approvalExpiry)
	contractCallH.SetPriceService(priceService)
	transferRL := handler.UserRateLimitMiddleware(transferRateLimiter)
	auth.POST("/wallets/:id/contract-call", transferRL, contractCallH.ContractCall)
	auth.POST("/wallets/:id/call-read", contractCallH.CallRead) // read-only eth_call; no signing, no whitelist
	auth.POST("/wallets/:id/approve-token", transferRL, contractCallH.ApproveToken)
	auth.POST("/wallets/:id/revoke-approval", transferRL, contractCallH.RevokeApproval)
	auth.POST("/wallets/:id/transfer", transferRL, walletH.Transfer) // backend builds+broadcasts tx
	auth.POST("/wallets/:id/wrap-sol", transferRL, walletH.WrapSOL)
	auth.POST("/wallets/:id/unwrap-sol", transferRL, walletH.UnwrapSOL)
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
	defer approvalH.Stop()
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
		"service_url", os.Getenv("SERVICE_URL"),
		"base_url", baseURL,
		"chains_file", chainsFile,
		"chains_loaded", model.ChainsLen(),
		"approval_expiry_minutes", approvalExpiryMinutes,
		"max_wallets_per_user", maxWalletsPerUser,
		"max_api_keys_per_user", maxAPIKeysPerUser,
		"max_users", maxUsers,
	)

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
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
	// Shutdown HTTP server first — waits for in-flight requests to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}
	// Run SQLite optimize on shutdown to update query planner statistics.
	// Background goroutines (nonce cleanup, DB) are stopped via defers above.
	if _, err := db.DB(); err == nil {
		sqlDB.Exec("PRAGMA optimize")
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

var privateCIDRs []*net.IPNet

func init() {
	for _, cidr := range []string{
		"0.0.0.0/8",
		"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"169.254.0.0/16", "::1/128", "fc00::/7", "fe80::/10",
	} {
		_, network, _ := net.ParseCIDR(cidr)
		privateCIDRs = append(privateCIDRs, network)
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
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		for _, cidr := range privateCIDRs {
			if cidr.Contains(ip) {
				return fmt.Errorf("private/internal IP address is not allowed")
			}
		}
	}
	return nil
}
