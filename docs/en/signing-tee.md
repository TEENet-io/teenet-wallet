# Signing & TEE Trust Model

This page explains what happens **inside the TEENet service** when the wallet sends a signing request. The wallet itself never touches private keys -- it delegates all cryptographic operations to the TEENet service through the SDK.

---

## Threshold Cryptography

TEENet uses threshold cryptography to eliminate single points of compromise:

- When a key is generated, it is **sharded** across 3--5 TEE nodes. Each node holds one share of the private key.
- Signing requires **M-of-N** cooperation (e.g. 2 of 3 nodes, or 3 of 5). A single compromised node cannot produce a valid signature.
- The full private key is **never reconstructed** anywhere -- not on any node, not in memory, not during signing. Each node computes a partial signature from its share, and the coordinator assembles the final signature from the partial results.

This means an attacker would need to compromise multiple TEE enclaves simultaneously to extract enough shares to reconstruct a key.

---

## The Signing Flow

When the wallet calls `sdk.Sign(ctx, msgBytes, keyName)`, the following happens inside the TEENet service:

```
Wallet                    TEENet Service
  │                           │
  │  Sign(msg, keyName)       │
  │ ─────────────────────────>│
  │                           │
  │              ┌──────┐ ┌──────┐ ┌──────┐
  │              │ TEE  │ │ TEE  │ │ TEE  │
  │              │ Node │ │ Node │ │ Node │
  │              │  1   │ │  2   │ │  3   │
  │              └──┬───┘ └──┬───┘ └──┬───┘
  │                 │        │        │
  │                 │ partial signatures
  │                 │        │        │
  │                 └───┬────┘────┘
  │                     ▼
  │              Initiating node
  │              assembles final
  │              signature
  │                     │
  │  Signature          │
  │ <───────────────────┘
  │
```

1. The wallet sends the message bytes and key name to the TEENet service via the SDK.
2. One of the TEE nodes initiates the signing process and distributes the request to other nodes holding shares for the requested key.
3. Each participating TEE node computes a **partial signature** using its key share inside the enclave.
4. The initiating node collects the required number of partial signatures (M out of N) and assembles the final threshold signature.
5. The complete signature is returned to the wallet, which broadcasts the signed transaction to the blockchain.

---

## What is `app_instance_id`?

Each application registered with the TEENet service receives an **application instance ID**. This ID serves two purposes:

- **Identity**: it tells the TEENet service which application is making the request, allowing key isolation between applications.
- **Signing mode**: the ID determines how signing requests are processed.

The wallet reads its `APP_INSTANCE_ID` from the environment at startup and passes it to the SDK via `SetDefaultAppInstanceIDFromEnv()`. All subsequent key generation and signing calls use this ID.

### Signing modes

| Mode | Behavior |
|------|----------|
| **Direct** | The coordinator signs immediately and returns the signature. Used for standard wallet operations. |
| **Multi-instance voting** | Multiple instances run concurrently. A signing request enters a pending state until enough instances independently submit the same request to reach the threshold. Used for multi-party workflows where no single instance should be able to sign alone. |

In typical wallet deployments, the application uses **direct** mode. The wallet's own approval system (threshold policies, daily limits) handles human authorization at the application layer, before the signing request ever reaches the TEENet service.

---

## Next Steps

- [SDK Usage](sdk-usage.md) -- SDK interface reference
- [Architecture Overview](architecture-overview.md) -- system mental model and core workflows
