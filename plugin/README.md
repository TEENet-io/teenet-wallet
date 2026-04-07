# TEENet Wallet — OpenClaw Plugin

Manage TEE hardware wallets through Telegram by chatting with an AI agent. Transfer crypto, interact with smart contracts, and approve sensitive operations with Passkey hardware security — all from a chat interface.

## Features

**Wallet Management**
- Create wallets on EVM chains (Ethereum, Sepolia, Base, Optimism) and Solana
- Check balances (native + whitelisted tokens)
- Rename wallets, view details

**Transfers**
- Send native coins and tokens (ERC-20, SPL)
- Transfers exceeding the USD threshold automatically trigger Passkey approval
- Address book support — send to nicknames instead of raw addresses
- Wrap/Unwrap SOL for Solana DeFi

**Smart Contract Interaction**
- Whitelist management for token contracts
- Read-only and state-changing contract calls
- ERC-20 approve/revoke (supports Uniswap V3 and other DeFi protocols)
- Tuple ABI encoding for complex function signatures

**Security**
- Passkey hardware approval for sensitive operations (transfers above threshold, policy changes, contract whitelist)
- USD-based approval thresholds with daily spending limits
- Real-time notifications — agent is notified within seconds after approval and continues the operation
- All operations audited with structured logs

## How It Works

```
User (Telegram)  ←→  OpenClaw Agent  ←→  Plugin Tools  ──REST──→  Wallet Backend
                          ↑                                            |
                     subagent.run()                              Passkey Approval
                     (deliver=true)                                    ↓
                          ↑                                    SSE Event Stream
                          └──────── ApprovalWatcher  ←─────────────────┘
```

1. User sends a message like "Send 0.1 ETH to alice"
2. Agent selects the right tool (`teenet_wallet_transfer`) and calls the wallet backend
3. If the amount exceeds the approval threshold, the backend returns `pending_approval`
4. Agent sends the user an approval link
5. User clicks the link and verifies with Passkey hardware (fingerprint / security key)
6. Backend approves, signs the transaction via TEE, and broadcasts to the blockchain
7. SSE event notifies the plugin → subagent runs in the original chat session
8. User receives: "Transfer complete! TX: 0x... [View on Explorer]"

For multi-step operations (e.g. token swaps), the agent maintains context through conversation history and continues to the next step automatically after each approval.

## Requirements

- [OpenClaw](https://openclaw.ai) >= 2026.3.24
- Node.js >= 18
- TEENet wallet backend instance
- API Key (`ocw_` prefix)

## Installation

```bash
# Install the plugin
openclaw plugins install "/path/to/teenet-wallet/plugin"

# Configure API endpoint and key
openclaw config set plugins.entries.teenet-wallet.config.apiUrl "https://your-wallet-instance/"
openclaw config set plugins.entries.teenet-wallet.config.apiKey "ocw_your_api_key"
openclaw config set plugins.entries.teenet-wallet.enabled true

# Restart the gateway
openclaw gateway restart

# Verify
openclaw plugins inspect teenet-wallet
# Expected: Status: loaded
```

## Configuration

| Parameter | Required | Description |
|-----------|----------|-------------|
| `apiUrl` | Yes | Wallet backend URL (e.g. `https://test.teenet.io/instance/xxx/`) |
| `apiKey` | Yes | API Key with `ocw_` prefix |

### tools.profile

OpenClaw's `tools.profile` controls which tools the agent can use. The plugin requires `full` profile (the default). If you've set a different profile, plugin tools will be **silently blocked** — no errors, no warnings, agent just won't use any wallet tools.

| Profile | Plugin Tools |
|---------|-------------|
| `full` (default) | Available |
| `coding` | Blocked |
| `messaging` | Blocked |
| `minimal` | Blocked |

To check and fix:

```bash
# Check current profile
openclaw config get tools.profile

# Clear if not full
openclaw config unset tools.profile
openclaw gateway restart
```

## Usage

After installation, chat with the agent on Telegram. The agent automatically selects the right tools based on your request.

### Basic Operations

| What you say | What happens |
|-------------|--------------|
| "Create a Sepolia wallet" | Creates an EVM wallet on Sepolia testnet |
| "Check my balances" | Lists all wallets with native + token balances |
| "Send 0.01 ETH to 0x..." | Transfers ETH, triggers approval if above threshold |
| "Set approval threshold to $100" | Configures USD threshold (requires Passkey approval) |
| "Whitelist USDC on my Sepolia wallet" | Adds token contract to whitelist |
| "Swap 100 USDC for ETH" | Multi-step: whitelist → approve → swap (agent handles the flow) |

### Approval Flow

When an operation requires approval:
1. Agent sends you an approval link
2. Open the link in your browser
3. Verify with Passkey (fingerprint / security key)
4. Agent automatically receives the result and continues

You don't need to come back and tell the agent you approved — it knows.

### Guided Test

Say "run the test flow" and the agent will walk you through:
1. Check balance → Get test tokens from faucet
2. Transfer to self (verifies the full transaction flow works)
3. Set approval policy → Approve via Passkey
4. Transfer below threshold (no approval needed)
5. Transfer above threshold (approval needed)
6. Whitelist a token contract

## Available Tools

| Category | Tools | Description |
|----------|-------|-------------|
| Wallet | `create`, `list`, `get`, `rename`, `balance` | Wallet CRUD and balance queries |
| Transfer | `transfer`, `wrap_sol`, `unwrap_sol` | Send native coins and tokens |
| Contracts | `list_contracts`, `add_contract`, `update_contract`, `contract_call`, `approve_token`, `revoke_approval` | Whitelist and interact with smart contracts |
| Policy | `get_policy`, `set_policy`, `daily_spent` | Approval thresholds and spending limits |
| Address Book | `list_contacts`, `add_contact`, `update_contact` | Save and use address nicknames |
| Approvals | `pending_approvals`, `check_approval` | Monitor approval status |
| Utility | `list_chains`, `health`, `faucet`, `audit_logs`, `prices`, `get_pubkey` | Chain info, testing, and diagnostics |

All tool names are prefixed with `teenet_wallet_` (e.g. `teenet_wallet_transfer`).

## Security Notes

- **API Key is stored in plugin config**, never exposed to the LLM or conversation
- **Passkey hardware verification** required for approvals — session tokens alone are not enough
- **All write operations** (transfer, policy change, contract whitelist) check approval policies
- **SSE events are user-scoped** — each user only receives their own approval events
- **SSRF protection** on custom chain RPC URLs — private IPs and cloud metadata addresses are blocked

## Project Structure

```
plugin/
├── index.ts                  # Plugin entry — registers tools and SSE service
├── openclaw.plugin.json      # Plugin manifest
├── package.json
├── src/
│   ├── api-client.ts         # Wallet backend HTTP client
│   ├── approval-watcher.ts   # SSE subscription + approval notifications
│   ├── tools/
│   │   ├── wallet.ts         # Wallet CRUD tools
│   │   ├── balance.ts        # Balance query tool
│   │   ├── transfer.ts       # Transfer tool
│   │   ├── contract.ts       # Contract interaction tools
│   │   ├── policy.ts         # Approval policy tools
│   │   ├── address-book.ts   # Address book tools
│   │   ├── misc.ts           # Utility tools (chains, health, faucet, etc.)
│   │   └── tool-result.ts    # Shared approval result handling
│   └── __tests__/            # Unit and E2E tests
├── skill/
│   └── tee-wallet/
│       └── SKILL.md          # Agent instructions (when and how to use each tool)
├── SOLUTION.md               # Technical design document
└── DEVLOG.md                 # Development log and lessons learned
```

## Supported Chains

Chains are configured by the backend administrator. Default built-in chains:

| Chain | Type | Currency | Protocol |
|-------|------|----------|----------|
| Ethereum | Mainnet | ETH | ECDSA / secp256k1 |
| Sepolia | Testnet | ETH | ECDSA / secp256k1 |
| Base Sepolia | Testnet | ETH | ECDSA / secp256k1 |
| Optimism | Mainnet | ETH | ECDSA / secp256k1 |
| Holesky | Testnet | ETH | ECDSA / secp256k1 |
| BSC Testnet | Testnet | tBNB | ECDSA / secp256k1 |
| Solana | Mainnet | SOL | Schnorr / ed25519 |
| Solana Devnet | Testnet | SOL | Schnorr / ed25519 |

Administrators can add custom EVM chains via `POST /api/chains` (Passkey required).

## License

MIT
