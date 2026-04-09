// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package chain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	uintRe      = regexp.MustCompile(`^uint(\d+)$`)
	intRe       = regexp.MustCompile(`^int(\d+)$`)
	bytesNRe    = regexp.MustCompile(`^bytes(\d+)$`)
	fixedArrRe  = regexp.MustCompile(`^(.+)\[(\d+)\]$`)
)

// EncodeCall encodes a Solidity function call into raw EVM calldata.
// funcSig is the canonical signature, e.g. "transfer(address,uint256)".
// args are the values for each parameter.
//
// Supported types:
//   - address
//   - bool
//   - uintN (uint8, uint16, uint24, uint32, ..., uint256)
//   - intN  (int8, int16, ..., int256)
//   - bytesN (bytes1, bytes2, ..., bytes32) — fixed-size
//   - bytes — dynamic byte array
//   - string — dynamic string
//   - T[] — dynamic array of any supported static type T
//   - tuple — structs encoded as JSON arrays (positional)
func EncodeCall(funcSig string, args []interface{}) ([]byte, error) {
	paramTypes, err := parseParamTypes(funcSig)
	if err != nil {
		return nil, err
	}
	if len(paramTypes) != len(args) {
		return nil, fmt.Errorf("arg count mismatch: signature has %d params but %d args provided", len(paramTypes), len(args))
	}

	selector := crypto.Keccak256([]byte(funcSig))[:4]

	encoded, err := encodeArgs(paramTypes, args)
	if err != nil {
		return nil, err
	}

	calldata := make([]byte, 0, 4+len(encoded))
	calldata = append(calldata, selector...)
	calldata = append(calldata, encoded...)
	return calldata, nil
}

// parseParamTypes extracts parameter type strings from a function signature.
// Handles nested tuples: "exactInputSingle((address,address,uint24,address,uint256,uint256,uint160))"
func parseParamTypes(funcSig string) ([]string, error) {
	openParen := strings.Index(funcSig, "(")
	closeParen := strings.LastIndex(funcSig, ")")
	if openParen < 0 || closeParen < 0 || closeParen <= openParen {
		return nil, fmt.Errorf("invalid function signature: %s", funcSig)
	}
	if closeParen != len(funcSig)-1 {
		return nil, fmt.Errorf("invalid function signature: unexpected characters after closing paren: %s", funcSig)
	}
	inner := funcSig[openParen+1 : closeParen]
	if inner == "" {
		return nil, nil
	}
	return splitTopLevel(inner), nil
}

// splitTopLevel splits comma-separated types at the top level, respecting nested parens.
// "address,(uint256,bool),uint24" → ["address", "(uint256,bool)", "uint24"]
func splitTopLevel(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, c := range s {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	return parts
}

// isDynamic returns true if the type requires offset-based (dynamic) encoding.
func isDynamic(typ string) bool {
	if typ == "bytes" || typ == "string" {
		return true
	}
	if strings.HasSuffix(typ, "[]") {
		return true
	}
	// Fixed-size array T[N]: dynamic if element type is dynamic.
	if m := parseFixedArray(typ); m != nil {
		return isDynamic(m.elemType)
	}
	if strings.HasPrefix(typ, "(") {
		// Tuple is dynamic if any element is dynamic.
		inner := typ[1 : len(typ)-1]
		for _, t := range splitTopLevel(inner) {
			if isDynamic(t) {
				return true
			}
		}
	}
	return false
}

// fixedArrayMatch holds the parsed parts of a T[N] type.
type fixedArrayMatch struct {
	elemType string
	size     int
}

// parseFixedArray parses a fixed-size array type like "uint256[3]" or "(address,bool)[2]".
// Returns nil if typ is not a fixed-size array.
func parseFixedArray(typ string) *fixedArrayMatch {
	// Handle tuple arrays like (address,uint256)[3] — regex won't work due to parens.
	if strings.HasPrefix(typ, "(") {
		// Find matching close paren, then check for [N].
		depth := 0
		closeIdx := -1
		for i, c := range typ {
			switch c {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					closeIdx = i
					break
				}
			}
			if closeIdx >= 0 {
				break
			}
		}
		if closeIdx >= 0 && closeIdx < len(typ)-1 {
			suffix := typ[closeIdx+1:]
			if len(suffix) >= 3 && suffix[0] == '[' && suffix[len(suffix)-1] == ']' {
				n, err := strconv.Atoi(suffix[1 : len(suffix)-1])
				if err == nil && n > 0 {
					return &fixedArrayMatch{elemType: typ[:closeIdx+1], size: n}
				}
			}
		}
		return nil
	}
	m := fixedArrRe.FindStringSubmatch(typ)
	if m == nil {
		return nil
	}
	n, err := strconv.Atoi(m[2])
	if err != nil || n == 0 {
		return nil
	}
	return &fixedArrayMatch{elemType: m[1], size: n}
}

// encodeArgs encodes multiple arguments with proper head/tail layout for dynamic types.
func encodeArgs(types []string, args []interface{}) ([]byte, error) {
	heads := make([][]byte, len(types))
	tails := make([][]byte, len(types))
	hasDynamic := false

	for i, typ := range types {
		if isDynamic(typ) {
			hasDynamic = true
			encoded, err := encodeDynamic(typ, args[i])
			if err != nil {
				return nil, fmt.Errorf("arg %d (%s): %w", i, typ, err)
			}
			tails[i] = encoded
			heads[i] = nil // placeholder, will fill with offset
		} else {
			word, err := encodeStatic(typ, args[i])
			if err != nil {
				return nil, fmt.Errorf("arg %d (%s): %w", i, typ, err)
			}
			heads[i] = word
		}
	}

	if !hasDynamic {
		// All static — simple concatenation.
		result := make([]byte, 0, 32*len(types))
		for _, h := range heads {
			result = append(result, h...)
		}
		return result, nil
	}

	// Calculate offsets for dynamic types.
	// headSize must account for the actual head size of each argument:
	// dynamic types contribute a 32-byte offset pointer; static tuples may
	// be larger than 32 bytes and are inlined (not pointer-referenced).
	headSize := 0
	for i := range types {
		if tails[i] != nil {
			headSize += 32 // offset pointer for dynamic types
		} else {
			headSize += len(heads[i]) // actual encoded size (static tuples may be >32 bytes)
		}
	}
	tailOffset := headSize
	for i := range types {
		if tails[i] != nil {
			// Store offset in head.
			offset := big.NewInt(int64(tailOffset))
			word := make([]byte, 32)
			b := offset.Bytes()
			copy(word[32-len(b):], b)
			heads[i] = word
			tailOffset += len(tails[i])
		}
	}

	result := make([]byte, 0, tailOffset)
	for _, h := range heads {
		result = append(result, h...)
	}
	for _, t := range tails {
		if t != nil {
			result = append(result, t...)
		}
	}
	return result, nil
}

// encodeStatic encodes a static-type argument as a 32-byte ABI word.
func encodeStatic(typ string, arg interface{}) ([]byte, error) {
	switch {
	case typ == "address":
		s, ok := arg.(string)
		if !ok {
			return nil, fmt.Errorf("address requires string, got %T", arg)
		}
		addr := common.HexToAddress(s)
		word := make([]byte, 32)
		copy(word[12:], addr.Bytes())
		return word, nil

	case typ == "bool":
		b, ok := arg.(bool)
		if !ok {
			return nil, fmt.Errorf("bool requires bool, got %T", arg)
		}
		word := make([]byte, 32)
		if b {
			word[31] = 1
		}
		return word, nil

	case typ == "uint256":
		return encodeUint(256, arg)

	case typ == "int256":
		v, err := toBigInt(arg)
		if err != nil {
			return nil, err
		}
		return encodeInt256(v)

	case uintRe.MatchString(typ):
		bits, _ := strconv.Atoi(uintRe.FindStringSubmatch(typ)[1])
		if bits == 0 || bits > 256 || bits%8 != 0 {
			return nil, fmt.Errorf("invalid uint size: %d", bits)
		}
		return encodeUint(bits, arg)

	case intRe.MatchString(typ):
		bits, _ := strconv.Atoi(intRe.FindStringSubmatch(typ)[1])
		if bits == 0 || bits > 256 || bits%8 != 0 {
			return nil, fmt.Errorf("invalid int size: %d", bits)
		}
		return encodeIntN(bits, arg)

	case bytesNRe.MatchString(typ):
		n, _ := strconv.Atoi(bytesNRe.FindStringSubmatch(typ)[1])
		if n == 0 || n > 32 {
			return nil, fmt.Errorf("invalid bytesN size: %d", n)
		}
		return encodeBytesN(n, arg)

	default:
		// Check for fixed-size array T[N] before tuple — tuple arrays like (T1,T2)[N] start with "(".
		if fa := parseFixedArray(typ); fa != nil && !isDynamic(fa.elemType) {
			return encodeFixedArray(fa, arg)
		}
		// Static tuple.
		if strings.HasPrefix(typ, "(") {
			return encodeTuple(typ, arg)
		}
		return nil, fmt.Errorf("unsupported static type: %s", typ)
	}
}

// encodeDynamic encodes a dynamic-type argument (bytes, string, T[], dynamic tuple).
func encodeDynamic(typ string, arg interface{}) ([]byte, error) {
	switch {
	case typ == "bytes":
		return encodeDynamicBytes(arg)

	case typ == "string":
		s, ok := arg.(string)
		if !ok {
			return nil, fmt.Errorf("string requires string, got %T", arg)
		}
		return encodeDynamicBytes(s)

	case strings.HasSuffix(typ, "[]"):
		elemType := typ[:len(typ)-2]
		return encodeDynamicArray(elemType, arg)

	default:
		// Check for fixed-size array T[N] with dynamic element type.
		if fa := parseFixedArray(typ); fa != nil {
			return encodeFixedArrayDynamic(fa, arg)
		}
		// Dynamic tuple.
		if strings.HasPrefix(typ, "(") {
			return encodeTupleDynamic(typ, arg)
		}
		return nil, fmt.Errorf("unsupported dynamic type: %s", typ)
	}
}

// encodeUint encodes a uint of the given bit size as a 32-byte word.
func encodeUint(bits int, arg interface{}) ([]byte, error) {
	v, err := toBigInt(arg)
	if err != nil {
		return nil, err
	}
	if v.Sign() < 0 {
		return nil, fmt.Errorf("uint%d cannot be negative", bits)
	}
	maxVal := new(big.Int).Lsh(big.NewInt(1), uint(bits))
	if v.Cmp(maxVal) >= 0 {
		return nil, fmt.Errorf("uint%d overflow", bits)
	}
	word := make([]byte, 32)
	b := v.Bytes()
	if len(b) > 32 {
		return nil, fmt.Errorf("uint%d overflow", bits)
	}
	copy(word[32-len(b):], b)
	return word, nil
}

// encodeIntN encodes a signed int of the given bit size.
func encodeIntN(bits int, arg interface{}) ([]byte, error) {
	v, err := toBigInt(arg)
	if err != nil {
		return nil, err
	}
	limit := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
	if v.Cmp(limit) >= 0 {
		return nil, fmt.Errorf("int%d overflow", bits)
	}
	negLimit := new(big.Int).Neg(limit)
	if v.Cmp(negLimit) < 0 {
		return nil, fmt.Errorf("int%d underflow", bits)
	}
	return encodeInt256(v)
}

// encodeBytesN encodes a fixed-size bytesN value.
func encodeBytesN(n int, arg interface{}) ([]byte, error) {
	s, ok := arg.(string)
	if !ok {
		return nil, fmt.Errorf("bytes%d requires hex string, got %T", n, arg)
	}
	s = strings.TrimPrefix(s, "0x")
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("bytes%d hex decode: %w", n, err)
	}
	if len(decoded) > n {
		return nil, fmt.Errorf("bytes%d: got %d bytes, max %d", n, len(decoded), n)
	}
	word := make([]byte, 32)
	copy(word, decoded) // left-aligned
	return word, nil
}

// encodeDynamicBytes encodes dynamic bytes or string.
// Format: length (32 bytes) + data (padded to 32-byte boundary)
func encodeDynamicBytes(arg interface{}) ([]byte, error) {
	var data []byte
	switch v := arg.(type) {
	case string:
		if strings.HasPrefix(v, "0x") {
			var err error
			data, err = hex.DecodeString(v[2:])
			if err != nil {
				return nil, fmt.Errorf("hex decode: %w", err)
			}
		} else {
			data = []byte(v)
		}
	case []byte:
		data = v
	default:
		return nil, fmt.Errorf("bytes/string requires string or []byte, got %T", arg)
	}

	// Length prefix.
	lenWord := make([]byte, 32)
	lenBytes := big.NewInt(int64(len(data))).Bytes()
	copy(lenWord[32-len(lenBytes):], lenBytes)

	// Pad data to 32-byte boundary.
	padded := data
	if rem := len(data) % 32; rem != 0 {
		padded = append(padded, make([]byte, 32-rem)...)
	}

	result := make([]byte, 0, 32+len(padded))
	result = append(result, lenWord...)
	result = append(result, padded...)
	return result, nil
}

// encodeDynamicArray encodes a dynamic array T[].
func encodeDynamicArray(elemType string, arg interface{}) ([]byte, error) {
	items, ok := arg.([]interface{})
	if !ok {
		return nil, fmt.Errorf("array requires []interface{}, got %T", arg)
	}

	// Length prefix.
	lenWord := make([]byte, 32)
	lenBytes := big.NewInt(int64(len(items))).Bytes()
	copy(lenWord[32-len(lenBytes):], lenBytes)

	types := make([]string, len(items))
	for i := range items {
		types[i] = elemType
	}

	encoded, err := encodeArgs(types, items)
	if err != nil {
		return nil, err
	}

	result := make([]byte, 0, 32+len(encoded))
	result = append(result, lenWord...)
	result = append(result, encoded...)
	return result, nil
}

// encodeTuple encodes a static tuple (all elements static).
func encodeTuple(typ string, arg interface{}) ([]byte, error) {
	inner := typ[1 : len(typ)-1]
	elemTypes := splitTopLevel(inner)

	items, err := toSlice(arg)
	if err != nil {
		return nil, fmt.Errorf("tuple: %w", err)
	}
	if len(items) != len(elemTypes) {
		return nil, fmt.Errorf("tuple element count mismatch: expected %d, got %d", len(elemTypes), len(items))
	}

	return encodeArgs(elemTypes, items)
}

// encodeTupleDynamic encodes a tuple that may contain dynamic elements.
func encodeTupleDynamic(typ string, arg interface{}) ([]byte, error) {
	return encodeTuple(typ, arg) // encodeArgs handles dynamic elements internally
}

// encodeFixedArray encodes a fixed-size array T[N] where T is a static type.
// T[N] is encoded as N consecutive 32-byte words (no length prefix).
func encodeFixedArray(fa *fixedArrayMatch, arg interface{}) ([]byte, error) {
	items, err := toSlice(arg)
	if err != nil {
		return nil, fmt.Errorf("%s[%d]: %w", fa.elemType, fa.size, err)
	}
	if len(items) != fa.size {
		return nil, fmt.Errorf("%s[%d]: expected %d elements, got %d", fa.elemType, fa.size, fa.size, len(items))
	}
	types := make([]string, fa.size)
	for i := range types {
		types[i] = fa.elemType
	}
	return encodeArgs(types, items)
}

// encodeFixedArrayDynamic encodes a fixed-size array T[N] where T is a dynamic type.
func encodeFixedArrayDynamic(fa *fixedArrayMatch, arg interface{}) ([]byte, error) {
	return encodeFixedArray(fa, arg)
}

// toSlice converts an interface to []interface{} (handles JSON arrays).
func toSlice(v interface{}) ([]interface{}, error) {
	switch val := v.(type) {
	case []interface{}:
		return val, nil
	default:
		return nil, fmt.Errorf("expected array, got %T", v)
	}
}

// toBigInt converts various numeric types to *big.Int.
func toBigInt(v interface{}) (*big.Int, error) {
	switch val := v.(type) {
	case *big.Int:
		return new(big.Int).Set(val), nil
	case string:
		n := new(big.Int)
		s := val
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			if _, ok := n.SetString(s[2:], 16); !ok {
				return nil, fmt.Errorf("cannot parse %q as hex integer", val)
			}
			return n, nil
		}
		if _, ok := n.SetString(s, 10); !ok {
			return nil, fmt.Errorf("cannot parse %q as decimal integer", val)
		}
		return n, nil
	case float64:
		bf := new(big.Float).SetFloat64(val)
		bi, accuracy := bf.Int(nil)
		if accuracy != big.Exact {
			return nil, fmt.Errorf("float64 value %v is not an exact integer", val)
		}
		return bi, nil
	case int:
		return big.NewInt(int64(val)), nil
	case int64:
		return big.NewInt(val), nil
	case json.Number:
		n := new(big.Int)
		if _, ok := n.SetString(val.String(), 10); !ok {
			return nil, fmt.Errorf("cannot parse json.Number %q as integer", val.String())
		}
		return n, nil
	default:
		return nil, fmt.Errorf("unsupported numeric type %T", v)
	}
}

// encodeInt256 encodes a signed integer as a 32-byte two's complement word.
func encodeInt256(v *big.Int) ([]byte, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 255)
	if v.Cmp(limit) >= 0 {
		return nil, fmt.Errorf("int256 overflow")
	}
	negLimit := new(big.Int).Neg(limit)
	if v.Cmp(negLimit) < 0 {
		return nil, fmt.Errorf("int256 underflow")
	}

	if v.Sign() >= 0 {
		word := make([]byte, 32)
		b := v.Bytes()
		copy(word[32-len(b):], b)
		return word, nil
	}

	modulus := new(big.Int).Lsh(big.NewInt(1), 256)
	tc := new(big.Int).Add(modulus, v)
	word := make([]byte, 32)
	b := tc.Bytes()
	copy(word[32-len(b):], b)
	return word, nil
}
