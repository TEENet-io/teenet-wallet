# Audit Log

All wallet operations are recorded in an audit log. Query it with:

```bash
curl -s "${SERVICE_URL}/api/audit/logs?page=1&limit=20" \
  -H "Authorization: Bearer ${API_KEY}"
```

## Query Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `page` | `1` | Page number |
| `limit` | `20` | Results per page (max: 100) |
| `action` | _(all)_ | Filter by action type |
| `wallet_id` | _(all)_ | Filter by wallet |

## Action Types

| Action | Description |
|--------|-------------|
| `login` | Passkey login |
| `wallet_create` | Wallet created |
| `wallet_delete` | Wallet deleted |
| `transfer` | Transfer sent or pending |
| `sign` | Internal signing step during transfer/contract operations |
| `policy_update` | Approval policy set or pending |
| `approval_approve` | Approval request approved |
| `approval_reject` | Approval request rejected |
| `contract_add` | Contract added to whitelist or pending |
| `wrap_sol` | SOL wrapped into wSOL |
| `unwrap_sol` | wSOL unwrapped to SOL |
| `contract_update` | Contract whitelist entry updated |
| `contract_delete` | Contract removed from whitelist |
| `contract_call` | Contract call executed |
| `addressbook_add` | Address book entry added |
| `addressbook_update` | Address book entry updated |
| `addressbook_delete` | Address book entry deleted |
| `apikey_generate` | API key generated |
| `apikey_revoke` | API key revoked |
| `apikey_rename` | API key renamed |
