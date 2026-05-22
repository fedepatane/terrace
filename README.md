# CEX-DEX Arbitrage Bot

A real-time arbitrage detection system in Go that monitors price discrepancies between a centralized exchange (CEX) orderbook and Uniswap V3 DEX pricing.

## How It Works

On every Ethereum block (~12 seconds), the bot:
1. Fetches the Binance orderbook for each configured trading pair
2. Queries the Uniswap V3 QuoterV2 contract for equivalent DEX pricing
3. Compares both prices across configurable trade sizes (1, 10, 100 ETH)
4. Reports any profitable arbitrage opportunity accounting for fees, slippage, and gas costs

```
Ethereum block arrives
    ├── goroutine: 1 ETH  → Binance ask/bid + Uniswap sell/buy → detect
    ├── goroutine: 10 ETH → Binance ask/bid + Uniswap sell/buy → detect
    └── goroutine: 100 ETH→ Binance ask/bid + Uniswap sell/buy → detect
```

## Architecture

```
cmd/
    main.go              — wires components, main loop
internal/
    arbitrage/
        detector.go      — arbitrage math, profit calculation
        opportunity.go   — output formatting
    cex/
        provider.go      — OrderbookProvider interface
        binance.go       — Binance REST API integration
    dex/
        quoter.go        — PriceQuoter interface
        uniswap.go       — Uniswap V3 QuoterV2 contract calls
        abi/
            quoterv2.json — QuoterV2 ABI (embedded at compile time)
    ethereum/
        streamer.go      — WebSocket block streaming with reconnection
    eth/
        convert.go       — token decimal conversions (wei, raw amounts)
    cache/
        cache.go         — generic thread-safe TTL cache
    config/
        config.go        — YAML + env var configuration
    apperrors/
        errors.go        — custom error types
```

### Key Design Decisions

**Block-driven polling over event-driven**: requoting on every block gives atomic consistency between CEX and DEX state. Trade-off: 12s latency vs event-driven millisecond latency.

**QuoterV2 simulation**: uses `eth_call` (no gas cost) to simulate swaps on-chain. The contract internally walks Uniswap V3 ticks and returns the exact output including pool fee and slippage.

**Interface-based adapters**: `OrderbookProvider` and `PriceQuoter` are interfaces — adding a new CEX or DEX means implementing the interface without touching the detection logic.

## Requirements

- Go 1.21+

## Setup

```bash
git clone https://github.com/fedepatane/terrace
cd terrace
go mod tidy
cp .env.example .env
```

## Configuration

Copy and edit `config.yaml`:

```yaml
ethereum:
  ws_url: "wss://ethereum.publicnode.com"   # WebSocket for block streaming
  rpc_url: "https://ethereum.publicnode.com" # HTTP for contract calls

binance:
  depth: 100   # orderbook depth (levels per side)

arbitrage:
  trade_sizes_base: [1.0, 10.0, 100.0]  # ETH amounts to check
  min_profit_pct: 0.3                    # minimum net profit % to report
  gas_limit_swap: 200000                 # estimated gas for a Uniswap V3 swap
  cex_fee_rate: 0.001                    # Binance taker fee (0.1%)

pairs:
  - name: "ETH-USDC"
    cex_symbol: "ETHUSDC"
    token_in_address: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"  # WETH
    token_out_address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"  # USDC
    token_in_decimals: 18
    token_out_decimals: 6
    pool_fee: 3000
```

### Environment Variables

Environment variables take priority over the config file:

| Variable | Description | Example |
|---|---|---|
| `ETH_WS_URL` | Ethereum WebSocket URL | `wss://mainnet.infura.io/ws/v3/KEY` |
| `ETH_RPC_URL` | Ethereum HTTP RPC URL | `https://mainnet.infura.io/v3/KEY` |
| `MIN_PROFIT_PCT` | Minimum profit % to report | `0.5` |
| `GAS_LIMIT_SWAP` | Gas limit for swap estimate | `200000` |

## Running

```bash
# default — reads .env automatically, no extra steps needed
go run cmd/main.go

# with a custom config path
go run cmd/main.go /path/to/config.yaml

# build binary
go build -o arbitrage-bot cmd/main.go
./arbitrage-bot
```

## Adding a New Trading Pair

Add an entry to `pairs` in `config.yaml`:

```yaml
pairs:
  - name: "BTC-USDC"
    cex_symbol: "BTCUSDC"
    token_in_address: "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599"  # WBTC
    token_out_address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48" # USDC
    token_in_decimals: 8
    token_out_decimals: 6
    pool_fee: 3000
```

No code changes required.

## Adding a New CEX

Implement the `OrderbookProvider` interface:

```go
type OrderbookProvider interface {
    EffectivePrice(ctx context.Context, amountETH float64, side Side) (float64, error)
}
```

```go
// example: coinbase.go
type CoinbaseProvider struct { ... }

func (p *CoinbaseProvider) EffectivePrice(ctx context.Context, amount float64, side Side) (float64, error) {
    // fetch and walk Coinbase orderbook
}
```

Then pass it in `main.go`:
```go
cexProviders["ETH-USDC"] = coinbase.NewCoinbaseProvider(...)
```

## Sample Output

When an arbitrage opportunity is detected:

```
======================================
=== ARBITRAGE OPPORTUNITY DETECTED ===
======================================
Block Number:     18234567
Timestamp:        2024-01-15 14:23:45 UTC
Direction:        CEX → DEX (Buy on Binance, Sell on Uniswap)

Trade Size:       10.0 ETH
CEX Price:        $2245.30 (effective with slippage)
DEX Price:        $2267.80 (effective with slippage)
Price Difference: $22.50 per ETH

Gas Cost:         $8.50
Estimated Profit: $216.50 (0.96% net)

Execution Steps:
  1. Buy 10.0 ETH on Binance at avg $2245.30
     - Required capital: ~$22453.00 USDC
  2. Transfer ETH to on-chain wallet
  3. Execute Uniswap V3 swap: 10.0 ETH → USDC
     - Pool: 0x88e6A0c2dDD26FEEb64F039a2c41296FcB3f5640 (0.3% fee)
     - Expected output: ~$22678.00 USDC
======================================
```

## Key Interfaces

### `cex.OrderbookProvider`
```go
// EffectivePrice returns the average execution price in USDC/ETH
// for the given trade size, walking the orderbook levels.
EffectivePrice(ctx context.Context, amountETH float64, side Side) (float64, error)
```

### `dex.PriceQuoter`
```go
// QuoteBaseForQuote simulates selling amountBase and returns effective price.
QuoteBaseForQuote(ctx context.Context, blockNumber uint64, pair TradingPair, amountBase float64) (float64, error)

// QuoteQuoteForBase simulates buying amountBase and returns effective price.
QuoteQuoteForBase(ctx context.Context, blockNumber uint64, pair TradingPair, amountBase float64) (float64, error)
```

## Known Limitations & TODOs

- **Resume from last block**: `lastBlock` is tracked atomically but missed blocks during reconnection are not re-fetched. In practice this is acceptable for detection-only use since stale prices have no arbitrage value.
- **Worker pool**: goroutines are unbounded. Under abnormal conditions (many pairs, many trade sizes) a semaphore-based pool would prevent resource exhaustion.
- **L2 cache**: only L1 in-memory cache is implemented. A Redis L2 would survive restarts and support multi-instance deployments.
- **Circuit breaker**: failing services are retried every block. `sony/gobreaker` would open the circuit after N failures and reduce load on degraded services.
- **No trade execution**: detection only. Extending to execution would require private key management, slippage tolerance on swaps, and MEV protection (e.g. Flashbots).
