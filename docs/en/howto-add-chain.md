# How To: Add a Chain

## Checklist

1. **Add an entry to `chains.json`** -- see [chains.json Schema](chains-schema.md) for the full field specification. Changes require a service restart; `chains.json` is loaded once at startup.

2. **Add a CoinGecko price feed mapping** in `handler/price.go` -- look for the `coinGeckoIDs` map and add the native currency symbol.

3. **Add a CoinGecko platform ID** in `handler/price.go` -- look for `coinGeckoPlatformIDs` to enable token pricing on that chain.

4. **Standard EVM chains:** no code changes needed beyond the steps above.

5. **Solana-family chains:** requires changes to `chain/tx_sol.go` for transaction building logic.

> **Watch out:** Price lookup fails silently without a CoinGecko mapping. Transfers will require approval for ALL amounts because the USD value is unknown (fail-closed behavior).
