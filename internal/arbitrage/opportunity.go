package arbitrage

import "fmt"

const ethusdcPoolAddress = "0x88e6A0c2dDD26FEEb64F039a2c41296FcB3f5640"

// Print imprime la oportunidad de arbitraje en el formato esperado por el challenge.
func (opportunity *Opportunity) Print() {
	capital := opportunity.TradeSizeETH * opportunity.CEXPrice
	separator := "======================================"

	fmt.Printf("\n%s\n", separator)
	fmt.Printf("=== ARBITRAGE OPPORTUNITY DETECTED ===\n")
	fmt.Printf("%s\n", separator)
	fmt.Printf("Block Number:     %d\n", opportunity.BlockNumber)
	fmt.Printf("Timestamp:        %s\n", opportunity.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("Direction:        %s\n\n", opportunity.Direction)
	fmt.Printf("Trade Size:       %.1f ETH\n", opportunity.TradeSizeETH)
	fmt.Printf("CEX Price:        $%.2f (effective with slippage)\n", opportunity.CEXPrice)
	fmt.Printf("DEX Price:        $%.2f (effective with slippage)\n", opportunity.DEXPrice)
	fmt.Printf("Price Difference: $%.2f per ETH\n\n", opportunity.DEXPrice-opportunity.CEXPrice)
	fmt.Printf("Gas Cost:         $%.2f\n", opportunity.GasCostUSD)
	fmt.Printf("Estimated Profit: $%.2f (%.2f%% net)\n\n", opportunity.ProfitUSD, opportunity.ProfitPct)
	fmt.Printf("Execution Steps:\n")

	if opportunity.Direction == CEXtoDEX {
		fmt.Printf("  1. Buy %.1f ETH on Binance at avg $%.2f\n", opportunity.TradeSizeETH, opportunity.CEXPrice)
		fmt.Printf("     - Required capital: ~$%.2f USDC\n", capital)
		fmt.Printf("  2. Transfer ETH to on-chain wallet\n")
		fmt.Printf("  3. Execute Uniswap V3 swap: %.1f ETH → USDC\n", opportunity.TradeSizeETH)
		fmt.Printf("     - Pool: %s (0.3%% fee)\n", ethusdcPoolAddress)
		fmt.Printf("     - Expected output: ~$%.2f USDC\n", opportunity.TradeSizeETH*opportunity.DEXPrice)
	} else {
		fmt.Printf("  1. Buy %.1f ETH on Uniswap V3 with USDC\n", opportunity.TradeSizeETH)
		fmt.Printf("     - Pool: %s (0.3%% fee)\n", ethusdcPoolAddress)
		fmt.Printf("     - Required capital: ~$%.2f USDC\n", capital)
		fmt.Printf("  2. Transfer ETH to Binance\n")
		fmt.Printf("  3. Sell %.1f ETH on Binance at avg $%.2f\n", opportunity.TradeSizeETH, opportunity.CEXPrice)
		fmt.Printf("     - Expected output: ~$%.2f USDC\n", opportunity.TradeSizeETH*opportunity.CEXPrice)
	}

	fmt.Printf("%s\n\n", separator)
}
