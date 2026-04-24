# Overview

> **Alpha release.** The public TEENet Wallet is currently in alpha. The deployment runs with `ALPHA_MODE=true`, which exposes **testnet chains only** (Sepolia, Optimism Sepolia, Arbitrum Sepolia, Base Sepolia, Polygon Amoy, BSC Testnet, Avalanche Fuji, Solana Devnet); registration is capped at the **first 500 users** (first-come, first-served). Mainnet chains (Ethereum, Optimism, Arbitrum, Base, Polygon, BNB Chain, Avalanche, Solana) are implemented and shipped in `chains.json`; they'll be re-enabled by clearing `ALPHA_MODE` once the alpha cohort validates the system.

TEENet Wallet is a multi-chain crypto wallet that lets your AI agent handle routine transactions — balance checks, transfers, contract calls — while you keep approval for what matters. Set transfer limits, restrict which contracts your agent can interact with, and confirm high-value actions with a single Passkey tap. Your rules are enforced inside hardware-protected enclaves, not just in application code.

Private keys never exist on any single machine. They are generated inside TEE nodes, sharded across multiple independent nodes using threshold cryptography, and never exported or reconstructed. Signing requires cooperation from multiple nodes — no operator, cloud provider, or compromised server can unilaterally access your keys.
