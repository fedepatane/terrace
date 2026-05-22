package arbitrage_test

import (
	"context"
	"testing"

	"github.com/fpatane/arbitrage-bot/internal/arbitrage"
	"github.com/fpatane/arbitrage-bot/internal/cex"
	"github.com/fpatane/arbitrage-bot/internal/config"
	"github.com/fpatane/arbitrage-bot/internal/dex"
)

// --- Mocks ---

type mockOrderbookProvider struct {
	askPrice float64
	bidPrice float64
}

func (m *mockOrderbookProvider) EffectivePrice(_ context.Context, _ float64, side cex.Side) (float64, error) {
	if side == cex.Buy {
		return m.askPrice, nil
	}
	return m.bidPrice, nil
}

type mockPriceQuoter struct {
	sellPrice float64 // what Uniswap pays when we sell base token
	buyPrice  float64 // what Uniswap charges when we buy base token
	gasPrice  float64
}

func (m *mockPriceQuoter) QuoteBaseForQuote(_ context.Context, _ uint64, _ config.TradingPair, _ float64) (float64, error) {
	return m.sellPrice, nil
}

func (m *mockPriceQuoter) QuoteQuoteForBase(_ context.Context, _ uint64, _ config.TradingPair, _ float64) (float64, error) {
	return m.buyPrice, nil
}

func (m *mockPriceQuoter) GasPrice(_ context.Context) (float64, error) {
	return m.gasPrice, nil
}

// --- Tests ---

func TestIntegration_OpportunityDetected_CEXtoDEX(t *testing.T) {
	// Binance is cheap, Uniswap pays more
	orderbookProvider := &mockOrderbookProvider{askPrice: 2100.0, bidPrice: 2099.0}
	priceQuoter := &mockPriceQuoter{sellPrice: 2200.0, buyPrice: 2300.0, gasPrice: 20.0}
	detector := arbitrage.NewDetector(0.001, 200_000, 0.3)

	pair := config.TradingPair{Name: "ETH-USDC"}
	tradeSize := 10.0

	cexAsk, _ := orderbookProvider.EffectivePrice(context.Background(), tradeSize, cex.Buy)
	cexBid, _ := orderbookProvider.EffectivePrice(context.Background(), tradeSize, cex.Sell)
	dexSell, _ := priceQuoter.QuoteBaseForQuote(context.Background(), 100, pair, tradeSize)
	dexBuy, _ := priceQuoter.QuoteQuoteForBase(context.Background(), 100, pair, tradeSize)
	gasPrice, _ := priceQuoter.GasPrice(context.Background())

	ethPriceUSD := (cexAsk + cexBid) / 2

	opp := detector.Detect(100, tradeSize, cexAsk, cexBid, dexSell, dexBuy, gasPrice, ethPriceUSD)

	if opp == nil {
		t.Fatal("expected opportunity to be detected, got nil")
	}
	if opp.Direction != arbitrage.CEXtoDEX {
		t.Errorf("expected CEXtoDEX direction, got %v", opp.Direction)
	}
	if opp.ProfitUSD <= 0 {
		t.Errorf("expected positive profit, got %.2f", opp.ProfitUSD)
	}
	if opp.ProfitPct < 0.3 {
		t.Errorf("expected profit above threshold 0.3%%, got %.2f%%", opp.ProfitPct)
	}

	t.Logf("opportunity detected: direction=%v profit=$%.2f (%.2f%%)", opp.Direction, opp.ProfitUSD, opp.ProfitPct)
}

func TestIntegration_NoOpportunity_EfficientMarket(t *testing.T) {
	// Prices are equal — no arbitrage possible
	orderbookProvider := &mockOrderbookProvider{askPrice: 2100.0, bidPrice: 2099.0}
	priceQuoter := &mockPriceQuoter{sellPrice: 2098.0, buyPrice: 2102.0, gasPrice: 20.0}
	detector := arbitrage.NewDetector(0.001, 200_000, 0.3)

	pair := config.TradingPair{Name: "ETH-USDC"}
	tradeSize := 10.0

	cexAsk, _ := orderbookProvider.EffectivePrice(context.Background(), tradeSize, cex.Buy)
	cexBid, _ := orderbookProvider.EffectivePrice(context.Background(), tradeSize, cex.Sell)
	dexSell, _ := priceQuoter.QuoteBaseForQuote(context.Background(), 100, pair, tradeSize)
	dexBuy, _ := priceQuoter.QuoteQuoteForBase(context.Background(), 100, pair, tradeSize)
	gasPrice, _ := priceQuoter.GasPrice(context.Background())

	ethPriceUSD := (cexAsk + cexBid) / 2

	opp := detector.Detect(100, tradeSize, cexAsk, cexBid, dexSell, dexBuy, gasPrice, ethPriceUSD)

	if opp != nil {
		t.Errorf("expected no opportunity in efficient market, got profit=$%.2f (%.2f%%)", opp.ProfitUSD, opp.ProfitPct)
	}
}

func TestIntegration_MocksReplaceRealAPIs(t *testing.T) {
	// This test demonstrates that the interface-based design allows
	// testing the full detection flow without any network calls.
	var (
		_ cex.OrderbookProvider = &mockOrderbookProvider{}
		_ dex.PriceQuoter       = &mockPriceQuoter{}
	)
	// If this compiles, the mocks correctly implement the interfaces.
}
