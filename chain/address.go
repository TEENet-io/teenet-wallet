package chain

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// DeriveAddress derives a chain address from a hex-encoded compressed public key.
// family must be "evm" or "solana".
func DeriveAddress(family, pubkeyHex string) (string, error) {
	pubkeyHex = strings.TrimPrefix(pubkeyHex, "0x")
	pubBytes, err := hex.DecodeString(pubkeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid pubkey hex: %w", err)
	}

	switch family {
	case "evm":
		return deriveEthAddress(pubBytes)
	case "solana":
		return deriveSolAddress(pubBytes)
	default:
		return "", fmt.Errorf("unsupported chain family: %s", family)
	}
}

// deriveEthAddress derives an EIP-55 checksummed Ethereum address from a compressed public key.
func deriveEthAddress(compressed []byte) (string, error) {
	pub, err := crypto.DecompressPubkey(compressed)
	if err != nil {
		return "", fmt.Errorf("decompress pubkey: %w", err)
	}
	addr := crypto.PubkeyToAddress(*pub)
	return addr.Hex(), nil
}

// deriveSolAddress derives a Solana address (Base58) from a 32-byte Ed25519 public key.
// Ed25519 public keys may arrive as 32 bytes (raw) or 33 bytes (with a leading 0x02/0x03 prefix).
func deriveSolAddress(pubBytes []byte) (string, error) {
	raw := pubBytes
	if len(raw) == 33 {
		// strip the compression prefix; Ed25519 is always on the curve
		raw = pubBytes[1:]
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("invalid ed25519 pubkey length: %d", len(raw))
	}
	return base58Encode(raw), nil
}

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// base58Decode decodes a base58-encoded string using Bitcoin/Solana alphabet.
func base58Decode(s string) ([]byte, error) {
	// Build reverse lookup table.
	reverseAlphabet := [256]int{}
	for i := range reverseAlphabet {
		reverseAlphabet[i] = -1
	}
	for i, c := range []byte(base58Alphabet) {
		reverseAlphabet[c] = i
	}

	n := new(big.Int)
	base := big.NewInt(58)
	for _, c := range []byte(s) {
		val := reverseAlphabet[c]
		if val < 0 {
			return nil, fmt.Errorf("invalid base58 character %q", c)
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(val)))
	}

	decoded := n.Bytes()

	// Prepend zero bytes for each leading '1' in the input.
	leadingOnes := 0
	for _, c := range []byte(s) {
		if c == '1' {
			leadingOnes++
		} else {
			break
		}
	}
	result := make([]byte, leadingOnes+len(decoded))
	copy(result[leadingOnes:], decoded)
	return result, nil
}

// base58Encode encodes bytes using Bitcoin/Solana Base58 alphabet.
func base58Encode(input []byte) string {
	// Count leading zero bytes.
	leadingZeros := 0
	for _, b := range input {
		if b != 0 {
			break
		}
		leadingZeros++
	}

	// Convert to big integer, then to base-58.
	digits := []int{0}
	for _, b := range input {
		carry := int(b)
		for i := range digits {
			carry += digits[i] << 8
			digits[i] = carry % 58
			carry /= 58
		}
		for carry > 0 {
			digits = append(digits, carry%58)
			carry /= 58
		}
	}

	result := make([]byte, leadingZeros+len(digits))
	for i := range leadingZeros {
		result[i] = base58Alphabet[0]
	}
	for i, d := range digits {
		result[len(result)-1-i] = base58Alphabet[d]
	}
	return string(result)
}
