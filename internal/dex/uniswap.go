package dex

import (
	_ "embed"
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/fpatane/arbitrage-bot/internal/apperrors"
	"github.com/fpatane/arbitrage-bot/internal/cache"
	"github.com/fpatane/arbitrage-bot/internal/config"
	"github.com/fpatane/arbitrage-bot/internal/eth"
)

//go:embed abi/quoterv2.json
var quoterV2ABI string

// QuoterV2 contract address on Ethereum mainnet.
var quoterV2Address = common.HexToAddress("0x61fFE014bA17989E743c5F6cB21bF9697530B21e")

type exactInputParams struct {
	TokenIn           common.Address
	TokenOut          common.Address
	AmountIn          *big.Int
	Fee               *big.Int
	SqrtPriceLimitX96 *big.Int
}

type exactOutputParams struct {
	TokenIn           common.Address
	TokenOut          common.Address
	Amount            *big.Int
	Fee               *big.Int
	SqrtPriceLimitX96 *big.Int
}

type UniswapQuoter struct {
	ethClient     *ethclient.Client
	parsedABI     abi.ABI
	priceCache    *cache.Cache[float64]
	gasPriceCache *cache.Cache[float64]
}

func NewUniswapQuoter(rpcURL string) (*UniswapQuoter, error) {
	ethClient, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("connect to ethereum: %w", err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(quoterV2ABI))
	if err != nil {
		return nil, fmt.Errorf("parse ABI: %w", err)
	}
	return &UniswapQuoter{
		ethClient:     ethClient,
		parsedABI:     parsedABI,
		priceCache:    cache.New[float64](),
		gasPriceCache: cache.New[float64](),
	}, nil
}

// QuoteBaseForQuote simulates selling exactly amountBase of the base token to receive the quote token.
// The returned price already includes the pool fee and real slippage (via QuoterV2 simulation).
func (quoter *UniswapQuoter) QuoteBaseForQuote(ctx context.Context, blockNumber uint64, pair config.TradingPair, amountBase float64) (float64, error) {
	cacheKey := fmt.Sprintf("base-quote-%s-%d-%.4f", pair.Name, blockNumber, amountBase)
	if cachedPrice, found := quoter.priceCache.Get(cacheKey); found {
		return cachedPrice, nil
	}

	params := exactInputParams{
		TokenIn:           common.HexToAddress(pair.TokenInAddress),
		TokenOut:          common.HexToAddress(pair.TokenOutAddress),
		AmountIn:          eth.EthToWei(amountBase),
		Fee:               big.NewInt(pair.PoolFee),
		SqrtPriceLimitX96: big.NewInt(0),
	}

	rawQuoteOut, err := quoter.callQuoteExactInput(ctx, params)
	if err != nil {
		return 0, &apperrors.PriceFetchError{Source: "uniswap", Pair: pair.Name, Err: err}
	}

	quoteReceived := eth.RawToFloat(rawQuoteOut, pair.TokenOutDecimals)
	pricePerBase := quoteReceived / amountBase

	slog.Debug("uniswap quote base→quote", "pair", pair.Name, "base", amountBase, "quote_received", quoteReceived, "price", pricePerBase)
	quoter.priceCache.Set(cacheKey, pricePerBase, 15*time.Second)
	return pricePerBase, nil
}

// QuoteQuoteForBase simulates buying exactly amountBase of the base token paying with the quote token.
// The returned price already includes the pool fee and real slippage (via QuoterV2 simulation).
func (quoter *UniswapQuoter) QuoteQuoteForBase(ctx context.Context, blockNumber uint64, pair config.TradingPair, amountBase float64) (float64, error) {
	cacheKey := fmt.Sprintf("quote-base-%s-%d-%.4f", pair.Name, blockNumber, amountBase)
	if cachedPrice, found := quoter.priceCache.Get(cacheKey); found {
		return cachedPrice, nil
	}

	params := exactOutputParams{
		TokenIn:           common.HexToAddress(pair.TokenOutAddress),
		TokenOut:          common.HexToAddress(pair.TokenInAddress),
		Amount:            eth.EthToWei(amountBase),
		Fee:               big.NewInt(pair.PoolFee),
		SqrtPriceLimitX96: big.NewInt(0),
	}

	rawQuoteIn, err := quoter.callQuoteExactOutput(ctx, params)
	if err != nil {
		return 0, &apperrors.PriceFetchError{Source: "uniswap", Pair: pair.Name, Err: err}
	}

	quotePaid := eth.RawToFloat(rawQuoteIn, pair.TokenOutDecimals)
	pricePerBase := quotePaid / amountBase

	slog.Debug("uniswap quote quote→base", "pair", pair.Name, "base_wanted", amountBase, "quote_paid", quotePaid, "price", pricePerBase)
	quoter.priceCache.Set(cacheKey, pricePerBase, 15*time.Second)
	return pricePerBase, nil
}

func (quoter *UniswapQuoter) GasPrice(ctx context.Context) (float64, error) {
	const cacheKey = "gas_price"
	const cacheTTL = 30 * time.Second

	if cachedGasPrice, found := quoter.gasPriceCache.Get(cacheKey); found {
		return cachedGasPrice, nil
	}

	gasPrice, err := quoter.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return 20.0, nil // conservative fallback: 20 gwei
	}

	gasPriceGwei := float64(gasPrice.Int64()) / 1e9
	quoter.gasPriceCache.Set(cacheKey, gasPriceGwei, cacheTTL)
	return gasPriceGwei, nil
}

func (quoter *UniswapQuoter) callQuoteExactInput(ctx context.Context, params exactInputParams) (*big.Int, error) {
	calldata, err := quoter.parsedABI.Pack("quoteExactInputSingle", params)
	if err != nil {
		return nil, fmt.Errorf("encode call: %w", err)
	}
	result, err := quoter.ethClient.CallContract(ctx, ethereum.CallMsg{To: &quoterV2Address, Data: calldata}, nil)
	if err != nil {
		return nil, fmt.Errorf("eth_call quoteExactInputSingle: %w", err)
	}
	outputs, err := quoter.parsedABI.Unpack("quoteExactInputSingle", result)
	if err != nil {
		return nil, fmt.Errorf("decode result: %w", err)
	}
	amountOut, ok := outputs[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected output type: %T", outputs[0])
	}
	return amountOut, nil
}

func (quoter *UniswapQuoter) callQuoteExactOutput(ctx context.Context, params exactOutputParams) (*big.Int, error) {
	calldata, err := quoter.parsedABI.Pack("quoteExactOutputSingle", params)
	if err != nil {
		return nil, fmt.Errorf("encode call: %w", err)
	}
	result, err := quoter.ethClient.CallContract(ctx, ethereum.CallMsg{To: &quoterV2Address, Data: calldata}, nil)
	if err != nil {
		return nil, fmt.Errorf("eth_call quoteExactOutputSingle: %w", err)
	}
	outputs, err := quoter.parsedABI.Unpack("quoteExactOutputSingle", result)
	if err != nil {
		return nil, fmt.Errorf("decode result: %w", err)
	}
	amountIn, ok := outputs[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected output type: %T", outputs[0])
	}
	return amountIn, nil
}
