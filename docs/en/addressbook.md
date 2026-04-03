# Address Book

The address book lets you save frequently used recipient addresses with human-readable nicknames. Once saved, you can use the nickname instead of the raw address when sending transfers.

### List Entries

```bash
curl -s "${TEE_WALLET_URL}/api/addressbook" \
  -H "Authorization: Bearer ${API_KEY}"
```

Filter by nickname or chain:

```bash
curl -s "${TEE_WALLET_URL}/api/addressbook?nickname=alice&chain=ethereum" \
  -H "Authorization: Bearer ${API_KEY}"
```

### Add an Entry

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/addressbook" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "nickname": "alice",
    "chain": "ethereum",
    "address": "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
    "memo": "Alice main wallet"
  }'
```

**Fields:**
- `nickname` (required): Lowercase alphanumeric with hyphens/underscores, max 100 chars. Must match `^[a-z0-9][a-z0-9_-]*$`.
- `chain` (required): Chain name from `GET /api/chains`.
- `address` (required): Valid on-chain address (EVM `0x...` or Solana base58).
- `memo` (optional): Free-text note, max 256 chars.

Nicknames are unique per user per chain -- you can have `alice` on both `ethereum` and `solana`, but not two `alice` entries on the same chain.

**Dual-auth behavior:**
- **API key:** Creates a pending approval request. The Passkey owner must approve before the entry is saved.
- **Passkey session:** Requires a fresh hardware assertion, then applies immediately.

### Update an Entry

```bash
curl -s -X PUT "${TEE_WALLET_URL}/api/addressbook/ENTRY_ID" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "address": "0xNewAddress...",
    "memo": "Updated memo"
  }'
```

You can update `nickname`, `address`, and/or `memo`. Only provided fields are changed. The same dual-auth behavior applies (API key creates pending approval, Passkey applies directly).

### Delete an Entry

Deletion requires a Passkey session:

```bash
curl -s -X DELETE "${TEE_WALLET_URL}/api/addressbook/ENTRY_ID" \
  -H "Authorization: Bearer ps_${SESSION_TOKEN}" \
  -H "X-CSRF-Token: nocheck"
```

### Using Nicknames in Transfers

Once an address book entry exists, you can use the nickname in the `to` field of a transfer request instead of a raw address:

```bash
curl -s -X POST "${TEE_WALLET_URL}/api/wallets/WALLET_ID/transfer" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "alice",
    "amount": "0.1"
  }'
```

The wallet automatically detects that `alice` is a nickname (not a raw address) and resolves it to the stored address for the wallet's chain. If no matching entry is found, the request fails with an error.

---
[Previous: Transfers](/en/transfers.md) | [Next: Smart Contracts](/en/smart-contracts.md)
