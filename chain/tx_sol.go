package chain

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

// systemProgramID is the Solana system program address (all zeros).
var systemProgramID = [32]byte{}

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
	lamports := uint64(amountSOL * 1e9)
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
