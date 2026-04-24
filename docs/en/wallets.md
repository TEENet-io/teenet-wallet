# Wallet Management

### Create a Wallet

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"chain": "sepolia", "label": "Main Wallet"}'
```

The `chain` field must match a name returned by `GET /api/chains`. During the public alpha (`ALPHA_MODE=true`) this is the 8 testnets: `sepolia`, `optimism-sepolia`, `arbitrum-sepolia`, `base-sepolia`, `polygon-amoy`, `bsc-testnet`, `avalanche-fuji`, `solana-devnet`. Mainnet chains (`ethereum`, `solana`, `optimism`, `arbitrum`, `base`, `polygon`, `bsc`, `avalanche`) are present in `chains.json` but the alpha gate hides them — see [chains.json Schema](chains-schema.md). EVM wallets use ECDSA on secp256k1 and may take 1-2 minutes for distributed key generation. Solana wallets use EdDSA on ed25519 and are created instantly.

Each user can create up to `MAX_WALLETS_PER_USER` wallets (default: 10).

### List Wallets

```bash
curl -s ${TEE_WALLET_URL}/api/wallets \
  -H "Authorization: Bearer ${API_KEY}"
```

Returns all wallets for the authenticated user, including `id`, `chain`, `address`, `label`, and `status` (`creating`, `ready`, or `error`).

### Get Wallet Details

```bash
curl -s ${TEE_WALLET_URL}/api/wallets/WALLET_ID \
  -H "Authorization: Bearer ${API_KEY}"
```

### Rename a Wallet

```bash
curl -s -X PATCH ${TEE_WALLET_URL}/api/wallets/WALLET_ID \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"label": "Updated Label"}'
```

### Delete a Wallet

Wallet deletion is irreversible and requires a Passkey session:

```bash
curl -s -X DELETE ${TEE_WALLET_URL}/api/wallets/WALLET_ID \
  -H "Authorization: Bearer ps_${SESSION_TOKEN}" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}"
```

### Chain Selection

Query the available chains before creating a wallet:

```bash
curl -s ${TEE_WALLET_URL}/api/chains
```

Response:

```json
{
  "success": true,
  "chains": [
    {"name": "sepolia", "label": "Sepolia Testnet", "protocol": "ecdsa", "curve": "secp256k1", "currency": "ETH", "family": "evm", "rpc_url": "", "chain_id": 0, "testnet": true},
    {"name": "optimism-sepolia", "label": "Optimism Sepolia Testnet", "protocol": "ecdsa", "curve": "secp256k1", "currency": "ETH", "family": "evm", "rpc_url": "", "chain_id": 0, "testnet": true},
    {"name": "arbitrum-sepolia", "label": "Arbitrum Sepolia Testnet", "protocol": "ecdsa", "curve": "secp256k1", "currency": "ETH", "family": "evm", "rpc_url": "", "chain_id": 0, "testnet": true},
    {"name": "base-sepolia", "label": "Base Sepolia Testnet", "protocol": "ecdsa", "curve": "secp256k1", "currency": "ETH", "family": "evm", "rpc_url": "", "chain_id": 0, "testnet": true},
    {"name": "polygon-amoy", "label": "Polygon Amoy Testnet", "protocol": "ecdsa", "curve": "secp256k1", "currency": "POL", "family": "evm", "rpc_url": "", "chain_id": 0, "testnet": true},
    {"name": "bsc-testnet", "label": "BSC Testnet", "protocol": "ecdsa", "curve": "secp256k1", "currency": "tBNB", "family": "evm", "rpc_url": "", "chain_id": 0, "testnet": true},
    {"name": "avalanche-fuji", "label": "Avalanche Fuji Testnet", "protocol": "ecdsa", "curve": "secp256k1", "currency": "AVAX", "family": "evm", "rpc_url": "", "chain_id": 0, "testnet": true},
    {"name": "solana-devnet", "label": "Solana Devnet", "protocol": "eddsa", "curve": "ed25519", "currency": "SOL", "family": "solana", "rpc_url": "", "chain_id": 0, "testnet": true}
  ]
}
```

This is the public alpha chain set (`ALPHA_MODE=true`). Deployments that leave `ALPHA_MODE` unset also see the mainnet chains present in `chains.json` — the `testnet` field is emitted only for testnet entries.

`rpc_url` is intentionally blanked in the response — all RPC calls happen server-side and the browser never needs the URL.

Chain definitions are loaded from `chains.json` at startup. To add or remove chains, edit that file and restart the service. See [chains.json Schema](chains-schema.md) and [How to Add a Chain](howto-add-chain.md) for details.
