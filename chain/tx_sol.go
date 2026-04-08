// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package chain

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
)

// systemProgramID is the Solana system program address (all zeros).
var systemProgramID = [32]byte{}

// splTokenProgramID is the SPL Token program address.
const splTokenProgramID = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"

// ataProgramID is the Associated Token Account program address.
const ataProgramID = "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"

// SOLTxParams contains all fields needed to reconstruct and broadcast a SOL transaction after signing.
type SOLTxParams struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Lamports  uint64 `json:"lamports"`
	Blockhash string `json:"blockhash"` // base58-encoded recent blockhash
}

// SOLTxData is returned by BuildSOLTx.
type SOLTxData struct {
	Params       SOLTxParams
	MessageBytes []byte // serialized Solana message — this is what gets signed
}

// BuildSOLTx queries the chain for a recent blockhash and constructs a Solana system transfer message.
// The returned MessageBytes is the exact byte sequence that must be signed by the Ed25519 key.
func BuildSOLTx(rpcURL, fromAddr, toAddr string, amountSOL float64) (*SOLTxData, error) {
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}
	lamportsBig := new(big.Float).SetFloat64(amountSOL)
	lamportsBig.Mul(lamportsBig, new(big.Float).SetFloat64(1e9))
	lamportsInt, _ := lamportsBig.Uint64()
	lamports := lamportsInt
	if lamports == 0 {
		return nil, fmt.Errorf("amount too small (rounds to 0 lamports)")
	}

	// Query recent blockhash.
	result, err := jsonRPCWithRetry(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"method": "getLatestBlockhash",
		"params": []interface{}{map[string]interface{}{"commitment": "finalized"}},
	})
	if err != nil {
		return nil, fmt.Errorf("get blockhash: %w", err)
	}
	resMap, _ := result["result"].(map[string]interface{})
	valMap, _ := resMap["value"].(map[string]interface{})
	blockhash, _ := valMap["blockhash"].(string)
	if blockhash == "" {
		return nil, fmt.Errorf("empty blockhash from RPC")
	}

	msgBytes, err := buildSOLMessage(fromAddr, toAddr, blockhash, lamports)
	if err != nil {
		return nil, err
	}

	return &SOLTxData{
		Params:       SOLTxParams{From: fromAddr, To: toAddr, Lamports: lamports, Blockhash: blockhash},
		MessageBytes: msgBytes,
	}, nil
}

// AssembleAndBroadcastSOL assembles a signed Solana transaction and broadcasts it.
// sig must be exactly 64 bytes (Ed25519 signature over the message bytes).
func AssembleAndBroadcastSOL(rpcURL string, params SOLTxParams, sig []byte) (string, error) {
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}
	if len(sig) != 64 {
		return "", fmt.Errorf("invalid Ed25519 signature length: %d (expected 64)", len(sig))
	}

	msgBytes, err := buildSOLMessage(params.From, params.To, params.Blockhash, params.Lamports)
	if err != nil {
		return "", err
	}

	// Legacy transaction format: [compact-u16(1), sig(64 bytes), message]
	txBytes := make([]byte, 0, 1+64+len(msgBytes))
	txBytes = append(txBytes, 0x01) // compact-u16: 1 signature
	txBytes = append(txBytes, sig[:64]...)
	txBytes = append(txBytes, msgBytes...)

	txB64 := base64.StdEncoding.EncodeToString(txBytes)
	result, err := jsonRPC(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"method": "sendTransaction",
		"params": []interface{}{txB64, map[string]interface{}{"encoding": "base64"}},
	})
	if err != nil {
		return "", fmt.Errorf("broadcast: %w", err)
	}
	txSig, _ := result["result"].(string)
	return txSig, nil
}

// buildSOLMessage constructs the binary Solana transaction message for a system program transfer.
//
// Message layout (legacy format):
//
//	Header:        [1, 0, 1]  (1 required sig, 0 readonly signed, 1 readonly unsigned)
//	Account keys:  compact-u16(3), from(32), to(32), system_program(32)
//	Blockhash:     32 bytes
//	Instructions:  compact-u16(1), {program_idx=2, accounts=[0,1], data=[2,0,0,0,lamports_le_u64]}
func buildSOLMessage(fromAddr, toAddr, blockhash string, lamports uint64) ([]byte, error) {
	fromPub, err := base58Decode(fromAddr)
	if err != nil || len(fromPub) != 32 {
		return nil, fmt.Errorf("invalid from address %q: %v", fromAddr, err)
	}
	toPub, err := base58Decode(toAddr)
	if err != nil || len(toPub) != 32 {
		return nil, fmt.Errorf("invalid to address %q: %v", toAddr, err)
	}
	bhBytes, err := base58Decode(blockhash)
	if err != nil || len(bhBytes) != 32 {
		return nil, fmt.Errorf("invalid blockhash %q: %v", blockhash, err)
	}

	// System program Transfer instruction data: 4-byte discriminant (2 as u32 LE) + 8-byte lamports (u64 LE).
	instrData := make([]byte, 12)
	binary.LittleEndian.PutUint32(instrData[:4], 2)
	binary.LittleEndian.PutUint64(instrData[4:], lamports)

	var msg []byte
	// Header.
	msg = append(msg, 1, 0, 1)
	// Account keys (3 accounts).
	msg = append(msg, compactU16(3)...)
	msg = append(msg, fromPub...)
	msg = append(msg, toPub...)
	msg = append(msg, systemProgramID[:]...)
	// Recent blockhash.
	msg = append(msg, bhBytes...)
	// Instructions (1 instruction).
	msg = append(msg, compactU16(1)...)
	msg = append(msg, 2)                         // program_id_index = 2 (system program)
	msg = append(msg, compactU16(2)...)           // 2 account indices
	msg = append(msg, 0, 1)                      // account indices: from=0, to=1
	msg = append(msg, compactU16(len(instrData))...)
	msg = append(msg, instrData...)

	return msg, nil
}

// SOLTokenTransferParams contains all fields needed to reconstruct and broadcast an SPL token transfer.
type SOLTokenTransferParams struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Mint      string `json:"mint"`
	Amount    uint64 `json:"amount"`
	Decimals  int    `json:"decimals"`
	Blockhash string `json:"blockhash"`
}

// SOLTokenTransferTxData is returned by BuildSOLTokenTransferTx.
type SOLTokenTransferTxData struct {
	Params       SOLTokenTransferParams
	MessageBytes []byte // serialized Solana message — this is what gets signed
}

// BuildSOLTokenTransferTx queries the chain for a recent blockhash and constructs a Solana SPL Token
// TransferChecked message. The returned MessageBytes is the exact byte sequence that must be signed.
func BuildSOLTokenTransferTx(rpcURL, fromAddr, toAddr, mintAddr string, amount uint64, decimals int) (*SOLTokenTransferTxData, error) {
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}

	// Query recent blockhash.
	result, err := jsonRPCWithRetry(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"method": "getLatestBlockhash",
		"params": []interface{}{map[string]interface{}{"commitment": "finalized"}},
	})
	if err != nil {
		return nil, fmt.Errorf("get blockhash: %w", err)
	}
	resMap, _ := result["result"].(map[string]interface{})
	valMap, _ := resMap["value"].(map[string]interface{})
	blockhash, _ := valMap["blockhash"].(string)
	if blockhash == "" {
		return nil, fmt.Errorf("empty blockhash from RPC")
	}

	msgBytes, err := buildSOLTokenTransferMessage(fromAddr, toAddr, mintAddr, amount, decimals, blockhash)
	if err != nil {
		return nil, err
	}

	return &SOLTokenTransferTxData{
		Params: SOLTokenTransferParams{
			From: fromAddr, To: toAddr, Mint: mintAddr,
			Amount: amount, Decimals: decimals, Blockhash: blockhash,
		},
		MessageBytes: msgBytes,
	}, nil
}

// AssembleAndBroadcastSOLToken assembles a signed SPL token transfer transaction and broadcasts it.
// sig must be exactly 64 bytes (Ed25519 signature over the message bytes).
func AssembleAndBroadcastSOLToken(rpcURL string, params SOLTokenTransferParams, sig []byte) (string, error) {
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}
	if len(sig) != 64 {
		return "", fmt.Errorf("invalid Ed25519 signature length: %d (expected 64)", len(sig))
	}

	msgBytes, err := buildSOLTokenTransferMessage(params.From, params.To, params.Mint, params.Amount, params.Decimals, params.Blockhash)
	if err != nil {
		return "", err
	}

	// Legacy transaction format: [compact-u16(1), sig(64 bytes), message]
	txBytes := make([]byte, 0, 1+64+len(msgBytes))
	txBytes = append(txBytes, 0x01) // compact-u16: 1 signature
	txBytes = append(txBytes, sig[:64]...)
	txBytes = append(txBytes, msgBytes...)

	txB64 := base64.StdEncoding.EncodeToString(txBytes)
	result, err := jsonRPC(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"method": "sendTransaction",
		"params": []interface{}{txB64, map[string]interface{}{"encoding": "base64"}},
	})
	if err != nil {
		return "", fmt.Errorf("broadcast: %w", err)
	}
	txSig, _ := result["result"].(string)
	return txSig, nil
}

// RebuildSOLTokenTransferTx refreshes the blockhash for a previously-built SPL token transfer transaction.
func RebuildSOLTokenTransferTx(rpcURL string, params SOLTokenTransferParams) (*SOLTokenTransferTxData, error) {
	return BuildSOLTokenTransferTx(rpcURL, params.From, params.To, params.Mint, params.Amount, params.Decimals)
}

// buildSOLTokenTransferMessage constructs the binary Solana transaction message for an SPL Token TransferChecked instruction,
// preceded by a CreateAssociatedTokenAccountIdempotent instruction to auto-create the destination ATA if needed.
//
// Message layout (legacy format):
//
//	Header:        [1, 0, 5]  (1 required sig, 0 readonly signed, 5 readonly unsigned)
//	Account keys:  compact-u16(8): owner, srcATA, dstATA, mint, tokenProgram, destWallet, systemProgram, ataProgram
//	Blockhash:     32 bytes
//	Instructions:  compact-u16(2):
//	  1. CreateAssociatedTokenAccountIdempotent (ataProgram): accounts=[0,2,5,3,6,4], data=[1]
//	  2. TransferChecked (tokenProgram): accounts=[1,3,2,0], data=[0x0C, amount_u64_LE, decimals_u8]
func buildSOLTokenTransferMessage(fromAddr, toAddr, mintAddr string, amount uint64, decimals int, blockhash string) ([]byte, error) {
	// Derive source and destination ATAs.
	srcATA, err := DeriveATA(fromAddr, mintAddr)
	if err != nil {
		return nil, fmt.Errorf("derive source ATA: %w", err)
	}
	dstATA, err := DeriveATA(toAddr, mintAddr)
	if err != nil {
		return nil, fmt.Errorf("derive dest ATA: %w", err)
	}

	// Decode public keys.
	ownerPub, err := base58Decode(fromAddr)
	if err != nil || len(ownerPub) != 32 {
		return nil, fmt.Errorf("invalid from address %q: %v", fromAddr, err)
	}
	destPub, err := base58Decode(toAddr)
	if err != nil || len(destPub) != 32 {
		return nil, fmt.Errorf("invalid to address %q: %v", toAddr, err)
	}
	mintPub, err := base58Decode(mintAddr)
	if err != nil || len(mintPub) != 32 {
		return nil, fmt.Errorf("invalid mint address %q: %v", mintAddr, err)
	}
	tokenProgramPub, err := base58Decode(splTokenProgramID)
	if err != nil || len(tokenProgramPub) != 32 {
		return nil, fmt.Errorf("invalid token program ID: %v", err)
	}
	ataProgramPub, err := base58Decode(ataProgramID)
	if err != nil || len(ataProgramPub) != 32 {
		return nil, fmt.Errorf("invalid ATA program ID: %v", err)
	}
	bhBytes, err := base58Decode(blockhash)
	if err != nil || len(bhBytes) != 32 {
		return nil, fmt.Errorf("invalid blockhash %q: %v", blockhash, err)
	}

	// TransferChecked instruction data: [0x0C, amount(u64 LE), decimals(u8)]  = 10 bytes
	instrData := make([]byte, 10)
	instrData[0] = 0x0C
	binary.LittleEndian.PutUint64(instrData[1:9], amount)
	instrData[9] = byte(decimals)

	var msg []byte
	// Header: 1 signer, 0 readonly signed, 5 readonly unsigned (mint, tokenProgram, destWallet, systemProgram, ataProgram).
	msg = append(msg, 1, 0, 5)
	// Account keys (8 accounts).
	msg = append(msg, compactU16(8)...)
	msg = append(msg, ownerPub...)              // index 0: owner (signer + writable)
	msg = append(msg, srcATA...)                // index 1: source ATA (writable)
	msg = append(msg, dstATA...)                // index 2: dest ATA (writable)
	msg = append(msg, mintPub...)               // index 3: mint (readonly)
	msg = append(msg, tokenProgramPub...)       // index 4: token program (readonly)
	msg = append(msg, destPub...)               // index 5: dest wallet (readonly)
	msg = append(msg, systemProgramID[:]...)    // index 6: system program (readonly)
	msg = append(msg, ataProgramPub...)         // index 7: ATA program (readonly)
	// Recent blockhash.
	msg = append(msg, bhBytes...)
	// Instructions (2 instructions).
	msg = append(msg, compactU16(2)...)

	// Instruction 1: CreateAssociatedTokenAccountIdempotent
	msg = append(msg, 7)                         // program_id_index = 7 (ATA program)
	msg = append(msg, compactU16(6)...)           // 6 account indices
	msg = append(msg, 0, 2, 5, 3, 6, 4)          // payer, dstATA, destWallet, mint, systemProgram, tokenProgram
	msg = append(msg, compactU16(1)...)           // 1 byte data
	msg = append(msg, 1)                          // instruction discriminator: 1 = idempotent create

	// Instruction 2: TransferChecked
	msg = append(msg, 4)                         // program_id_index = 4 (token program)
	msg = append(msg, compactU16(4)...)           // 4 account indices
	msg = append(msg, 1, 3, 2, 0)                // account indices: srcATA, mint, dstATA, owner
	msg = append(msg, compactU16(len(instrData))...)
	msg = append(msg, instrData...)

	return msg, nil
}

// SOLAccountMeta describes a single account input for a Solana instruction.
type SOLAccountMeta struct {
	Pubkey     string `json:"pubkey"`
	IsSigner   bool   `json:"is_signer"`
	IsWritable bool   `json:"is_writable"`
}

// SOLProgramCallParams contains all fields needed to reconstruct and broadcast a generic Solana program call.
type SOLProgramCallParams struct {
	From      string           `json:"from"`
	ProgramID string           `json:"program_id"`
	Accounts  []SOLAccountMeta `json:"accounts"`
	Data      string           `json:"data"`      // hex-encoded instruction data
	Blockhash string           `json:"blockhash"`
}

// SOLProgramCallTxData is returned by BuildSOLProgramCallTx.
type SOLProgramCallTxData struct {
	Params       SOLProgramCallParams
	MessageBytes []byte // serialized Solana message — this is what gets signed
}

// BuildSOLProgramCallTx queries the chain for a recent blockhash and constructs a generic Solana program call message.
func BuildSOLProgramCallTx(rpcURL, fromAddr, programID string, accounts []SOLAccountMeta, instrData []byte) (*SOLProgramCallTxData, error) {
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}

	// Query recent blockhash.
	result, err := jsonRPCWithRetry(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"method": "getLatestBlockhash",
		"params": []interface{}{map[string]interface{}{"commitment": "finalized"}},
	})
	if err != nil {
		return nil, fmt.Errorf("get blockhash: %w", err)
	}
	resMap, _ := result["result"].(map[string]interface{})
	valMap, _ := resMap["value"].(map[string]interface{})
	blockhash, _ := valMap["blockhash"].(string)
	if blockhash == "" {
		return nil, fmt.Errorf("empty blockhash from RPC")
	}

	msgBytes, err := buildSOLProgramCallMessage(fromAddr, programID, accounts, instrData, blockhash)
	if err != nil {
		return nil, err
	}

	return &SOLProgramCallTxData{
		Params: SOLProgramCallParams{
			From:      fromAddr,
			ProgramID: programID,
			Accounts:  accounts,
			Data:      hex.EncodeToString(instrData),
			Blockhash: blockhash,
		},
		MessageBytes: msgBytes,
	}, nil
}

// AssembleAndBroadcastSOLProgram assembles a signed generic Solana program call transaction and broadcasts it.
// sig must be exactly 64 bytes (Ed25519 signature over the message bytes).
func AssembleAndBroadcastSOLProgram(rpcURL string, params SOLProgramCallParams, sig []byte) (string, error) {
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}
	if len(sig) != 64 {
		return "", fmt.Errorf("invalid Ed25519 signature length: %d (expected 64)", len(sig))
	}

	instrData, err := hex.DecodeString(params.Data)
	if err != nil {
		return "", fmt.Errorf("decode instruction data hex: %w", err)
	}

	msgBytes, err := buildSOLProgramCallMessage(params.From, params.ProgramID, params.Accounts, instrData, params.Blockhash)
	if err != nil {
		return "", err
	}

	// Legacy transaction format: [compact-u16(1), sig(64 bytes), message]
	txBytes := make([]byte, 0, 1+64+len(msgBytes))
	txBytes = append(txBytes, 0x01) // compact-u16: 1 signature
	txBytes = append(txBytes, sig[:64]...)
	txBytes = append(txBytes, msgBytes...)

	txB64 := base64.StdEncoding.EncodeToString(txBytes)
	result, err := jsonRPC(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"method": "sendTransaction",
		"params": []interface{}{txB64, map[string]interface{}{"encoding": "base64"}},
	})
	if err != nil {
		return "", fmt.Errorf("broadcast: %w", err)
	}
	txSig, _ := result["result"].(string)
	return txSig, nil
}

// RebuildSOLProgramCallTx refreshes the blockhash for a previously-built generic program call transaction.
func RebuildSOLProgramCallTx(rpcURL string, params SOLProgramCallParams) (*SOLProgramCallTxData, error) {
	instrData, err := hex.DecodeString(params.Data)
	if err != nil {
		return nil, fmt.Errorf("decode instruction data hex: %w", err)
	}
	return BuildSOLProgramCallTx(rpcURL, params.From, params.ProgramID, params.Accounts, instrData)
}

// buildSOLProgramCallMessage constructs a generic Solana legacy transaction message for a single program instruction.
//
// The fee payer (from) is always the first account (signer + writable). Accounts are deduplicated by pubkey
// (merging isSigner/isWritable flags via OR), then sorted into Solana header groups:
// signer+writable, signer+readonly, non-signer+writable, non-signer+readonly.
func buildSOLProgramCallMessage(fromAddr, programID string, accounts []SOLAccountMeta, instrData []byte, blockhash string) ([]byte, error) {
	// Validate from address.
	fromPub, err := base58Decode(fromAddr)
	if err != nil || len(fromPub) != 32 {
		return nil, fmt.Errorf("invalid from address %q: %v", fromAddr, err)
	}
	// Validate program ID.
	progPub, err := base58Decode(programID)
	if err != nil || len(progPub) != 32 {
		return nil, fmt.Errorf("invalid program ID %q: %v", programID, err)
	}
	// Validate blockhash.
	bhBytes, err := base58Decode(blockhash)
	if err != nil || len(bhBytes) != 32 {
		return nil, fmt.Errorf("invalid blockhash %q: %v", blockhash, err)
	}

	// Collect all accounts into a deduplicated map.
	// Key = pubkey string, value = merged flags.
	type acctInfo struct {
		pubkey     string
		isSigner   bool
		isWritable bool
	}
	seen := make(map[string]int) // pubkey -> index in accts
	var accts []acctInfo

	// Fee payer is always first: signer + writable.
	seen[fromAddr] = 0
	accts = append(accts, acctInfo{pubkey: fromAddr, isSigner: true, isWritable: true})

	// Add instruction accounts, merging flags if duplicate.
	for _, a := range accounts {
		if idx, ok := seen[a.Pubkey]; ok {
			accts[idx].isSigner = accts[idx].isSigner || a.IsSigner
			accts[idx].isWritable = accts[idx].isWritable || a.IsWritable
		} else {
			// Validate pubkey.
			pub, err := base58Decode(a.Pubkey)
			if err != nil || len(pub) != 32 {
				return nil, fmt.Errorf("invalid account pubkey %q: %v", a.Pubkey, err)
			}
			seen[a.Pubkey] = len(accts)
			accts = append(accts, acctInfo{pubkey: a.Pubkey, isSigner: a.IsSigner, isWritable: a.IsWritable})
		}
	}

	// Add program ID (non-signer, non-writable) if not already present.
	if idx, ok := seen[programID]; ok {
		// Program already in accounts; don't override signer/writable flags to false.
		_ = idx
	} else {
		seen[programID] = len(accts)
		accts = append(accts, acctInfo{pubkey: programID, isSigner: false, isWritable: false})
	}

	// Sort into header groups: signer+writable, signer+readonly, non-signer+writable, non-signer+readonly.
	// We use a stable partition approach so fee payer stays first within its group.
	type sortedAcct struct {
		acctInfo
		origIdx int
		group   int // 0=signer+writable, 1=signer+readonly, 2=nonsigner+writable, 3=nonsigner+readonly
	}
	sorted := make([]sortedAcct, len(accts))
	for i, a := range accts {
		g := 3
		if a.isSigner && a.isWritable {
			g = 0
		} else if a.isSigner {
			g = 1
		} else if a.isWritable {
			g = 2
		}
		sorted[i] = sortedAcct{acctInfo: a, origIdx: i, group: g}
	}
	// Stable sort by group, preserving insertion order within each group.
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].group < sorted[j-1].group; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	// Build index map: pubkey -> position in sorted order.
	indexMap := make(map[string]int, len(sorted))
	for i, s := range sorted {
		indexMap[s.pubkey] = i
	}

	// Count header values.
	numSigners := 0
	numReadonlySigned := 0
	numReadonlyUnsigned := 0
	for _, s := range sorted {
		if s.isSigner {
			numSigners++
			if !s.isWritable {
				numReadonlySigned++
			}
		} else {
			if !s.isWritable {
				numReadonlyUnsigned++
			}
		}
	}

	// Assemble message.
	var msg []byte
	// Header: [numRequiredSignatures, numReadonlySignedAccounts, numReadonlyUnsignedAccounts]
	msg = append(msg, byte(numSigners), byte(numReadonlySigned), byte(numReadonlyUnsigned))
	// Account keys.
	msg = append(msg, compactU16(len(sorted))...)
	for _, s := range sorted {
		pub, _ := base58Decode(s.pubkey) // already validated
		msg = append(msg, pub...)
	}
	// Recent blockhash.
	msg = append(msg, bhBytes...)
	// Instructions (1 instruction).
	msg = append(msg, compactU16(1)...)
	// Program ID index.
	msg = append(msg, byte(indexMap[programID]))
	// Instruction account indices (only the instruction accounts, not fee payer unless it was in accounts).
	// We need to list the accounts in the order they appear in the instruction's accounts list.
	// The instruction accounts are: all items from the `accounts` parameter (in order).
	// Deduplicated accounts that mapped to fee payer still get their index.
	instrAcctIndices := make([]byte, 0, len(accounts))
	for _, a := range accounts {
		instrAcctIndices = append(instrAcctIndices, byte(indexMap[a.Pubkey]))
	}
	msg = append(msg, compactU16(len(instrAcctIndices))...)
	msg = append(msg, instrAcctIndices...)
	// Instruction data.
	msg = append(msg, compactU16(len(instrData))...)
	msg = append(msg, instrData...)

	return msg, nil
}

// nativeMintID is the Wrapped SOL (wSOL) native mint address.
const nativeMintID = "So11111111111111111111111111111111111111112"

// SOLWrapParams contains all fields needed to reconstruct and broadcast a wrap/unwrap SOL transaction.
type SOLWrapParams struct {
	From      string `json:"from"`
	Lamports  uint64 `json:"lamports"`
	Blockhash string `json:"blockhash"`
	Wrap      bool   `json:"wrap"` // true=wrap, false=unwrap
}

// SOLWrapTxData is returned by BuildSOLWrapTx / BuildSOLUnwrapTx.
type SOLWrapTxData struct {
	Params       SOLWrapParams
	MessageBytes []byte
}

// BuildSOLWrapTx constructs a transaction to wrap SOL into wSOL.
// Instructions: CreateAssociatedTokenAccountIdempotent + SystemProgram.Transfer + Token.SyncNative
func BuildSOLWrapTx(rpcURL, ownerAddr string, amountSOL float64) (*SOLWrapTxData, error) {
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}
	lamportsBig := new(big.Float).SetFloat64(amountSOL)
	lamportsBig.Mul(lamportsBig, new(big.Float).SetFloat64(1e9))
	lamportsInt, _ := lamportsBig.Uint64()
	lamports := lamportsInt
	if lamports == 0 {
		return nil, fmt.Errorf("amount too small (rounds to 0 lamports)")
	}

	result, err := jsonRPCWithRetry(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"method": "getLatestBlockhash",
		"params": []interface{}{map[string]interface{}{"commitment": "finalized"}},
	})
	if err != nil {
		return nil, fmt.Errorf("get blockhash: %w", err)
	}
	resMap, _ := result["result"].(map[string]interface{})
	valMap, _ := resMap["value"].(map[string]interface{})
	blockhash, _ := valMap["blockhash"].(string)
	if blockhash == "" {
		return nil, fmt.Errorf("empty blockhash from RPC")
	}

	msgBytes, err := buildSOLWrapMessage(ownerAddr, lamports, blockhash)
	if err != nil {
		return nil, err
	}
	return &SOLWrapTxData{
		Params:       SOLWrapParams{From: ownerAddr, Lamports: lamports, Blockhash: blockhash, Wrap: true},
		MessageBytes: msgBytes,
	}, nil
}

// buildSOLWrapMessage constructs the message for wrapping SOL → wSOL.
//
// Accounts (7):
//   0: owner           (signer + writable)
//   1: wSOL ATA        (writable)
//   2: native mint     (readonly)
//   3: token program   (readonly)
//   4: system program  (readonly)
//   5: ATA program     (readonly)
//
// Instructions (3):
//   1. ATA program: CreateAssociatedTokenAccountIdempotent [0, 1, 0, 2, 4, 3]  data=[1]
//   2. System program: Transfer [0, 1]  data=[2, lamports_u64_LE]
//   3. Token program: SyncNative [1]    data=[0x11]
func buildSOLWrapMessage(ownerAddr string, lamports uint64, blockhash string) ([]byte, error) {
	wsolATA, err := DeriveATA(ownerAddr, nativeMintID)
	if err != nil {
		return nil, fmt.Errorf("derive wSOL ATA: %w", err)
	}

	ownerPub, err := base58Decode(ownerAddr)
	if err != nil || len(ownerPub) != 32 {
		return nil, fmt.Errorf("invalid owner address: %v", err)
	}
	nativeMintPub, err := base58Decode(nativeMintID)
	if err != nil || len(nativeMintPub) != 32 {
		return nil, fmt.Errorf("invalid native mint: %v", err)
	}
	tokenProgramPub, err := base58Decode(splTokenProgramID)
	if err != nil || len(tokenProgramPub) != 32 {
		return nil, fmt.Errorf("invalid token program: %v", err)
	}
	ataProgramPub, err := base58Decode(ataProgramID)
	if err != nil || len(ataProgramPub) != 32 {
		return nil, fmt.Errorf("invalid ATA program: %v", err)
	}
	bhBytes, err := base58Decode(blockhash)
	if err != nil || len(bhBytes) != 32 {
		return nil, fmt.Errorf("invalid blockhash: %v", err)
	}

	var msg []byte
	// Header: 1 signer, 0 readonly signed, 4 readonly unsigned
	msg = append(msg, 1, 0, 4)
	// Account keys (6)
	msg = append(msg, compactU16(6)...)
	msg = append(msg, ownerPub...)          // 0: owner (signer + writable)
	msg = append(msg, wsolATA...)           // 1: wSOL ATA (writable)
	msg = append(msg, nativeMintPub...)     // 2: native mint (readonly)
	msg = append(msg, tokenProgramPub...)   // 3: token program (readonly)
	msg = append(msg, systemProgramID[:]...) // 4: system program (readonly)
	msg = append(msg, ataProgramPub...)     // 5: ATA program (readonly)
	// Blockhash
	msg = append(msg, bhBytes...)

	// 3 instructions
	msg = append(msg, compactU16(3)...)

	// Instruction 1: CreateAssociatedTokenAccountIdempotent
	msg = append(msg, 5)                    // program = ATA program (index 5)
	msg = append(msg, compactU16(6)...)     // 6 accounts
	msg = append(msg, 0, 1, 0, 2, 4, 3)    // payer, ata, wallet, mint, system, token
	msg = append(msg, compactU16(1)...)     // 1 byte data
	msg = append(msg, 1)                    // idempotent create

	// Instruction 2: System.Transfer (SOL to wSOL ATA)
	transferData := make([]byte, 12)
	binary.LittleEndian.PutUint32(transferData[0:4], 2) // instruction index 2 = Transfer
	binary.LittleEndian.PutUint64(transferData[4:12], lamports)
	msg = append(msg, 4)                    // program = system program (index 4)
	msg = append(msg, compactU16(2)...)     // 2 accounts
	msg = append(msg, 0, 1)                // from (owner), to (wSOL ATA)
	msg = append(msg, compactU16(len(transferData))...)
	msg = append(msg, transferData...)

	// Instruction 3: Token.SyncNative
	msg = append(msg, 3)                    // program = token program (index 3)
	msg = append(msg, compactU16(1)...)     // 1 account
	msg = append(msg, 1)                    // wSOL ATA
	msg = append(msg, compactU16(1)...)     // 1 byte data
	msg = append(msg, 0x11)                 // SyncNative instruction

	return msg, nil
}

// BuildSOLUnwrapTx constructs a transaction to unwrap wSOL back to SOL by closing the wSOL ATA.
func BuildSOLUnwrapTx(rpcURL, ownerAddr string) (*SOLWrapTxData, error) {
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}

	result, err := jsonRPCWithRetry(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"method": "getLatestBlockhash",
		"params": []interface{}{map[string]interface{}{"commitment": "finalized"}},
	})
	if err != nil {
		return nil, fmt.Errorf("get blockhash: %w", err)
	}
	resMap, _ := result["result"].(map[string]interface{})
	valMap, _ := resMap["value"].(map[string]interface{})
	blockhash, _ := valMap["blockhash"].(string)
	if blockhash == "" {
		return nil, fmt.Errorf("empty blockhash from RPC")
	}

	msgBytes, err := buildSOLUnwrapMessage(ownerAddr, blockhash)
	if err != nil {
		return nil, err
	}
	return &SOLWrapTxData{
		Params:       SOLWrapParams{From: ownerAddr, Blockhash: blockhash, Wrap: false},
		MessageBytes: msgBytes,
	}, nil
}

// buildSOLUnwrapMessage constructs the message for unwrapping wSOL → SOL.
// Instruction: Token.CloseAccount [wsolATA, owner, owner]  data=[0x09]
func buildSOLUnwrapMessage(ownerAddr string, blockhash string) ([]byte, error) {
	wsolATA, err := DeriveATA(ownerAddr, nativeMintID)
	if err != nil {
		return nil, fmt.Errorf("derive wSOL ATA: %w", err)
	}

	ownerPub, err := base58Decode(ownerAddr)
	if err != nil || len(ownerPub) != 32 {
		return nil, fmt.Errorf("invalid owner address: %v", err)
	}
	tokenProgramPub, err := base58Decode(splTokenProgramID)
	if err != nil || len(tokenProgramPub) != 32 {
		return nil, fmt.Errorf("invalid token program: %v", err)
	}
	bhBytes, err := base58Decode(blockhash)
	if err != nil || len(bhBytes) != 32 {
		return nil, fmt.Errorf("invalid blockhash: %v", err)
	}

	var msg []byte
	// Header: 1 signer, 0 readonly signed, 1 readonly unsigned (token program)
	msg = append(msg, 1, 0, 1)
	// Account keys (3)
	msg = append(msg, compactU16(3)...)
	msg = append(msg, ownerPub...)          // 0: owner (signer + writable)
	msg = append(msg, wsolATA...)           // 1: wSOL ATA (writable)
	msg = append(msg, tokenProgramPub...)   // 2: token program (readonly)
	// Blockhash
	msg = append(msg, bhBytes...)

	// 1 instruction: CloseAccount
	msg = append(msg, compactU16(1)...)
	msg = append(msg, 2)                    // program = token program (index 2)
	msg = append(msg, compactU16(3)...)     // 3 accounts
	msg = append(msg, 1, 0, 0)             // account (wSOL ATA), destination (owner), authority (owner)
	msg = append(msg, compactU16(1)...)     // 1 byte data
	msg = append(msg, 0x09)                 // CloseAccount instruction

	return msg, nil
}

// RebuildSOLWrapTx refreshes the blockhash for a wrap/unwrap transaction.
func RebuildSOLWrapTx(rpcURL string, params SOLWrapParams) (*SOLWrapTxData, error) {
	if params.Wrap {
		return BuildSOLWrapTx(rpcURL, params.From, float64(params.Lamports)/1e9)
	}
	return BuildSOLUnwrapTx(rpcURL, params.From)
}

// AssembleAndBroadcastSOLWrap assembles and broadcasts a signed wrap/unwrap transaction.
func AssembleAndBroadcastSOLWrap(rpcURL string, params SOLWrapParams, sig []byte) (string, error) {
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}
	if len(sig) != 64 {
		return "", fmt.Errorf("invalid Ed25519 signature length: %d (expected 64)", len(sig))
	}

	var msgBytes []byte
	var err error
	if params.Wrap {
		msgBytes, err = buildSOLWrapMessage(params.From, params.Lamports, params.Blockhash)
	} else {
		msgBytes, err = buildSOLUnwrapMessage(params.From, params.Blockhash)
	}
	if err != nil {
		return "", err
	}

	txBytes := make([]byte, 0, 1+64+len(msgBytes))
	txBytes = append(txBytes, 0x01)
	txBytes = append(txBytes, sig[:64]...)
	txBytes = append(txBytes, msgBytes...)

	txB64 := base64.StdEncoding.EncodeToString(txBytes)
	result, err := jsonRPC(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"method": "sendTransaction",
		"params": []interface{}{txB64, map[string]interface{}{"encoding": "base64"}},
	})
	if err != nil {
		return "", fmt.Errorf("broadcast: %w", err)
	}
	txSig, _ := result["result"].(string)
	return txSig, nil
}

// compactU16 encodes an integer as Solana compact-u16.
// Values 0–127 use 1 byte; 128–16383 use 2 bytes.
func compactU16(v int) []byte {
	if v <= 0x7f {
		return []byte{byte(v)}
	}
	if v <= 0x3fff {
		return []byte{byte(v&0x7f | 0x80), byte(v >> 7)}
	}
	return []byte{byte(v&0x7f | 0x80), byte(v>>7&0x7f | 0x80), byte(v >> 14)}
}
