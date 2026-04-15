# What You Can Do

You interact with your wallet by chatting with an AI agent. Just tell it what you want in plain language.

---

## Wallets & Balances

- **"Create an Ethereum wallet called Trading"**
- **"Show my wallets"** -- lists all wallets with addresses, chains, and labels
- **"What's my balance?"** -- shows balances across your wallets

## Transfers & DeFi

- **"Send 0.1 ETH to 0xABC...123"** -- goes through instantly if below your threshold
- **"Send 0.1 ETH to alice"** -- sends to the address saved as "alice" in your address book
- **"Send 50 USDC to 0xDEF...456"** -- ERC-20 token transfer; the agent ensures the contract is whitelisted
- **"Swap 0.5 ETH for USDC on Uniswap"** -- builds the swap and sends you an approval link (all contract interactions require Passkey)

<div align="center"><img src="picture/transfer-flow.gif" alt="Transfer conversation flow demo" width="480" /></div>

## Settings & Security

- **"Set approval threshold to $200"** -- changes your spending threshold (requires Passkey)
- **"Add USDC contract to my whitelist"** -- proposes adding a contract (requires Passkey)
- **"Show pending approvals"** -- lists transactions waiting for your approval
- **"How much have I spent today?"** -- shows today's USD spend and remaining daily budget
- **"Save alice as 0xABC...123 on Ethereum"** -- saves a contact to your address book

---

## What Needs Approval?

Most read operations (balances, history, prices) never need approval. Transactions below your USD threshold go through automatically. Everything else requires your Passkey:

- Transactions above your threshold or daily limit
- All smart contract interactions (swaps, token approvals, DeFi)
- Whitelist and policy changes
- API key management

---

## Quick Commands

Some agent platforms support shortcut commands. For example, on [OpenClaw](https://openclaw.ai):

| Command | What it does |
|---------|-------------|
| `/balance` | Show all wallet balances |
| `/transfer 0.1 ETH to 0xabc...` | Send crypto |
| `/approve` | List pending approvals |
| `/whitelist` | List whitelisted contracts |
| `/policy` | Show current approval policy |
| `/contacts` | Show address book |
| `/spent` | Today's USD spend |
| `/chains` | Available blockchains |

You can always say the same thing in plain language instead.
