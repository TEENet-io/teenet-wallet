package chain

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

// ─── EncodeCall tests ────────────────────────────────────────────────────────

func TestEncodeCall_TransferSelector(t *testing.T) {
	data, err := EncodeCall("transfer(address,uint256)", []interface{}{
		"0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
		big.NewInt(1000000),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := crypto.Keccak256([]byte("transfer(address,uint256)"))[:4]
	if !bytes.Equal(data[:4], want) {
		t.Fatalf("selector mismatch: got %x, want %x", data[:4], want)
	}
}

func TestEncodeCall_TransferLength(t *testing.T) {
	data, err := EncodeCall("transfer(address,uint256)", []interface{}{
		"0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
		big.NewInt(1000000),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 68 {
		t.Fatalf("expected 68 bytes (4+32+32), got %d", len(data))
	}
}

func TestEncodeCall_MatchesERC20Transfer(t *testing.T) {
	addr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	amount := big.NewInt(1_000_000)

	want := EncodeERC20Transfer(addr, amount)
	got, err := EncodeCall("transfer(address,uint256)", []interface{}{addr, amount})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("EncodeCall output differs from EncodeERC20Transfer:\ngot:  %x\nwant: %x", got, want)
	}
}

func TestEncodeCall_MatchesERC20Approve(t *testing.T) {
	addr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	amount, _ := new(big.Int).SetString("115792089237316195423570985008687907853269984665640564039457584007913129639935", 10) // max uint256

	want := EncodeERC20Approve(addr, amount)
	got, err := EncodeCall("approve(address,uint256)", []interface{}{addr, amount})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("EncodeCall output differs from EncodeERC20Approve:\ngot:  %x\nwant: %x", got, want)
	}
}

func TestEncodeCall_BoolArg(t *testing.T) {
	// true -> 0x...01
	dataTrue, err := EncodeCall("setFlag(bool)", []interface{}{true})
	if err != nil {
		t.Fatal(err)
	}
	if len(dataTrue) != 36 {
		t.Fatalf("expected 36 bytes, got %d", len(dataTrue))
	}
	if dataTrue[35] != 1 {
		t.Fatalf("expected last byte 1 for true, got %d", dataTrue[35])
	}
	// bytes 4..34 should be zero
	for i := 4; i < 35; i++ {
		if dataTrue[i] != 0 {
			t.Fatalf("expected zero at byte %d for bool true, got %d", i, dataTrue[i])
		}
	}

	// false -> 0x...00
	dataFalse, err := EncodeCall("setFlag(bool)", []interface{}{false})
	if err != nil {
		t.Fatal(err)
	}
	for i := 4; i < 36; i++ {
		if dataFalse[i] != 0 {
			t.Fatalf("expected zero at byte %d for bool false, got %d", i, dataFalse[i])
		}
	}
}

func TestEncodeCall_Bytes32(t *testing.T) {
	hexVal := "0x" + "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	data, err := EncodeCall("store(bytes32)", []interface{}{hexVal})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 36 {
		t.Fatalf("expected 36 bytes, got %d", len(data))
	}
	// The word should start with 0xab
	if data[4] != 0xab {
		t.Fatalf("expected first data byte 0xab, got 0x%02x", data[4])
	}
}

func TestEncodeCall_Uint256FromString(t *testing.T) {
	addr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	amount := "1000000"
	amountBig := big.NewInt(1_000_000)

	want := EncodeERC20Transfer(addr, amountBig)
	got, err := EncodeCall("transfer(address,uint256)", []interface{}{addr, amount})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("string uint256 mismatch:\ngot:  %x\nwant: %x", got, want)
	}
}

func TestEncodeCall_Uint256FromFloat64(t *testing.T) {
	addr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	amount := float64(1000000)
	amountBig := big.NewInt(1_000_000)

	want := EncodeERC20Transfer(addr, amountBig)
	got, err := EncodeCall("transfer(address,uint256)", []interface{}{addr, amount})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("float64 uint256 mismatch:\ngot:  %x\nwant: %x", got, want)
	}
}

func TestEncodeCall_Uint256FromJsonNumber(t *testing.T) {
	addr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	amount := json.Number("1000000")
	amountBig := big.NewInt(1_000_000)

	want := EncodeERC20Transfer(addr, amountBig)
	got, err := EncodeCall("transfer(address,uint256)", []interface{}{addr, amount})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("json.Number uint256 mismatch:\ngot:  %x\nwant: %x", got, want)
	}
}

func TestEncodeCall_Int256Negative(t *testing.T) {
	// -1 in two's complement (256-bit) is all 0xFF
	data, err := EncodeCall("setVal(int256)", []interface{}{big.NewInt(-1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 36 {
		t.Fatalf("expected 36 bytes, got %d", len(data))
	}
	for i := 4; i < 36; i++ {
		if data[i] != 0xFF {
			t.Fatalf("expected 0xFF at byte %d for int256(-1), got 0x%02x", i, data[i])
		}
	}
}

func TestEncodeCall_WrongArgCount(t *testing.T) {
	_, err := EncodeCall("transfer(address,uint256)", []interface{}{
		"0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
	})
	if err == nil {
		t.Fatal("expected error for wrong arg count")
	}
}

func TestEncodeCall_NoArgs(t *testing.T) {
	data, err := EncodeCall("pause()", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 4 {
		t.Fatalf("expected 4 bytes for no-arg call, got %d", len(data))
	}
	want := crypto.Keccak256([]byte("pause()"))[:4]
	if !bytes.Equal(data, want) {
		t.Fatalf("selector mismatch: got %x, want %x", data, want)
	}
}

func TestEncodeCall_ThreeArgs(t *testing.T) {
	// transferFrom(address,address,uint256)
	data, err := EncodeCall("transferFrom(address,address,uint256)", []interface{}{
		"0x1111111111111111111111111111111111111111",
		"0x2222222222222222222222222222222222222222",
		big.NewInt(500),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 4+32*3 {
		t.Fatalf("expected %d bytes, got %d", 4+32*3, len(data))
	}
	want := crypto.Keccak256([]byte("transferFrom(address,address,uint256)"))[:4]
	if !bytes.Equal(data[:4], want) {
		t.Fatalf("selector mismatch: got %x, want %x", data[:4], want)
	}
}

func TestEncodeCall_UnsupportedType(t *testing.T) {
	_, err := EncodeCall("foo(fixed128x18)", []interface{}{"1.0"})
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}
