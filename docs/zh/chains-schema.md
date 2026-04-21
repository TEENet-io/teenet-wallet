# chains.json Schema

## 字段参考

| 字段 | 必填 | 类型 | 有效值 | 描述 |
|------|------|------|--------|------|
| `name` | 是 | string | 任意唯一标识符 | API 标识符（例如 `sepolia`、`solana-devnet`） |
| `label` | 是 | string | -- | 人类可读名称（例如 `Sepolia Testnet`） |
| `protocol` | 是 | string | `ecdsa`, `eddsa`, `schnorr-bip340` | 签名方案。Solana 这类 EdDSA / ed25519 链使用 `eddsa`，Bitcoin Taproot 使用 `schnorr-bip340`，EVM 链使用 `ecdsa`。 |
| `curve` | 是 | string | `secp256k1`, `ed25519`, `secp256r1` | 密码学曲线。必须与协议匹配：ecdsa→secp256k1/secp256r1，eddsa→ed25519，schnorr-bip340→secp256k1 |
| `currency` | 是 | string | -- | 原生代币符号（例如 `ETH`、`SOL`、`tBNB`） |
| `family` | 是 | string | `evm`, `solana` | 链系列，决定交易构建逻辑 |
| `rpc_url` | 是 | string | -- | JSON-RPC 端点 URL |
| `chain_id` | 否 | uint64 | -- | EVM 链 ID（例如主网为 1，Sepolia 为 11155111）。Solana 忽略此字段。 |
| `quicknode_network` | 否 | string | QuickNode network slug（例如 `ethereum-sepolia`），或 `-` 表示无子域（Ethereum Mainnet） | 配置后，且 `QUICKNODE_ENDPOINT` + token 源（`QUICKNODE_TOKEN` 或 `QUICKNODE_TOKEN_KEY`）齐全时，启动时 `rpc_url` 会被覆写为 `https://{endpoint}.{quicknode_network}.quiknode.pro/{token}/`。`-` 哨兵值表示不带 network 子域——Ethereum Mainnet 的 URL 形如 `https://{endpoint}.quiknode.pro/{token}/`。留空则保持写死的 `rpc_url`。详见 [configuration.md](./configuration.md#quicknode-rpc-覆写)。 |
| `quicknode_path` | 否 | string | 路径后缀（例如 `/ext/bc/C/rpc`） | 拼接在 QuickNode URL 中的 `{token}` 之后。仅当 `quicknode_network` 设置时生效。非标准路径的链需要——默认配置里只有 Avalanche C-Chain 用到。 |

## 默认链

以下链已包含在开箱即用的 `chains.json` 中：

| 链 | Name (API) | Currency | Protocol | Curve | Family |
|-----|------------|----------|----------|-------|--------|
| Ethereum Mainnet | `ethereum` | ETH | ECDSA | secp256k1 | EVM |
| Optimism Mainnet | `optimism` | ETH | ECDSA | secp256k1 | EVM |
| Sepolia Testnet | `sepolia` | ETH | ECDSA | secp256k1 | EVM |
| Base Sepolia Testnet | `base-sepolia` | ETH | ECDSA | secp256k1 | EVM |
| BSC Testnet | `bsc-testnet` | tBNB | ECDSA | secp256k1 | EVM |
| Arbitrum One | `arbitrum` | ETH | ECDSA | secp256k1 | EVM |
| Base Mainnet | `base` | ETH | ECDSA | secp256k1 | EVM |
| Polygon PoS | `polygon` | POL | ECDSA | secp256k1 | EVM |
| BNB Smart Chain | `bsc` | BNB | ECDSA | secp256k1 | EVM |
| Avalanche C-Chain | `avalanche` | AVAX | ECDSA | secp256k1 | EVM |
| Solana Mainnet | `solana` | SOL | EdDSA | ed25519 | Solana |
| Solana Devnet | `solana-devnet` | SOL | EdDSA | ed25519 | Solana |

## 常用代币合约地址

**Ethereum Mainnet：**

| Token | Contract | Decimals |
|-------|----------|----------|
| USDC | `0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48` | 6 |
| USDT | `0xdac17f958d2ee523a2206206994597c13d831ec7` | 6 |
| WETH | `0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2` | 18 |
| DAI | `0x6b175474e89094c44da98b954eedeac495271d0f` | 18 |

**Sepolia Testnet：**

| Token | Contract | Decimals |
|-------|----------|----------|
| USDC | `0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238` | 6 |
| WETH | `0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14` | 18 |
| LINK | `0x779877A7B0D9E8603169DdbD7836e478b4624789` | 18 |

**Base Sepolia Testnet：**

| Token | Contract | Decimals |
|-------|----------|----------|
| USDC | `0x036CbD53842c5426634e7929541eC2318f3dCF7e` | 6 |
| WETH | `0x4200000000000000000000000000000000000006` | 18 |
