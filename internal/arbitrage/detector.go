package arbitrage

import "time"

// Direction indica en qué exchanges se compra y se vende.
type Direction int

const (
	CEXtoDEX Direction = iota // Comprar en Binance, vender en Uniswap
	DEXtoCEX                  // Comprar en Uniswap, vender en Binance
)

func (direction Direction) String() string {
	if direction == CEXtoDEX {
		return "CEX → DEX (Buy on Binance, Sell on Uniswap)"
	}
	return "DEX → CEX (Buy on Uniswap, Sell on Binance)"
}

// Opportunity representa una oportunidad de arbitraje detectada.
type Opportunity struct {
	BlockNumber  uint64
	Timestamp    time.Time
	Direction    Direction
	TradeSizeETH float64
	CEXPrice     float64 // precio efectivo de ejecución en el lado CEX
	DEXPrice     float64 // precio efectivo de ejecución en el lado DEX
	GasCostUSD   float64
	ProfitUSD    float64
	ProfitPct    float64
}

// Detector compara precios entre CEX y DEX y calcula si hay arbitraje rentable.
type Detector struct {
	cexFeeRate   float64
	gasLimitSwap uint64
	minProfitPct float64
}

func NewDetector(cexFeeRate float64, gasLimitSwap uint64, minProfitPct float64) *Detector {
	return &Detector{
		cexFeeRate:   cexFeeRate,
		gasLimitSwap: gasLimitSwap,
		minProfitPct: minProfitPct,
	}
}

// Detect evalúa ambas direcciones de arbitraje y devuelve la mejor oportunidad,
// o nil si ninguna supera el minProfitPct configurado.
//
// cexAskPrice:  precio promedio para COMPRAR ETH en Binance (walk de asks)
// cexBidPrice:  precio promedio para VENDER ETH en Binance (walk de bids)
// dexSellPrice: USDC recibidos por ETH en Uniswap (fee ya incluido por QuoterV2)
// dexBuyPrice:  USDC pagados por ETH en Uniswap  (fee ya incluido por QuoterV2)
func (detector *Detector) Detect(
	blockNumber uint64,
	tradeSize float64,
	cexAskPrice, cexBidPrice float64,
	dexSellPrice, dexBuyPrice float64,
	gasPriceGwei float64,
	ethPriceUSD float64,
) *Opportunity {
	gasCostUSD := detector.estimateGasCostUSD(gasPriceGwei, ethPriceUSD)

	cexToDexOpportunity := detector.evaluateCEXtoDEX(blockNumber, tradeSize, cexAskPrice, dexSellPrice, gasCostUSD)
	dexToCexOpportunity := detector.evaluateDEXtoCEX(blockNumber, tradeSize, cexBidPrice, dexBuyPrice, gasCostUSD)

	// Devolvemos la más rentable de las dos
	if cexToDexOpportunity != nil && (dexToCexOpportunity == nil || cexToDexOpportunity.ProfitUSD >= dexToCexOpportunity.ProfitUSD) {
		return cexToDexOpportunity
	}
	return dexToCexOpportunity
}

// evaluateCEXtoDEX calcula: comprar ETH en Binance (ask) y vender en Uniswap.
func (detector *Detector) evaluateCEXtoDEX(blockNumber uint64, tradeSize, cexAskPrice, dexSellPrice, gasCostUSD float64) *Opportunity {
	cexBuyCost := tradeSize * cexAskPrice * (1 + detector.cexFeeRate)
	dexSellRevenue := tradeSize * dexSellPrice // fee ya incluido en la quote de Uniswap
	profitUSD := dexSellRevenue - cexBuyCost - gasCostUSD
	profitPct := profitUSD / cexBuyCost * 100

	if profitPct < detector.minProfitPct {
		return nil
	}
	return &Opportunity{
		BlockNumber:  blockNumber,
		Timestamp:    time.Now().UTC(),
		Direction:    CEXtoDEX,
		TradeSizeETH: tradeSize,
		CEXPrice:     cexAskPrice,
		DEXPrice:     dexSellPrice,
		GasCostUSD:   gasCostUSD,
		ProfitUSD:    profitUSD,
		ProfitPct:    profitPct,
	}
}

// evaluateDEXtoCEX calcula: comprar ETH en Uniswap y vender en Binance (bid).
func (detector *Detector) evaluateDEXtoCEX(blockNumber uint64, tradeSize, cexBidPrice, dexBuyPrice, gasCostUSD float64) *Opportunity {
	dexBuyCost := tradeSize * dexBuyPrice // fee ya incluido en la quote de Uniswap
	cexSellRevenue := tradeSize * cexBidPrice * (1 - detector.cexFeeRate)
	profitUSD := cexSellRevenue - dexBuyCost - gasCostUSD
	profitPct := profitUSD / dexBuyCost * 100

	if profitPct < detector.minProfitPct {
		return nil
	}
	return &Opportunity{
		BlockNumber:  blockNumber,
		Timestamp:    time.Now().UTC(),
		Direction:    DEXtoCEX,
		TradeSizeETH: tradeSize,
		CEXPrice:     cexBidPrice,
		DEXPrice:     dexBuyPrice,
		GasCostUSD:   gasCostUSD,
		ProfitUSD:    profitUSD,
		ProfitPct:    profitPct,
	}
}

func (detector *Detector) estimateGasCostUSD(gasPriceGwei, ethPriceUSD float64) float64 {
	gasCostETH := float64(detector.gasLimitSwap) * gasPriceGwei * 1e-9
	return gasCostETH * ethPriceUSD
}
