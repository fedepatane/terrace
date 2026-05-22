package eth

import (
	"fmt"
	"math/big"
)

// EthToWei converts a float ETH amount to wei as *big.Int.
// We need big.Int because 1 ETH = 10^18 wei — float64 lacks the precision.
func EthToWei(ethAmount float64) *big.Int {
	tenToThe18 := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	result, _ := new(big.Float).Mul(
		new(big.Float).SetFloat64(ethAmount),
		new(big.Float).SetInt(tenToThe18),
	).Int(nil)
	return result
}

// RawToFloat converts a raw ERC-20 token amount to float64 given its decimals.
// Example: RawToFloat(2250000000, 6) = 2250.0 USDC
func RawToFloat(rawAmount *big.Int, decimals int64) float64 {
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(decimals), nil)
	result, _ := new(big.Float).Quo(
		new(big.Float).SetInt(rawAmount),
		new(big.Float).SetInt(divisor),
	).Float64()
	return result
}

// HexToUint64 converts a hex string like "0x17FFFF7" to uint64.
func HexToUint64(hexStr string) (uint64, error) {
	if len(hexStr) < 3 || hexStr[:2] != "0x" {
		return 0, fmt.Errorf("invalid hex: %s", hexStr)
	}
	var result uint64
	for _, character := range hexStr[2:] {
		result <<= 4
		switch {
		case character >= '0' && character <= '9':
			result |= uint64(character - '0')
		case character >= 'a' && character <= 'f':
			result |= uint64(character-'a') + 10
		case character >= 'A' && character <= 'F':
			result |= uint64(character-'A') + 10
		default:
			return 0, fmt.Errorf("invalid hex character: %c", character)
		}
	}
	return result, nil
}
