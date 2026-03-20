package chain

import (
	"testing"
)

func TestFindProgramAddress(t *testing.T) {
	// Use the ATA program derivation as a realistic test case.
	// Seeds: [wallet_pubkey, TOKEN_PROGRAM_ID, mint_pubkey]
	// Program: ATA_PROGRAM_ID
	walletBytes, err := base58Decode("7v91N7iZ9mNicL8WfG6cgSCKyRXydQjLh6UYBWwm6y1Q")
	if err != nil {
		t.Fatalf("decode wallet: %v", err)
	}
	tokenProgram, err := base58Decode("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	if err != nil {
		t.Fatalf("decode token program: %v", err)
	}
	mint, err := base58Decode("So11111111111111111111111111111111111111112")
	if err != nil {
		t.Fatalf("decode mint: %v", err)
	}
	ataProgram, err := base58Decode("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	if err != nil {
		t.Fatalf("decode ata program: %v", err)
	}

	seeds := [][]byte{walletBytes, tokenProgram, mint}
	addr, bump, err := FindProgramAddress(seeds, ataProgram)
	if err != nil {
		t.Fatalf("FindProgramAddress: %v", err)
	}
	if len(addr) != 32 {
		t.Errorf("expected 32-byte address, got %d", len(addr))
	}
	if bump > 255 {
		t.Errorf("bump out of range: %d", bump)
	}
	t.Logf("PDA address: %s (bump=%d)", base58Encode(addr), bump)
}

func TestDeriveATA(t *testing.T) {
	// Derive ATA for a known wallet + WSOL mint.
	wallet := "7v91N7iZ9mNicL8WfG6cgSCKyRXydQjLh6UYBWwm6y1Q"
	mint := "So11111111111111111111111111111111111111112"

	ata, err := DeriveATA(wallet, mint)
	if err != nil {
		t.Fatalf("DeriveATA: %v", err)
	}
	if len(ata) != 32 {
		t.Errorf("expected 32-byte ATA, got %d", len(ata))
	}
	t.Logf("ATA address: %s", base58Encode(ata))

	// Calling again with the same inputs should produce the same result.
	ata2, err := DeriveATA(wallet, mint)
	if err != nil {
		t.Fatalf("DeriveATA second call: %v", err)
	}
	if base58Encode(ata) != base58Encode(ata2) {
		t.Errorf("DeriveATA not deterministic: %s != %s", base58Encode(ata), base58Encode(ata2))
	}
}

func TestBuildSOLProgramCallMessage(t *testing.T) {
	from := "11111111111111111111111111111111"
	programID := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	blockhash := "11111111111111111111111111111111"

	accounts := []SOLAccountMeta{}
	instrData := []byte{0x01, 0x02, 0x03}

	msg, err := buildSOLProgramCallMessage(from, programID, accounts, instrData, blockhash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg) == 0 {
		t.Fatal("expected non-empty message")
	}
	// Header byte 0 = numRequiredSignatures = 1 (just fee payer)
	if msg[0] != 1 {
		t.Errorf("expected 1 required sig (header byte 0), got %d", msg[0])
	}
}

func TestBuildSOLProgramCallMessage_DeduplicatesAccounts(t *testing.T) {
	from := "11111111111111111111111111111111"
	programID := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	blockhash := "11111111111111111111111111111111"

	// Include `from` in the accounts list — should be deduplicated with fee payer.
	accounts := []SOLAccountMeta{
		{Pubkey: from, IsSigner: true, IsWritable: true},
	}
	instrData := []byte{0xAA}

	msg, err := buildSOLProgramCallMessage(from, programID, accounts, instrData, blockhash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Total unique accounts should be 2: from + programID (not 3).
	// After header(3), next byte(s) are compact-u16 encoding of account count.
	numAccounts := int(msg[3]) // compact-u16: value <= 127 fits in 1 byte
	if numAccounts != 2 {
		t.Errorf("expected 2 unique accounts (from + program), got %d", numAccounts)
	}
}

func TestBuildSOLProgramCallMessage_InvalidAddress(t *testing.T) {
	programID := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	blockhash := "11111111111111111111111111111111"

	_, err := buildSOLProgramCallMessage("INVALIDADDR!!!", programID, nil, []byte{0x01}, blockhash)
	if err == nil {
		t.Fatal("expected error for invalid from address, got nil")
	}
}

func TestBuildSOLTokenTransferMessage(t *testing.T) {
	from := "11111111111111111111111111111111"
	to := "11111111111111111111111111111112"
	mint := "So11111111111111111111111111111111111111112"
	blockhash := "11111111111111111111111111111111"

	msg, err := buildSOLTokenTransferMessage(from, to, mint, 1000000, 6, blockhash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg) == 0 {
		t.Fatal("expected non-empty message")
	}
	// Header: 1 signer, 0 readonly signed, 5 readonly unsigned
	if msg[0] != 1 {
		t.Errorf("expected 1 required sig, got %d", msg[0])
	}
	if msg[1] != 0 {
		t.Errorf("expected 0 readonly signed, got %d", msg[1])
	}
	if msg[2] != 5 {
		t.Errorf("expected 5 readonly unsigned, got %d", msg[2])
	}
	// 8 accounts total (owner, srcATA, dstATA, mint, tokenProg, destWallet, sysProg, ataProg)
	if msg[3] != 8 {
		t.Errorf("expected 8 accounts, got %d", msg[3])
	}
}

func TestBuildSOLTokenTransferMessage_InvalidAddresses(t *testing.T) {
	blockhash := "11111111111111111111111111111111"
	_, err := buildSOLTokenTransferMessage("INVALID!!!", "11111111111111111111111111111112", "So11111111111111111111111111111111111111112", 100, 6, blockhash)
	if err == nil {
		t.Fatal("expected error for invalid from address")
	}
	_, err = buildSOLTokenTransferMessage("11111111111111111111111111111111", "INVALID!!!", "So11111111111111111111111111111111111111112", 100, 6, blockhash)
	if err == nil {
		t.Fatal("expected error for invalid to address")
	}
}

func TestBuildSOLWrapMessage(t *testing.T) {
	owner := "11111111111111111111111111111111"
	blockhash := "11111111111111111111111111111111"

	msg, err := buildSOLWrapMessage(owner, 100_000_000, blockhash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg) == 0 {
		t.Fatal("expected non-empty message")
	}
	// Header: 1 signer, 0 readonly signed, 4 readonly unsigned
	if msg[0] != 1 {
		t.Errorf("expected 1 required sig, got %d", msg[0])
	}
	if msg[1] != 0 {
		t.Errorf("expected 0 readonly signed, got %d", msg[1])
	}
	if msg[2] != 4 {
		t.Errorf("expected 4 readonly unsigned, got %d", msg[2])
	}
	// 6 accounts
	if msg[3] != 6 {
		t.Errorf("expected 6 accounts, got %d", msg[3])
	}
}

func TestBuildSOLWrapMessage_InvalidOwner(t *testing.T) {
	_, err := buildSOLWrapMessage("BADADDR!!!", 100, "11111111111111111111111111111111")
	if err == nil {
		t.Fatal("expected error for invalid owner address")
	}
}

func TestBuildSOLUnwrapMessage(t *testing.T) {
	owner := "11111111111111111111111111111111"
	blockhash := "11111111111111111111111111111111"

	msg, err := buildSOLUnwrapMessage(owner, blockhash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg) == 0 {
		t.Fatal("expected non-empty message")
	}
	// Header: 1 signer, 0 readonly signed, 1 readonly unsigned (token program)
	if msg[0] != 1 {
		t.Errorf("expected 1 required sig, got %d", msg[0])
	}
	if msg[2] != 1 {
		t.Errorf("expected 1 readonly unsigned, got %d", msg[2])
	}
	// 3 accounts (owner, wsolATA, tokenProgram)
	if msg[3] != 3 {
		t.Errorf("expected 3 accounts, got %d", msg[3])
	}
}

func TestBuildSOLUnwrapMessage_InvalidOwner(t *testing.T) {
	_, err := buildSOLUnwrapMessage("BADADDR!!!", "11111111111111111111111111111111")
	if err == nil {
		t.Fatal("expected error for invalid owner address")
	}
}

func TestBuildSOLWrapMessage_Deterministic(t *testing.T) {
	owner := "11111111111111111111111111111111"
	blockhash := "11111111111111111111111111111111"

	msg1, err := buildSOLWrapMessage(owner, 500_000_000, blockhash)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	msg2, err := buildSOLWrapMessage(owner, 500_000_000, blockhash)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(msg1) != len(msg2) {
		t.Fatalf("messages differ in length: %d vs %d", len(msg1), len(msg2))
	}
	for i := range msg1 {
		if msg1[i] != msg2[i] {
			t.Fatalf("messages differ at byte %d: %02x vs %02x", i, msg1[i], msg2[i])
		}
	}
}
