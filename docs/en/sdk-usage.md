# TEENet SDK Usage

This page documents how the wallet uses the TEENet SDK (`github.com/TEENet-io/teenet-sdk/go`) in practice. It covers initialization, key generation, signing, Passkey integration, and the nil-client testing pattern.

---

## Initialization

The SDK client is created once at startup in `main.go`:

```go
opts := &sdk.ClientOptions{
    RequestTimeout:     3 * time.Minute, // ECDSA DKG can take 1-2 min
    PendingWaitTimeout: 3 * time.Minute,
}
sdkClient := sdk.NewClientWithOptions(serviceURL, opts)
```

- `serviceURL` comes from the `SERVICE_URL` environment variable (default: `http://localhost:8089`).
- Both timeouts are set to 3 minutes to accommodate ECDSA distributed key generation, which can take 1--2 minutes on the TEE cluster.
- The client is closed on shutdown via `defer sdkClient.Close()`.

After creation, the client loads its application identity:

```go
sdkClient.SetDefaultAppInstanceIDFromEnv()
```

This reads `APP_INSTANCE_ID` from the environment. If not set, a warning is logged and signing calls will require an explicit app instance ID.

The single `*sdk.Client` instance is passed by reference to every handler that needs signing or Passkey operations.

---

## Key Generation

Key generation is called during wallet creation in `handler/wallet.go`. The target SDK interface uses a unified method:

```go
keyResult, err := sdkClient.GenerateKey(ctx, scheme, curve)
```

| Chain family | Scheme | Curve | Example |
|---|---|---|---|
| EVM (Ethereum, Avalanche, etc.) | `ecdsa` | `secp256k1` | `GenerateKey(ctx, "ecdsa", "secp256k1")` |
| Solana | `ed25519` | `ed25519` | `GenerateKey(ctx, "ed25519", "ed25519")` |

> **Note:** The SDK key generation interface is being updated. The current code may still use `GenerateECDSAKey` / `GenerateSchnorrKey`. Verify the actual function signature against the code.

The result contains:

- `keyResult.PublicKey.Name` -- a unique key identifier stored on the wallet record as `KeyName`. This name is used in all subsequent signing calls.
- `keyResult.PublicKey.KeyData` -- the raw public key bytes, used to derive the chain address.
- `keyResult.Success` and `keyResult.Message` -- status fields for error handling.

The wallet record is initially created with status `"creating"` and a placeholder key name. After key generation succeeds, the record is updated to `"ready"` with the real key name, public key, and derived address.

---

## Signing

Signing is the most frequently called SDK operation. The interface is:

```go
result, err := sdkClient.Sign(ctx, msgBytes, keyName)
```

- `msgBytes` -- the raw message bytes to sign. For EVM transactions, this is the Keccak-256 hash of the RLP-encoded transaction. For Solana, this is the serialized transaction message.
- `keyName` -- the key identifier from the wallet record (`wallet.KeyName`), obtained during key generation.
- `result.Signature` -- the returned signature bytes.
- `result.Success` -- whether the signing succeeded.

Signing is used in three handlers:

| Handler | File | Context |
|---|---|---|
| `WalletHandler.signAndBroadcast` | `handler/wallet.go` | Native transfers, wrap/unwrap SOL |
| `ApprovalHandler.Approve` | `handler/approval.go` | Executing approved transactions |
| `ContractCallHandler` | `handler/contract_call.go` | Contract calls, token approvals, revocations |

After receiving the signature, the handler assembles the signed transaction and broadcasts it to the blockchain via the chain's RPC endpoint.

---

## Passkey Integration

The SDK handles all WebAuthn flows. The wallet calls these methods from `handler/auth.go` and `handler/approval.go`:

| Method | Purpose | Called from |
|---|---|---|
| `InvitePasskeyUser` | Create an invitation for a new Passkey user | `auth.go` (registration, invite) |
| `PasskeyRegistrationOptions` | Get WebAuthn registration challenge | `auth.go` (registration flow) |
| `PasskeyRegistrationVerify` | Verify the WebAuthn credential after registration | `auth.go` (registration flow) |
| `PasskeyLoginOptions` | Get WebAuthn login challenge | `auth.go` (login flow) |
| `PasskeyLoginVerify` | Verify a login assertion | `auth.go` (login flow) |
| `PasskeyLoginVerifyAs` | Verify a login assertion and confirm it matches a specific user | `approval.go` (approval confirmation) |
| `DeletePasskeyUser` | Remove a Passkey user from the TEENet service | `auth.go` (account deletion) |

The `PasskeyLoginVerifyAs` method is particularly important for the approval flow: it confirms both that the hardware key assertion is valid **and** that the person approving is the same user who owns the wallet.

---

## Key Deletion

When a wallet or user account is deleted, the corresponding TEE key is cleaned up:

```go
sdkClient.DeletePublicKey(ctx, keyName)
```

- Called in `handler/wallet.go` when deleting a single wallet.
- Called in `handler/auth.go` when deleting a user account (iterates over all of the user's wallets).
- Best-effort: failures are logged but do not block the deletion response.

For account deletion, the Passkey user is also removed:

```go
sdkClient.DeletePasskeyUser(ctx, passkeyUserID)
```

---

## Nil Client Pattern

In tests, the SDK client is set to `nil`:

```go
wh := handler.NewWalletHandler(db, nil, "http://localhost:8080")
```

Handlers proceed through all validation, database operations, and transaction building up to the point where they call `sdk.Sign()` or `sdk.GenerateKey()`. At that point, the nil pointer causes a panic or error, which is expected. This is by design:

- Tests verify **everything except** the actual TEE signing -- input validation, authorization checks, approval policy enforcement, database state transitions, and transaction construction.
- No mock server or SDK stub is needed for unit tests.
- Integration tests that need real signing use the mock server (see below).

---

## Mock Service

The `teenet-sdk/mock-server` stands in for the full TEENet service during local development:

```bash
cd teenet-sdk/mock-server
go build && ./mock-server    # listens on 127.0.0.1:8089
```

Then run the wallet pointing at it:

```bash
SERVICE_URL=http://127.0.0.1:8089 ./teenet-wallet
```

The mock server:

- Implements the same HTTP API as the production TEENet service (34 endpoints).
- Performs real cryptographic operations (ECDSA, Ed25519 signing).
- Uses deterministic keys for reproducible behavior.
- Supports direct, voting, and approval signing modes.
- Resets all state on restart (in-memory only).

**Do not use in production** -- deterministic keys are insecure.

See the [TEENet SDK mock-server documentation](https://github.com/TEENet-io/teenet-sdk/tree/main/mock-server) for the full list of endpoints and pre-configured test applications.

---

## Next Steps

- [Signing & TEE Trust Model](signing-tee.md) -- what happens inside the TEENet service when a signing request arrives
- [Architecture Overview](architecture-overview.md) -- the full system mental model
