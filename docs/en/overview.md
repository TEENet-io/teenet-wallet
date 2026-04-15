# Overview

TEENet Wallet is a multi-chain cryptocurrency wallet where private keys never exist on any single machine. Every key is split across a cluster of Trusted Execution Environment (TEE) nodes using threshold cryptography. When a transaction needs to be signed, multiple TEE nodes cooperate to produce a valid signature -- the full key is never reconstructed. The wallet exposes a REST API for AI agents and automation, with Passkey-based human approval for high-value operations.

---

## Who Is This For?

- **Contributors** who want to modify the wallet codebase.
- **Developers** integrating their applications via the REST API.
- **Agent platform developers** building autonomous workflows that need secure signing.

---

## How Signing Works

```
 AI Agent / App                        Human (Browser)
     |  API Key (ocw_*)                    |  Passkey (ps_*)
     v                                     v
+--------------------------------------------------------------+
|  TEENet Wallet  (:8080)                                      |
|  REST API · approval policies · contract whitelist            |
+--------------------------------------------------------------+
     |  TEENet SDK
     v
+--------------------------------------------------------------+
|  app-comm-consensus  (:8089)                                 |
|  M-of-N voting coordination                                  |
+--------------------------------------------------------------+
     |  gRPC + mutual TLS
     v
+--------------------------------------------------------------+
|  TEE-DAO Key Management Cluster  (3-5 nodes)                 |
|  Threshold signing · keys never leave TEE hardware            |
+--------------------------------------------------------------+
```

1. Client sends a request (API key or Passkey).
2. Wallet checks whitelist, threshold, and daily limit.
3. If approval is needed, the request enters a pending state until the owner confirms with Passkey.
4. Wallet routes the signing request through the TEE cluster.
5. TEE nodes produce a threshold signature -- the full key is never reconstructed.
6. Wallet broadcasts the signed transaction and returns the hash.

---

## Where to Go Next

| Goal | Page |
|---|---|
| Get hands-on quickly | [Get Started](en/quick-start.md) |
| Understand the architecture | [Concepts](en/architecture-overview.md) |
| Look up API details | [Reference](en/authentication.md) |
| Contribute to the project | [Contributing](en/contributing-process.md) |
