package chain

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

// ─── DeriveAddress — Ethereum ─────────────────────────────────────────────────

func TestDeriveAddress_Ethereum_Valid(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	compressed := crypto.CompressPubkey(&privKey.PublicKey)
	expected := crypto.PubkeyToAddress(privKey.PublicKey).Hex()

	got, err := DeriveAddress("evm", hex.EncodeToString(compressed))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expected {
		t.Errorf("got %s, want %s", got, expected)
	}
}

func TestDeriveAddress_Ethereum_WithHexPrefix(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	compressed := crypto.CompressPubkey(&privKey.PublicKey)
	expected := crypto.PubkeyToAddress(privKey.PublicKey).Hex()

	got, err := DeriveAddress("evm", "0x"+hex.EncodeToString(compressed))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expected {
		t.Errorf("got %s, want %s", got, expected)
	}
}

// ─── DeriveAddress — Solana ───────────────────────────────────────────────────

func TestDeriveAddress_Solana_32Bytes(t *testing.T) {
	// Deterministic 32-byte Ed25519 pubkey (arbitrary but non-zero)
	pubBytes := make([]byte, 32)
	for i := range pubBytes {
		pubBytes[i] = byte(i + 1)
	}
	addr, err := DeriveAddress("solana", hex.EncodeToString(pubBytes))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr == "" {
		t.Error("expected non-empty address")
	}
	// Verify all characters are valid base58
	for _, c := range addr {
		if !strings.ContainsRune(base58Alphabet, c) {
			t.Errorf("invalid base58 character %q in address %s", c, addr)
		}
	}
}

func TestDeriveAddress_Solana_33BytesMatchesRaw32(t *testing.T) {
	// A 33-byte pubkey (with 0x02 compression prefix) should yield the same
	// Solana address as the raw 32-byte pubkey after stripping the prefix.
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 7)
	}
	withPrefix := append([]byte{0x02}, raw...)

	addr32, err := DeriveAddress("solana", hex.EncodeToString(raw))
	if err != nil {
		t.Fatal(err)
	}
	addr33, err := DeriveAddress("solana", hex.EncodeToString(withPrefix))
	if err != nil {
		t.Fatal(err)
	}
	if addr32 != addr33 {
		t.Errorf("expected same address: raw=%s prefix=%s", addr32, addr33)
	}
}

func TestDeriveAddress_Solana_Deterministic(t *testing.T) {
	pubBytes := make([]byte, 32)
	for i := range pubBytes {
		pubBytes[i] = byte(i + 3)
	}
	hexPub := hex.EncodeToString(pubBytes)
	addr1, _ := DeriveAddress("solana", hexPub)
	addr2, _ := DeriveAddress("solana", hexPub)
	if addr1 != addr2 {
		t.Errorf("address derivation is not deterministic: %s vs %s", addr1, addr2)
	}
}

// ─── DeriveAddress — error cases ─────────────────────────────────────────────

func TestDeriveAddress_UnsupportedChain(t *testing.T) {
	_, err := DeriveAddress("bitcoin", "02"+strings.Repeat("ab", 32))
	if err == nil {
		t.Error("expected error for unsupported chain")
	}
}

func TestDeriveAddress_InvalidHex(t *testing.T) {
	_, err := DeriveAddress("evm", "zzzzzzzz")
	if err == nil {
		t.Error("expected error for invalid hex input")
	}
}

func TestDeriveAddress_Solana_WrongLength(t *testing.T) {
	// 10-byte pubkey should be rejected
	pubBytes := make([]byte, 10)
	_, err := DeriveAddress("solana", hex.EncodeToString(pubBytes))
	if err == nil {
		t.Error("expected error for wrong-length pubkey")
	}
}

// ─── base58 round-trip ────────────────────────────────────────────────────────

func TestBase58Roundtrip(t *testing.T) {
	cases := [][]byte{
		{1, 2, 3, 4, 5},
		{0, 0, 0, 1},
		{255, 254, 253},
	}
	for _, input := range cases {
		encoded := base58Encode(input)
		decoded, err := base58Decode(encoded)
		if err != nil {
			t.Fatalf("decode error for %v: %v", input, err)
		}
		if len(decoded) != len(input) {
			t.Errorf("length mismatch: got %d, want %d (input=%v)", len(decoded), len(input), input)
			continue
		}
		for i, b := range input {
			if decoded[i] != b {
				t.Errorf("byte mismatch at index %d: got %d, want %d", i, decoded[i], b)
			}
		}
	}
}

func TestBase58Roundtrip_WithLeadingZeros(t *testing.T) {
	// 3 leading zero bytes followed by non-zero data
	input := []byte{0, 0, 0, 42, 99, 128}
	encoded := base58Encode(input)
	decoded, err := base58Decode(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(decoded) != len(input) {
		t.Errorf("length mismatch: got %d, want %d", len(decoded), len(input))
	}
	for i, b := range input {
		if decoded[i] != b {
			t.Errorf("byte mismatch at %d: got %d, want %d", i, decoded[i], b)
		}
	}
}

func TestBase58Decode_InvalidChar(t *testing.T) {
	// '0', 'O', 'I', 'l' are excluded from the Bitcoin/Solana base58 alphabet
	for _, bad := range []string{"0abc", "Oabc", "Iabc", "labc"} {
		if _, err := base58Decode(bad); err == nil {
			t.Errorf("expected error for input containing excluded char: %q", bad)
		}
	}
}
