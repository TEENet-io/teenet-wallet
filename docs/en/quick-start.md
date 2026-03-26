# Quick Start

### Prerequisites

- **Go 1.24+** (for building from source)
- **SQLite3 development headers** (`apt-get install libsqlite3-dev` on Debian/Ubuntu)
- A running **TEENet mesh node** with `app-comm-consensus` on port 8089

### Installation

Build from source:

```bash
git clone https://github.com/TEENet-io/teenet-wallet.git
cd teenet-wallet
make build
```

Or use Docker:

```bash
make docker
docker run -p 8080:8080 \
  -e CONSENSUS_URL=http://host.docker.internal:8089 \
  -v wallet-data:/data \
  teenet-wallet:latest
```

Start the server:

```bash
./teenet-wallet
```

The server listens on `http://0.0.0.0:8080` by default.

All examples in this documentation use shell variables. Set them before running the commands:

```bash
export TEE_WALLET_URL="http://localhost:8080"   # your wallet server URL
export API_KEY="ocw_..."                         # your API key (generated in Step 2)
```

### Create Your First Wallet

**Step 1: Register with a Passkey.**

Open the web UI (e.g., `http://localhost:8080`) in a browser that supports WebAuthn (Chrome, Safari, Firefox). Complete the Passkey registration flow. This creates your user account and a Passkey session.

**Step 2: Generate an API key.**

From the web UI, go to Settings and generate an API key. The key starts with `ocw_`. Save it securely -- it is shown only once.

Alternatively, if you already have a Passkey session token (`ps_`), you can use the API:

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/auth/apikey/generate \
  -H "Authorization: Bearer ps_${SESSION_TOKEN}" \
  -H "X-CSRF-Token: nocheck" \
  -H "Content-Type: application/json" \
  -d '{"label": "my-agent-key"}'
```

**Step 3: Create a wallet.**

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"chain": "sepolia", "label": "Test Wallet"}'
```

Response:

```json
{
  "success": true,
  "wallet": {
    "id": "8a2fbc16-faf4-451a-be34-9fc5c49cde00",
    "chain": "sepolia",
    "address": "0x1234abcd5678ef90...",
    "label": "Test Wallet",
    "status": "ready"
  }
}
```

Note: Ethereum (ECDSA) wallets may take 1-2 minutes to create due to distributed key generation. Solana wallets are created instantly.

### Send Your First Transaction

Fund the wallet address with testnet ETH from a Sepolia faucet, then send a transfer:

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets/8a2fbc16-faf4-451a-be34-9fc5c49cde00/transfer \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "0xRecipientAddress...",
    "amount": "0.01",
    "memo": "first test transfer"
  }'
```

Response (direct completion):

```json
{
  "success": true,
  "status": "completed",
  "tx_hash": "0xabc123...",
  "chain": "sepolia",
  "explorer_url": "https://sepolia.etherscan.io/tx/0xabc123..."
}
```

### Set an Approval Policy

Protect the wallet by requiring Passkey approval for transfers above $50 USD, with a $500 daily limit:

```bash
curl -s -X PUT ${TEE_WALLET_URL}/api/wallets/8a2fbc16-faf4-451a-be34-9fc5c49cde00/policy \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "threshold_usd": "50",
    "daily_limit_usd": "500",
    "enabled": true
  }'
```

When called with an API key, policy changes require Passkey approval. The response includes an `approval_id`:

```json
{
  "success": true,
  "pending": true,
  "approval_id": 1,
  "message": "Policy change submitted for approval"
}
```

Open the approval link in the web UI and confirm with your Passkey to activate the policy.

---
[Previous: Introduction](introduction.md) | [Next: Configuration](configuration.md)
