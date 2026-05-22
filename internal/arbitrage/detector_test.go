package arbitrage_test

import (
	"testing"

	"github.com/fpatane/arbitrage-bot/internal/arbitrage"
)

func newTestDetector() *arbitrage.Detector {
	return arbitrage.NewDetector(
		0.001,   // 0.1% CEX fee (Binance taker)
		200_000, // gas limit for a Uniswap V3 swap
		0.3,     // 0.3% minimum profit threshold
	)
}

func TestDetect_CEXtoDEX_Profitable(t *testing.T) {
	detector := newTestDetector()

	// CEX ask $2100, DEX sells at $2150 — clear opportunity
	opp := detector.Detect(
		100,    // blockNumber
		10.0,   // tradeSize ETH
		2100.0, // cexAskPrice
		2099.0, // cexBidPrice
		2150.0, // dexSellPrice — Uniswap pays more than Binance asks
		2160.0, // dexBuyPrice
		20.0,   // gasPriceGwei
		2100.0, // ethPriceUSD (for gas cost conversion)
	)

	if opp == nil {
		t.Fatal("expected opportunity, got nil")
	}
	if opp.Direction != arbitrage.CEXtoDEX {
		t.Errorf("expected CEXtoDEX, got %v", opp.Direction)
	}
	if opp.ProfitUSD <= 0 {
		t.Errorf("expected positive profit, got %.2f", opp.ProfitUSD)
	}
	if opp.TradeSizeETH != 10.0 {
		t.Errorf("expected trade size 10.0, got %.1f", opp.TradeSizeETH)
	}
	if opp.BlockNumber != 100 {
		t.Errorf("expected block 100, got %d", opp.BlockNumber)
	}
}

func TestDetect_DEXtoCEX_Profitable(t *testing.T) {
	detector := newTestDetector()

	// DEX sells ETH cheap at $2050, CEX bids $2100 — buy on DEX, sell on CEX
	opp := detector.Detect(
		200,
		10.0,
		2101.0, // cexAskPrice
		2100.0, // cexBidPrice — Binance pays more than DEX charges
		2090.0, // dexSellPrice
		2050.0, // dexBuyPrice — Uniswap charges less than Binance pays
		20.0,
		2100.0,
	)

	if opp == nil {
		t.Fatal("expected opportunity, got nil")
	}
	if opp.Direction != arbitrage.DEXtoCEX {
		t.Errorf("expected DEXtoCEX, got %v", opp.Direction)
	}
	if opp.ProfitUSD <= 0 {
		t.Errorf("expected positive profit, got %.2f", opp.ProfitUSD)
	}
}

func TestDetect_BelowThreshold_ReturnsNil(t *testing.T) {
	detector := newTestDetector()

	// Prices almost equal — profit exists but below 0.3% threshold after fees and gas
	opp := detector.Detect(
		300,
		10.0,
		2100.00, // cexAskPrice
		2099.90, // cexBidPrice
		2100.50, // dexSellPrice — barely higher, not enough to cover fees+gas
		2101.00, // dexBuyPrice
		20.0,
		2100.0,
	)

	if opp != nil {
		t.Errorf("expected nil opportunity below threshold, got profit=%.2f (%.2f%%)", opp.ProfitUSD, opp.ProfitPct)
	}
}

func TestDetect_HighGas_EliminatesOpportunity(t *testing.T) {
	detector := newTestDetector()

	// Small spread that would be profitable with low gas, but not with high gas
	opp := detector.Detect(
		400,
		1.0,    // only 1 ETH — small trade, gas cost matters more
		2100.0,
		2099.0,
		2115.0, // $15 spread per ETH = $15 gross profit
		2120.0,
		500.0,  // 500 gwei — extremely high gas, costs ~$50
		2100.0,
	)

	if opp != nil {
		t.Errorf("expected nil — gas cost should eliminate opportunity, got profit=%.2f", opp.ProfitUSD)
	}
}

func TestDetect_ReturnsCEXtoDEX_WhenOnlyThatDirectionProfitable(t *testing.T) {
	detector := newTestDetector()

	// CEXtoDEX: buy at $2100, sell on DEX at $2200 → very profitable
	// DEXtoCEX: would need to buy on DEX at $2300 and sell on CEX at $2099 → loss
	opp := detector.Detect(
		500,
		10.0,
		2100.0, // cexAsk — cheap on Binance
		2099.0, // cexBid
		2200.0, // dexSell — Uniswap pays well
		2300.0, // dexBuy — too expensive to buy on Uniswap
		20.0,
		2100.0,
	)

	if opp == nil {
		t.Fatal("expected opportunity, got nil")
	}
	if opp.Direction != arbitrage.CEXtoDEX {
		t.Errorf("expected CEXtoDEX, got %v", opp.Direction)
	}
}

func TestDetect_GasCostIncludedInProfit(t *testing.T) {
	detector := newTestDetector()

	// Run same scenario with different gas prices — profit should differ
	lowGas := detector.Detect(100, 10.0, 2100.0, 2099.0, 2150.0, 2160.0, 5.0, 2100.0)
	highGas := detector.Detect(100, 10.0, 2100.0, 2099.0, 2150.0, 2160.0, 100.0, 2100.0)

	if lowGas == nil || highGas == nil {
		t.Fatal("both should find opportunities")
	}
	if lowGas.ProfitUSD <= highGas.ProfitUSD {
		t.Errorf("low gas should yield higher profit: low=%.2f high=%.2f", lowGas.ProfitUSD, highGas.ProfitUSD)
	}
}
