# chains.json Schema

## Field Reference

| Field | Required | Type | Valid Values | Description |
|-------|----------|------|--------------|-------------|
| `name` | Yes | string | any unique identifier | API identifier (e.g., `sepolia`, `solana-devnet`) |
| `label` | Yes | string | -- | Human-readable name (e.g., `Sepolia Testnet`) |
| `protocol` | Yes | string | `ecdsa`, `eddsa`, `schnorr`, `schnorr-bip340` | Signature scheme. Use `eddsa` for EdDSA / ed25519 chains (Solana), `schnorr-bip340` for Bitcoin Taproot, `ecdsa` for EVM chains. |
| `curve` | Yes | string | `secp256k1`, `ed25519`, `secp256r1` | Cryptographic curve. Must match protocol: ecdsa→secp256k1/secp256r1, eddsa→ed25519, schnorr-bip340→secp256k1 |
| `currency` | Yes | string | -- | Native currency symbol (e.g., `ETH`, `SOL`, `tBNB`) |
| `family` | Yes | string | `evm`, `solana` | Chain family, determines tx building logic |
| `rpc_url` | Yes | string | -- | JSON-RPC endpoint URL |
| `chain_id` | No | uint64 | -- | EVM chain ID (e.g., 1 for mainnet, 11155111 for Sepolia). Ignored for Solana. |

## Default Chains

The following chains are included in `chains.json` out of the box:

| Chain | Name (API) | Currency | Protocol | Curve | Family |
|-------|------------|----------|----------|-------|--------|
| Ethereum Mainnet | `ethereum` | ETH | ECDSA | secp256k1 | EVM |
| Optimism Mainnet | `optimism` | ETH | ECDSA | secp256k1 | EVM |
| Sepolia Testnet | `sepolia` | ETH | ECDSA | secp256k1 | EVM |
| Holesky Testnet | `holesky` | ETH | ECDSA | secp256k1 | EVM |
| Base Sepolia Testnet | `base-sepolia` | ETH | ECDSA | secp256k1 | EVM |
| BSC Testnet | `bsc-testnet` | tBNB | ECDSA | secp256k1 | EVM |
| Arbitrum One | `arbitrum` | ETH | ECDSA | secp256k1 | EVM |
| Base Mainnet | `base` | ETH | ECDSA | secp256k1 | EVM |
| Polygon PoS | `polygon` | POL | ECDSA | secp256k1 | EVM |
| BNB Smart Chain | `bsc` | BNB | ECDSA | secp256k1 | EVM |
| Avalanche C-Chain | `avalanche` | AVAX | ECDSA | secp256k1 | EVM |
| Solana Mainnet | `solana` | SOL | EdDSA | ed25519 | Solana |
| Solana Devnet | `solana-devnet` | SOL | EdDSA | ed25519 | Solana |

## Common Token Contract Addresses

**Ethereum Mainnet:**

| Token | Contract | Decimals |
|-------|----------|----------|
| USDC | `0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48` | 6 |
| USDT | `0xdac17f958d2ee523a2206206994597c13d831ec7` | 6 |
| WETH | `0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2` | 18 |
| DAI | `0x6b175474e89094c44da98b954eedeac495271d0f` | 18 |

**Sepolia Testnet:**

| Token | Contract | Decimals |
|-------|----------|----------|
| USDC | `0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238` | 6 |
| WETH | `0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14` | 18 |
| LINK | `0x779877A7B0D9E8603169DdbD7836e478b4624789` | 18 |

**Base Sepolia Testnet:**

| Token | Contract | Decimals |
|-------|----------|----------|
| USDC | `0x036CbD53842c5426634e7929541eC2318f3dCF7e` | 6 |
| WETH | `0x4200000000000000000000000000000000000006` | 18 |
