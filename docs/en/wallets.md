# Wallet Management

### Create a Wallet

```bash
curl -s -X POST http://localhost:8080/api/wallets \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"chain": "ethereum", "label": "Main Wallet"}'
```

The `chain` field must match a name from `GET /api/chains` (e.g., `ethereum`, `solana`, `sepolia`, `solana-devnet`, or a custom chain name). Ethereum/EVM wallets use ECDSA on secp256k1 and may take 1-2 minutes for distributed key generation. Solana wallets use Schnorr on ed25519 and are created instantly.

Each user can create up to `MAX_WALLETS_PER_USER` wallets (default: 20).

### List Wallets

```bash
curl -s http://localhost:8080/api/wallets \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

Returns all wallets for the authenticated user, including `id`, `chain`, `address`, `label`, and `status` (`creating`, `ready`, or `error`).

### Get Wallet Details

```bash
curl -s http://localhost:8080/api/wallets/WALLET_ID \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

### Rename a Wallet

```bash
curl -s -X PATCH http://localhost:8080/api/wallets/WALLET_ID \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"label": "Updated Label"}'
```

### Delete a Wallet

Wallet deletion is irreversible and requires a Passkey session:

```bash
curl -s -X DELETE http://localhost:8080/api/wallets/WALLET_ID \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck"
```

### Chain Selection

Query the available chains before creating a wallet:

```bash
curl -s http://localhost:8080/api/chains
```

Response:

```json
{
  "success": true,
  "chains": [
    {"name": "ethereum", "label": "Ethereum Mainnet", "currency": "ETH", "family": "evm", "custom": false},
    {"name": "solana", "label": "Solana Mainnet", "currency": "SOL", "family": "solana", "custom": false},
    {"name": "sepolia", "label": "Sepolia Testnet", "currency": "ETH", "family": "evm", "custom": false}
  ]
}
```

Custom EVM chains can be added at runtime (Passkey required):

```bash
curl -s -X POST http://localhost:8080/api/chains \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "polygon",
    "label": "Polygon Mainnet",
    "currency": "MATIC",
    "rpc_url": "https://polygon-rpc.com",
    "chain_id": 137
  }'
```

Custom chains are persisted in the database and survive restarts. They can be removed with `DELETE /api/chains/:name` (fails if any wallet exists on that chain).

---
[Previous: Authentication](authentication.md) | [Next: Transfers](transfers.md)
