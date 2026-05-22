package dex

import (
	"context"

	"github.com/fpatane/arbitrage-bot/internal/config"
)

// PriceQuoter obtiene precios efectivos de swap desde un exchange descentralizado.
// Los métodos son genéricos — operan sobre cualquier TradingPair configurado.
type PriceQuoter interface {
	// QuoteBaseForQuote devuelve el precio efectivo al vender amountBase del token base
	// y recibir el token quote. Ejemplo: vender ETH y recibir USDC.
	QuoteBaseForQuote(ctx context.Context, blockNumber uint64, pair config.TradingPair, amountBase float64) (float64, error)

	// QuoteQuoteForBase devuelve el precio efectivo al comprar exactamente amountBase
	// del token base pagando con el token quote. Ejemplo: comprar ETH pagando USDC.
	QuoteQuoteForBase(ctx context.Context, blockNumber uint64, pair config.TradingPair, amountBase float64) (float64, error)

	// GasPrice devuelve el gas price actual sugerido en Gwei.
	GasPrice(ctx context.Context) (float64, error)
}
