package cex

import (
	"testing"
)

func TestWalkLevels_FillsAcrossMultipleLevels(t *testing.T) {
	levels := [][]string{
		{"2100.00", "1.5"}, // fill 1.5 ETH at $2100
		{"2100.50", "2.0"}, // fill 2.0 ETH at $2100.50
		{"2101.00", "5.0"}, // fill remaining 1.5 ETH at $2101
	}

	price, err := walkLevels(levels, 5.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// expected: (1.5*2100 + 2.0*2100.50 + 1.5*2101) / 5.0
	expected := (1.5*2100 + 2.0*2100.50 + 1.5*2101) / 5.0
	if abs(price-expected) > 0.01 {
		t.Errorf("expected price %.4f, got %.4f", expected, price)
	}
}

func TestWalkLevels_ExactFillSingleLevel(t *testing.T) {
	levels := [][]string{
		{"2100.00", "10.0"},
	}

	price, err := walkLevels(levels, 10.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if abs(price-2100.0) > 0.001 {
		t.Errorf("expected 2100.00, got %.4f", price)
	}
}

func TestWalkLevels_InsufficientLiquidity(t *testing.T) {
	levels := [][]string{
		{"2100.00", "1.0"}, // only 1 ETH available
	}

	_, err := walkLevels(levels, 10.0) // want 10 ETH
	if err == nil {
		t.Fatal("expected error for insufficient liquidity, got nil")
	}
}

func TestWalkLevels_SkipsMalformedLevels(t *testing.T) {
	levels := [][]string{
		{"not-a-price", "1.0"}, // malformed — should be skipped
		{"2100.00", "5.0"},     // valid
	}

	price, err := walkLevels(levels, 5.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if abs(price-2100.0) > 0.001 {
		t.Errorf("expected 2100.00, got %.4f", price)
	}
}

func TestWalkLevels_PartialFillLastLevel(t *testing.T) {
	levels := [][]string{
		{"2100.00", "3.0"},
		{"2101.00", "10.0"}, // only need 2 ETH from this level
	}

	price, err := walkLevels(levels, 5.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// expected: (3.0*2100 + 2.0*2101) / 5.0
	expected := (3.0*2100 + 2.0*2101) / 5.0
	if abs(price-expected) > 0.01 {
		t.Errorf("expected %.4f, got %.4f", expected, price)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
