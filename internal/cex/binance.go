package cex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/fpatane/arbitrage-bot/internal/apperrors"
	"github.com/fpatane/arbitrage-bot/internal/cache"
)

type orderbook struct {
	LastUpdateID int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}

// binanceRateLimit is Binance's public API limit: 1200 requests/minute.
const binanceRateLimit = 1200

type BinanceProvider struct {
	baseURL        string
	symbol         string
	depth          int
	httpClient     *http.Client
	orderbookCache *cache.Cache[orderbook]
	rateLimiter    <-chan time.Time
}

func NewBinanceProvider(baseURL, symbol string, depth int) *BinanceProvider {
	return &BinanceProvider{
		baseURL:        baseURL,
		symbol:         symbol,
		depth:          depth,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		orderbookCache: cache.New[orderbook](),
		rateLimiter:    time.NewTicker(time.Minute / binanceRateLimit).C,
	}
}

func (provider *BinanceProvider) EffectivePrice(ctx context.Context, amountETH float64, side Side) (float64, error) {
	book, err := provider.fetchOrderbook(ctx)
	if err != nil {
		return 0, err
	}

	var levels [][]string
	if side == Buy {
		levels = book.Asks
	} else {
		levels = book.Bids
	}

	return walkLevels(levels, amountETH)
}

// walkLevels computes the average fill price for amountETH across orderbook levels.
func walkLevels(levels [][]string, amountETH float64) (float64, error) {
	remaining := amountETH
	totalCost := 0.0

	for _, level := range levels {
		if len(level) < 2 {
			continue
		}
		price, err := strconv.ParseFloat(level[0], 64)
		if err != nil {
			continue
		}
		quantity, err := strconv.ParseFloat(level[1], 64)
		if err != nil {
			continue
		}

		filled := min(quantity, remaining)
		totalCost += filled * price
		remaining -= filled

		if remaining <= 1e-10 {
			break
		}
	}

	if remaining > 1e-10 {
		return 0, &apperrors.OrderbookError{
			Symbol:    "orderbook",
			AmountETH: amountETH,
			Unfilled:  remaining,
		}
	}

	return totalCost / amountETH, nil
}

func (provider *BinanceProvider) fetchOrderbook(ctx context.Context) (orderbook, error) {
	const cacheKey = "orderbook"
	const cacheTTL = 5 * time.Second

	if cachedBook, found := provider.orderbookCache.Get(cacheKey); found {
		return cachedBook, nil
	}

	// Wait for rate limiter slot before making the request.
	select {
	case <-provider.rateLimiter:
	case <-ctx.Done():
		return orderbook{}, ctx.Err()
	}

	url := fmt.Sprintf("%s/api/v3/depth?symbol=%s&limit=%d", provider.baseURL, provider.symbol, provider.depth)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return orderbook{}, err
	}

	response, err := provider.httpClient.Do(request)
	if err != nil {
		return orderbook{}, &apperrors.ConnectionError{Service: "binance", Err: err}
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return orderbook{}, fmt.Errorf("binance responded with status %d: %s", response.StatusCode, body)
	}

	var book orderbook
	if err := json.NewDecoder(response.Body).Decode(&book); err != nil {
		return orderbook{}, fmt.Errorf("failed to decode orderbook: %w", err)
	}

	slog.Debug("fetched binance orderbook", "symbol", provider.symbol, "bids", len(book.Bids), "asks", len(book.Asks))
	provider.orderbookCache.Set(cacheKey, book, cacheTTL)
	return book, nil
}
