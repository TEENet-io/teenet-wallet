# Talking to OpenClaw

This is the heart of the experience. You interact with your wallet by chatting with OpenClaw. You can use natural language or quick commands -- both work the same way.

---

## Feature Overview

| Feature | What you can do | Needs approval? |
|---------|----------------|----------------|
| **Create wallet** | Create wallets on Ethereum, Solana, and other supported chains | No |
| **Check balance** | View native and token balances across all wallets | No |
| **Send crypto** | Transfer ETH, SOL, ERC-20, or SPL tokens to any address | Only if above your USD threshold |
| **Swap / DeFi** | Interact with Uniswap, Aave, or any whitelisted contract | Yes (all contract calls) |
| **Token approval** | Grant or revoke token spending permission | Yes (always) |
| **Wrap / Unwrap SOL** | Convert between native SOL and wSOL | Only if above threshold |
| **Read contract data** | Query balances, allowances, prices on-chain | No |
| **Manage whitelist** | Add or remove contracts from the whitelist | Yes (add); Passkey only (remove) |
| **Set spending policy** | Configure USD threshold and daily limit | Yes |
| **View approvals** | List and check pending approval requests | No |
| **View history** | See audit log of all past operations | No |
| **Check prices** | See current ETH, SOL, and stablecoin prices | No |
| **Manage API keys** | Generate or revoke API keys | Passkey only |

---

## Natural Language Examples

### Wallet Management

- **"Create an Ethereum wallet called Trading"** -- creates a new wallet on Ethereum with the label "Trading."
- **"Create a Solana wallet"** -- creates a new Solana wallet.
- **"Show my wallets"** -- lists all your wallets with their addresses, chains, and labels.
- **"What's my balance?"** -- shows the balance of your current wallet (or asks you to pick one if you have several).
- **"Show all my balances"** -- shows balances across all your wallets.

### Sending Crypto

- **"Send 0.1 ETH to 0xABC...123"** -- sends 0.1 ETH to the specified address. If it is below your threshold, it goes through instantly.
- **"Send 50 USDC to 0xDEF...456"** -- sends 50 USDC (an ERC-20 token transfer). OpenClaw will make sure the USDC contract is whitelisted first.
- **"Transfer 1 SOL to ABC...XYZ"** -- sends 1 SOL on Solana.

### DeFi and Contract Interaction

- **"Swap 0.5 ETH for USDC on Uniswap"** -- OpenClaw builds the swap transaction and sends you an approval link. All contract interactions require your Passkey approval.
- **"Approve USDC spending for Uniswap router"** -- grants a token allowance (requires your approval).
- **"Call balanceOf on USDC contract for my address"** -- reads data from a smart contract without sending a transaction. Read-only queries don't need approval.

### Whitelist Management

- **"Add USDC contract to my whitelist"** -- proposes adding the USDC contract. You will need to approve this in the Web UI.
- **"Show my whitelisted contracts"** -- lists all contracts currently on your whitelist.

Note: Adding a contract to your whitelist always requires Passkey approval. This is a safety measure to prevent unauthorized contracts from being added.

### Policy and Security

- **"Set approval threshold to $200"** -- proposes changing your spending threshold (requires Passkey approval).
- **"Show my approval policy"** -- displays your current threshold and daily limit.
- **"Show pending approvals"** -- lists any transactions waiting for your approval.
- **"How much have I spent today?"** -- shows today's USD spend and remaining daily budget.

---

## Quick Commands

For experienced users, these shortcuts provide fast access to common operations:

| Command | What it does |
|---------|-------------|
| `/balance` | Show all wallet balances |
| `/balance eth` | Show balance for a specific chain |
| `/wallets` | List all your wallets |
| `/transfer 0.1 ETH to 0xabc...` | Send crypto |
| `/approve` | List pending approvals |
| `/approve 123456` | View a specific approval |
| `/whitelist` | List whitelisted contracts |
| `/whitelist 0xabc...` | Add a contract to whitelist |
| `/policy` | Show current approval policy |
| `/policy 100` | Set threshold to $100 |
| `/spent` | Today's USD spend and remaining budget |
| `/prices` | Current crypto prices |
| `/chains` | Available blockchains |
| `/call 0xabc... method(args)` | Call a smart contract |

Commands are case-insensitive. You can also just say the same thing in plain language -- OpenClaw understands both.

---
[Previous: Getting Started](/en/user-getting-started.md) | [Next: Approvals & Web UI](/en/user-approvals.md)
