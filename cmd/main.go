package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fpatane/arbitrage-bot/internal/arbitrage"
	"github.com/fpatane/arbitrage-bot/internal/cex"
	"github.com/fpatane/arbitrage-bot/internal/config"
	"github.com/fpatane/arbitrage-bot/internal/dex"
	"github.com/fpatane/arbitrage-bot/internal/ethereum"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Warn("config file not found, using defaults", "err", err)
		cfg = config.Default()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	streamer := ethereum.NewStreamer(
		cfg.Ethereum.WSURL,
		cfg.Ethereum.PingInterval,
		cfg.Ethereum.ReadTimeout,
		cfg.Ethereum.ReconnectDelay,
		cfg.Ethereum.MaxBackoff,
	)

	// One OrderbookProvider per pair — each has its own symbol and cache.
	cexProviders := make(map[string]cex.OrderbookProvider, len(cfg.Pairs))
	for _, pair := range cfg.Pairs {
		cexProviders[pair.Name] = cex.NewBinanceProvider(cfg.Binance.BaseURL, pair.CEXSymbol, cfg.Binance.Depth)
	}

	priceQuoter, err := dex.NewUniswapQuoter(cfg.Ethereum.RPCURL)
	if err != nil {
		slog.Error("failed to connect to ethereum RPC", "err", err)
		os.Exit(1)
	}

	detector := arbitrage.NewDetector(
		cfg.Arbitrage.CEXFeeRate,
		cfg.Arbitrage.GasLimitSwap,
		cfg.Arbitrage.MinProfitPct,
	)

	slog.Info("arbitrage bot started",
		"pairs", len(cfg.Pairs),
		"trade_sizes", cfg.Arbitrage.TradeSizesBase,
		"min_profit_pct", cfg.Arbitrage.MinProfitPct,
	)

	var opportunitiesFound atomic.Uint64

	blocks := streamer.Stream(ctx)
	for block := range blocks {
		start := time.Now()
		processBlock(ctx, block, cfg, cexProviders, priceQuoter, detector, &opportunitiesFound)
		slog.Info("block processed",
			"number", block.NumberInt,
			"duration_ms", time.Since(start).Milliseconds(),
			"total_opportunities", opportunitiesFound.Load(),
		)
	}

	slog.Info("shutdown complete", "total_opportunities_detected", opportunitiesFound.Load())
}

func processBlock(
	ctx context.Context,
	block ethereum.BlockHeader,
	cfg *config.Config,
	cexProviders map[string]cex.OrderbookProvider,
	priceQuoter dex.PriceQuoter,
	detector *arbitrage.Detector,
	opportunitiesFound *atomic.Uint64,
) {
	gasPriceGwei, err := priceQuoter.GasPrice(ctx)
	if err != nil {
		slog.Warn("failed to get gas price, using 20 gwei fallback", "err", err)
		gasPriceGwei = 20.0
	}

	var waitGroup sync.WaitGroup
	for _, pair := range cfg.Pairs {
		for _, tradeSize := range cfg.Arbitrage.TradeSizesBase {
			pair := pair
			tradeSize := tradeSize
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()
				checkArbitrage(ctx, block, pair, tradeSize, gasPriceGwei, cexProviders[pair.Name], priceQuoter, detector, opportunitiesFound)
			}()
		}
	}
	waitGroup.Wait()
}

// priceResult groups the value and error from a price fetch.
type priceResult struct {
	price float64
	err   error
}

func checkArbitrage(
	ctx context.Context,
	block ethereum.BlockHeader,
	pair config.TradingPair,
	tradeSize float64,
	gasPriceGwei float64,
	orderbookProvider cex.OrderbookProvider,
	priceQuoter dex.PriceQuoter,
	detector *arbitrage.Detector,
	opportunitiesFound *atomic.Uint64,
) {
	cexAskCh  := make(chan priceResult, 1)
	cexBidCh  := make(chan priceResult, 1)
	dexSellCh := make(chan priceResult, 1)
	dexBuyCh  := make(chan priceResult, 1)

	go func() {
		price, err := orderbookProvider.EffectivePrice(ctx, tradeSize, cex.Buy)
		cexAskCh <- priceResult{price, err}
	}()
	go func() {
		price, err := orderbookProvider.EffectivePrice(ctx, tradeSize, cex.Sell)
		cexBidCh <- priceResult{price, err}
	}()
	go func() {
		price, err := priceQuoter.QuoteBaseForQuote(ctx, block.NumberInt, pair, tradeSize)
		dexSellCh <- priceResult{price, err}
	}()
	go func() {
		price, err := priceQuoter.QuoteQuoteForBase(ctx, block.NumberInt, pair, tradeSize)
		dexBuyCh <- priceResult{price, err}
	}()

	cexAsk  := <-cexAskCh
	cexBid  := <-cexBidCh
	dexSell := <-dexSellCh
	dexBuy  := <-dexBuyCh

	allPrices := []struct {
		source string
		result priceResult
	}{
		{"cex_ask",  cexAsk},
		{"cex_bid",  cexBid},
		{"dex_sell", dexSell},
		{"dex_buy",  dexBuy},
	}
	for _, priceData := range allPrices {
		if priceData.result.err != nil {
			slog.Warn("price fetch error", "pair", pair.Name, "source", priceData.source, "size", tradeSize, "err", priceData.result.err)
			return
		}
	}

	ethPriceUSD := (cexAsk.price + cexBid.price) / 2

	opportunity := detector.Detect(
		block.NumberInt,
		tradeSize,
		cexAsk.price, cexBid.price,
		dexSell.price, dexBuy.price,
		gasPriceGwei,
		ethPriceUSD,
	)

	if opportunity != nil {
		opportunitiesFound.Add(1)
		opportunity.Print()
		return
	}

	// Log best spread even when no opportunity — useful for monitoring market conditions.
	cexToDexSpread := (dexSell.price - cexAsk.price) / cexAsk.price * 100
	dexToCexSpread := (cexBid.price - dexBuy.price) / dexBuy.price * 100
	bestSpreadPct := max(cexToDexSpread, dexToCexSpread)

	slog.Debug("no opportunity",
		"pair", pair.Name,
		"size", tradeSize,
		"cex_ask", cexAsk.price,
		"cex_bid", cexBid.price,
		"dex_sell", dexSell.price,
		"dex_buy", dexBuy.price,
		"best_spread_pct", bestSpreadPct,
	)
}
