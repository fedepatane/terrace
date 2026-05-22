package cex

import "context"

// Side indica la dirección de la operación en el orderbook.
type Side int

const (
	Buy  Side = iota // consumimos asks — compramos ETH
	Sell             // consumimos bids — vendemos ETH
)

// OrderbookProvider calcula el precio efectivo de ejecución en un exchange centralizado.
type OrderbookProvider interface {
	EffectivePrice(ctx context.Context, amountETH float64, side Side) (float64, error)
}
