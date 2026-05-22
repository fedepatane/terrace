package eth_test

import (
	"math/big"
	"testing"

	"github.com/fpatane/arbitrage-bot/internal/eth"
)

func TestEthToWei_OneETH(t *testing.T) {
	result := eth.EthToWei(1.0)
	expected := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	if result.Cmp(expected) != 0 {
		t.Errorf("1 ETH: expected %s wei, got %s", expected.String(), result.String())
	}
}

func TestEthToWei_HalfETH(t *testing.T) {
	result := eth.EthToWei(0.5)

	// 0.5 ETH = 5 * 10^17 wei
	expected := new(big.Int).Mul(big.NewInt(5), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil))

	if result.Cmp(expected) != 0 {
		t.Errorf("0.5 ETH: expected %s wei, got %s", expected.String(), result.String())
	}
}

func TestEthToWei_TenETH(t *testing.T) {
	result := eth.EthToWei(10.0)

	// 10 ETH = 10 * 10^18 wei
	expected := new(big.Int).Mul(big.NewInt(10), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))

	if result.Cmp(expected) != 0 {
		t.Errorf("10 ETH: expected %s wei, got %s", expected.String(), result.String())
	}
}

func TestRawToFloat_USDC(t *testing.T) {
	// 2250 USDC = 2250000000 raw (6 decimals)
	raw := big.NewInt(2_250_000_000)
	result := eth.RawToFloat(raw, 6)

	if result != 2250.0 {
		t.Errorf("expected 2250.0 USDC, got %.6f", result)
	}
}

func TestRawToFloat_OneUSDC(t *testing.T) {
	raw := big.NewInt(1_000_000)
	result := eth.RawToFloat(raw, 6)

	if result != 1.0 {
		t.Errorf("expected 1.0 USDC, got %.6f", result)
	}
}

func TestHexToUint64_Simple(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"0x1", 1},
		{"0xFF", 255},
		{"0x10", 16},
		{"0x17FFFF7", 25165815},
		{"0x0", 0},
	}

	for _, tc := range tests {
		result, err := eth.HexToUint64(tc.input)
		if err != nil {
			t.Errorf("HexToUint64(%s): unexpected error: %v", tc.input, err)
			continue
		}
		if result != tc.expected {
			t.Errorf("HexToUint64(%s): expected %d, got %d", tc.input, tc.expected, result)
		}
	}
}

func TestHexToUint64_InvalidInput(t *testing.T) {
	invalidInputs := []string{
		"",
		"0x",
		"FF",     // missing 0x prefix
		"0xGG",   // invalid hex chars
	}

	for _, input := range invalidInputs {
		_, err := eth.HexToUint64(input)
		if err == nil {
			t.Errorf("HexToUint64(%q): expected error, got nil", input)
		}
	}
}
