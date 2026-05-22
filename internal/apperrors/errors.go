package apperrors

import "fmt"

// PriceFetchError is returned when fetching a price from an external source fails.
type PriceFetchError struct {
	Source string // e.g. "binance", "uniswap"
	Pair   string // e.g. "ETH-USDC"
	Err    error
}

func (e *PriceFetchError) Error() string {
	return fmt.Sprintf("price fetch failed [%s/%s]: %v", e.Source, e.Pair, e.Err)
}

func (e *PriceFetchError) Unwrap() error { return e.Err }

// ConnectionError is returned when a connection to an external service fails.
type ConnectionError struct {
	Service string // e.g. "ethereum-ws", "ethereum-rpc"
	Err     error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("connection failed [%s]: %v", e.Service, e.Err)
}

func (e *ConnectionError) Unwrap() error { return e.Err }

// OrderbookError is returned when the orderbook has insufficient liquidity.
type OrderbookError struct {
	Symbol    string
	AmountETH float64
	Unfilled  float64
}

func (e *OrderbookError) Error() string {
	return fmt.Sprintf("insufficient liquidity on %s: wanted %.4f ETH, unfilled %.6f ETH", e.Symbol, e.AmountETH, e.Unfilled)
}
