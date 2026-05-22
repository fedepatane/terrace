package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Ethereum  EthereumConfig  `yaml:"ethereum"`
	Binance   BinanceConfig   `yaml:"binance"`
	Arbitrage ArbitrageConfig `yaml:"arbitrage"`
	Pairs     []TradingPair   `yaml:"pairs"`
}

type EthereumConfig struct {
	WSURL          string        `yaml:"ws_url"`
	RPCURL         string        `yaml:"rpc_url"`
	ReconnectDelay time.Duration `yaml:"reconnect_delay"`
	MaxBackoff     time.Duration `yaml:"max_backoff"`
	PingInterval   time.Duration `yaml:"ping_interval"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
}

type BinanceConfig struct {
	BaseURL string `yaml:"base_url"`
	Depth   int    `yaml:"depth"`
}

type ArbitrageConfig struct {
	TradeSizesBase []float64 `yaml:"trade_sizes_base"`
	MinProfitPct   float64   `yaml:"min_profit_pct"`
	GasLimitSwap   uint64    `yaml:"gas_limit_swap"`
	CEXFeeRate     float64   `yaml:"cex_fee_rate"`
}

// TradingPair define un par de trading con toda la info necesaria
// para operar tanto en el CEX como en el DEX.
type TradingPair struct {
	Name              string `yaml:"name"`
	CEXSymbol         string `yaml:"cex_symbol"`
	TokenInAddress    string `yaml:"token_in_address"`
	TokenOutAddress   string `yaml:"token_out_address"`
	TokenInDecimals   int64  `yaml:"token_in_decimals"`
	TokenOutDecimals  int64  `yaml:"token_out_decimals"`
	PoolFee           int64  `yaml:"pool_fee"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.applyDefaults()
	cfg.applyEnvOverrides()
	return &cfg, nil
}

func Default() *Config {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.applyEnvOverrides()
	return cfg
}

// applyEnvOverrides sobreescribe valores del config con variables de entorno.
// Las env vars tienen prioridad sobre el archivo YAML.
//
// Variables disponibles:
//   ETH_WS_URL       — WebSocket URL del nodo Ethereum
//   ETH_RPC_URL      — HTTP RPC URL del nodo Ethereum
//   MIN_PROFIT_PCT   — porcentaje mínimo de profit para reportar (ej: "0.3")
//   GAS_LIMIT_SWAP   — gas limit estimado para un swap en Uniswap (ej: "200000")
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("ETH_WS_URL"); v != "" {
		c.Ethereum.WSURL = v
	}
	if v := os.Getenv("ETH_RPC_URL"); v != "" {
		c.Ethereum.RPCURL = v
	}
	if v := os.Getenv("MIN_PROFIT_PCT"); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			c.Arbitrage.MinProfitPct = parsed
		}
	}
	if v := os.Getenv("GAS_LIMIT_SWAP"); v != "" {
		if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
			c.Arbitrage.GasLimitSwap = parsed
		}
	}
}

func (c *Config) applyDefaults() {
	if c.Ethereum.WSURL == "" {
		c.Ethereum.WSURL = "wss://ethereum.publicnode.com"
	}
	if c.Ethereum.RPCURL == "" {
		c.Ethereum.RPCURL = "https://ethereum.publicnode.com"
	}
	if c.Ethereum.ReconnectDelay == 0 {
		c.Ethereum.ReconnectDelay = time.Second
	}
	if c.Ethereum.MaxBackoff == 0 {
		c.Ethereum.MaxBackoff = 30 * time.Second
	}
	if c.Ethereum.PingInterval == 0 {
		c.Ethereum.PingInterval = 20 * time.Second
	}
	if c.Ethereum.ReadTimeout == 0 {
		c.Ethereum.ReadTimeout = 35 * time.Second
	}
	if c.Binance.BaseURL == "" {
		c.Binance.BaseURL = "https://api.binance.com"
	}
	if c.Binance.Depth == 0 {
		c.Binance.Depth = 100
	}
	if len(c.Arbitrage.TradeSizesBase) == 0 {
		c.Arbitrage.TradeSizesBase = []float64{1.0, 10.0, 100.0}
	}
	if c.Arbitrage.MinProfitPct == 0 {
		c.Arbitrage.MinProfitPct = 0.3
	}
	if c.Arbitrage.GasLimitSwap == 0 {
		c.Arbitrage.GasLimitSwap = 200_000
	}
	if c.Arbitrage.CEXFeeRate == 0 {
		c.Arbitrage.CEXFeeRate = 0.001
	}
	if len(c.Pairs) == 0 {
		c.Pairs = []TradingPair{
			{
				Name:             "ETH-USDC",
				CEXSymbol:        "ETHUSDC",
				TokenInAddress:   "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
				TokenOutAddress:  "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
				TokenInDecimals:  18,
				TokenOutDecimals: 6,
				PoolFee:          3000,
			},
		}
	}
}
