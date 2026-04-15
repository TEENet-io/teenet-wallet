# Transfers

### Native Transfers (ETH / SOL)

Send native currency by calling the `/transfer` endpoint without a `token` field:

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets/WALLET_ID/transfer \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "0.1",
    "memo": "payment for services"
  }'
```

For Solana wallets, use a base58 recipient address:

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets/WALLET_ID/transfer \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "RecipientBase58Address...",
    "amount": "1.5"
  }'
```

The `to` field accepts either a raw on-chain address or an address book nickname (see [Address Book](addressbook.md)). When a nickname is provided, the wallet resolves it to the stored address for the wallet's chain.

The backend builds the transaction (EIP-1559 for EVM, native transfer instruction for Solana), signs it via the TEE cluster, and broadcasts it. The response includes the `tx_hash` on success.

### ERC-20 Token Transfers

Include the `token` field to send ERC-20 tokens. The token contract must be whitelisted first (see [Contract Whitelist](smart-contracts.md#contract-whitelist)).

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets/WALLET_ID/transfer \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "100",
    "token": {
      "contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
      "symbol": "USDC",
      "decimals": 6
    }
  }'
```

The `amount` is in human-readable token units (e.g., `100` for 100 USDC). The backend converts to raw units using the `decimals` value.

**Important:** Omitting the `token` field sends native ETH instead. Always double-check that your request payload includes the `token` object when sending tokens.

### SPL Token Transfers (Solana)

SPL token transfers use the same `/transfer` endpoint with the `token` field. The token mint must be whitelisted.

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets/WALLET_ID/transfer \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "RecipientBase58Address...",
    "amount": "50",
    "token": {
      "contract": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
      "symbol": "USDC",
      "decimals": 6
    }
  }'
```

If the recipient does not have an Associated Token Account (ATA) for the token mint, the backend creates it automatically in the same transaction.

### Wrap and Unwrap SOL

Convert native SOL to wSOL (Wrapped SOL SPL token):

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets/WALLET_ID/wrap-sol \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"amount": "0.5"}'
```

Unwrap all wSOL back to native SOL (closes the wSOL ATA):

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets/WALLET_ID/unwrap-sol \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{}'
```

### Idempotency

To prevent duplicate transactions when retrying after network errors, include the `Idempotency-Key` header:

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets/WALLET_ID/transfer \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Idempotency-Key: unique-request-id-12345" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "0.5"
  }'
```

If a request with the same idempotency key has already been processed, the wallet returns the original cached response without executing the transfer again.

- **Scope:** Per-user -- two different API keys for the same user share the same idempotency namespace.
- **TTL:** 24 hours -- after that, the same key can be reused.
- **Applies to:** `/transfer`, `/contract-call`, `/wrap-sol`, `/unwrap-sol`.
