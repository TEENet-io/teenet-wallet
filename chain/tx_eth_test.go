package chain

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// ─── EncodeERC20Transfer ──────────────────────────────────────────────────────

func TestEncodeERC20Transfer_Length(t *testing.T) {
	data := EncodeERC20Transfer("0x742d35Cc6634C0532925a3b844Bc454e4438f44e", big.NewInt(1))
	if len(data) != 68 {
		t.Fatalf("expected 68 bytes (4 selector + 32 addr + 32 amount), got %d", len(data))
	}
}

func TestEncodeERC20Transfer_Selector(t *testing.T) {
	data := EncodeERC20Transfer("0x742d35Cc6634C0532925a3b844Bc454e4438f44e", big.NewInt(1))
	want := crypto.Keccak256([]byte("transfer(address,uint256)"))[:4]
	if !bytes.Equal(data[:4], want) {
		t.Fatalf("selector mismatch: got %x, want %x", data[:4], want)
	}
}

func TestEncodeERC20Transfer_AddressPadding(t *testing.T) {
	addr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	data := EncodeERC20Transfer(addr, big.NewInt(1))

	// Bytes 4..15 must be zero (12-byte left-pad of 20-byte address).
	for i := 4; i < 16; i++ {
		if data[i] != 0 {
			t.Fatalf("expected zero padding at byte %d, got 0x%02x", i, data[i])
		}
	}

	// Bytes 16..35 must match the raw address bytes.
	want := common.HexToAddress(addr).Bytes()
	if !bytes.Equal(data[16:36], want) {
		t.Fatalf("address bytes mismatch: got %x, want %x", data[16:36], want)
	}
}

func TestEncodeERC20Transfer_AmountPadding(t *testing.T) {
	amount := big.NewInt(1_000_000) // 1 USDC with 6 decimals
	data := EncodeERC20Transfer("0x742d35Cc6634C0532925a3b844Bc454e4438f44e", amount)

	want := make([]byte, 32)
	ab := amount.Bytes()
	copy(want[32-len(ab):], ab)

	if !bytes.Equal(data[36:], want) {
		t.Fatalf("amount mismatch: got %x, want %x", data[36:], want)
	}
}

func TestEncodeERC20Transfer_LargeAmount(t *testing.T) {
	// 1 WETH = 1e18 wei — 18-byte big.Int, fits in 32-byte padding
	weth, _ := new(big.Int).SetString("1000000000000000000", 10)
	data := EncodeERC20Transfer("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", weth)

	if len(data) != 68 {
		t.Fatalf("expected 68 bytes, got %d", len(data))
	}
	want := make([]byte, 32)
	ab := weth.Bytes()
	copy(want[32-len(ab):], ab)
	if !bytes.Equal(data[36:], want) {
		t.Fatalf("large amount encoding wrong")
	}
}

func TestEncodeERC20Transfer_ZeroAmount(t *testing.T) {
	// Zero is a valid calldata value (uint256 = 0x00...00).
	data := EncodeERC20Transfer("0x742d35Cc6634C0532925a3b844Bc454e4438f44e", big.NewInt(0))
	if len(data) != 68 {
		t.Fatalf("expected 68 bytes, got %d", len(data))
	}
	// Last 32 bytes should all be zero.
	for i := 36; i < 68; i++ {
		if data[i] != 0 {
			t.Fatalf("expected zero amount at byte %d, got 0x%02x", i, data[i])
		}
	}
}

// ─── EncodeERC20Approve ───────────────────────────────────────────────────────

func TestEncodeERC20Approve_Length(t *testing.T) {
	data := EncodeERC20Approve("0x742d35Cc6634C0532925a3b844Bc454e4438f44e", big.NewInt(1))
	if len(data) != 68 {
		t.Fatalf("expected 68 bytes, got %d", len(data))
	}
}

func TestEncodeERC20Approve_Selector(t *testing.T) {
	data := EncodeERC20Approve("0x742d35Cc6634C0532925a3b844Bc454e4438f44e", big.NewInt(1))
	want := crypto.Keccak256([]byte("approve(address,uint256)"))[:4]
	if !bytes.Equal(data[:4], want) {
		t.Fatalf("approve selector mismatch: got %x, want %x", data[:4], want)
	}
}

func TestEncodeERC20Approve_DifferFromTransfer(t *testing.T) {
	addr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	amount := big.NewInt(999)
	transfer := EncodeERC20Transfer(addr, amount)
	approve := EncodeERC20Approve(addr, amount)
	// Selectors must differ; rest of encoding should be identical.
	if bytes.Equal(transfer[:4], approve[:4]) {
		t.Fatal("transfer and approve must have different selectors")
	}
	if !bytes.Equal(transfer[4:], approve[4:]) {
		t.Fatal("arguments encoding for same addr/amount should be identical")
	}
}

func TestEncodeERC20Approve_MaxUint256(t *testing.T) {
	// Unlimited approval: max uint256
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	data := EncodeERC20Approve("0x742d35Cc6634C0532925a3b844Bc454e4438f44e", max)
	if len(data) != 68 {
		t.Fatalf("expected 68 bytes, got %d", len(data))
	}
	// Last 32 bytes should all be 0xFF.
	for i := 36; i < 68; i++ {
		if data[i] != 0xFF {
			t.Fatalf("expected 0xFF at byte %d for max uint256, got 0x%02x", i, data[i])
		}
	}
}
