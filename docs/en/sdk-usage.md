# TEENet SDK Usage

This page documents the TEENet SDK interface (`github.com/TEENet-io/teenet-sdk/go`) as used by the wallet. It covers initialization, key generation, signing, Passkey integration, and the nil-client testing pattern.

---

## Initialization

The SDK client is created once at startup:

```go
sdkClient := sdk.NewClient()
defer sdkClient.Close()
```

`SERVICE_URL` and `APP_INSTANCE_ID` are read from environment variables automatically.

---

## Key Generation

```go
keyResult, err := sdkClient.GenerateKey(ctx, scheme, curve)
```

| Chain family | Scheme | Curve | Example |
|---|---|---|---|
| EVM (Ethereum, Avalanche, etc.) | `ecdsa` | `secp256k1` | `GenerateKey(ctx, "ecdsa", "secp256k1")` |
| Solana | `eddsa` | `ed25519` | `GenerateKey(ctx, "eddsa", "ed25519")` |
| Bitcoin Taproot (P2TR) | `schnorr-bip340` | `secp256k1` | `GenerateKey(ctx, "schnorr-bip340", "secp256k1")` |

Returns:

- `keyResult.PublicKey.Name` -- unique key identifier, used in all subsequent signing calls.
- `keyResult.PublicKey.KeyData` -- raw public key bytes, used to derive the chain address.
- `keyResult.Success` and `keyResult.Message` -- status fields.

---

## Signing

```go
result, err := sdkClient.Sign(ctx, msgBytes, keyName)
```

- `msgBytes` -- raw message bytes to sign.
- `keyName` -- key identifier from `GenerateKey`.
- `result.Signature` -- the returned signature bytes.
- `result.Success` -- whether the signing succeeded.

---

## Passkey Integration

The SDK handles all WebAuthn flows:

| Method | Purpose |
|---|---|
| `InvitePasskeyUser` | Create an invitation for a new Passkey user |
| `PasskeyRegistrationOptions` | Get WebAuthn registration challenge |
| `PasskeyRegistrationVerify` | Verify the WebAuthn credential after registration |
| `PasskeyLoginOptions` | Get WebAuthn login challenge |
| `PasskeyLoginVerify` | Verify a login assertion |
| `PasskeyLoginVerifyAs` | Verify a login assertion and confirm it matches a specific user |
| `DeletePasskeyUser` | Remove a Passkey user from the TEENet service |

`PasskeyLoginVerifyAs` is used during approval: it confirms both that the hardware key assertion is valid **and** that the person approving is the wallet owner.

---

## Key Deletion

```go
sdkClient.DeletePublicKey(ctx, keyName)
sdkClient.DeletePasskeyUser(ctx, passkeyUserID)
```

---

## Nil Client Pattern

In tests, the SDK client is set to `nil`. Handlers proceed through validation, database operations, and transaction building, then fail at the signing step. This is by design — tests verify everything except the actual signing. Integration tests that need real signing use the mock service.

---

## Mock Service

The `teenet-sdk/mock-server` stands in for the TEENet service during local development:

```bash
cd teenet-sdk/mock-server
make run    # listens on 127.0.0.1:8089 with localhost Passkey defaults
```

- Implements the same HTTP API as the production TEENet service.
- Performs real cryptographic operations (ECDSA and EdDSA / ed25519 signing).
- Uses deterministic keys for reproducible behavior.
- Resets all state on restart (in-memory only).

**Do not use in production** -- deterministic keys are insecure.

---

## Next Steps

- [Signing & TEE Trust Model](signing-tee.md) -- what happens inside the TEENet service
- [Architecture Overview](architecture-overview.md) -- system mental model
