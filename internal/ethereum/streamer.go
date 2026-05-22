package ethereum

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/fpatane/arbitrage-bot/internal/eth"
)

type BlockHeader struct {
	Number    string `json:"number"`
	Hash      string `json:"hash"`
	Timestamp string `json:"timestamp"`
	NumberInt uint64
}

type Streamer struct {
	wsURL        string
	pingInterval time.Duration
	readTimeout  time.Duration
	initialDelay time.Duration
	maxBackoff   time.Duration
	lastBlock    atomic.Uint64
}

func NewStreamer(wsURL string, pingInterval, readTimeout, initialDelay, maxBackoff time.Duration) *Streamer {
	return &Streamer{
		wsURL:        wsURL,
		pingInterval: pingInterval,
		readTimeout:  readTimeout,
		initialDelay: initialDelay,
		maxBackoff:   maxBackoff,
	}
}

func (streamer *Streamer) LastBlock() uint64 {
	return streamer.lastBlock.Load()
}

// Stream returns a channel that emits block headers. Closed when ctx is cancelled.
func (streamer *Streamer) Stream(ctx context.Context) <-chan BlockHeader {
	blockChannel := make(chan BlockHeader, 1)
	go streamer.reconnectLoop(ctx, blockChannel)
	return blockChannel
}

func (streamer *Streamer) reconnectLoop(ctx context.Context, blockChannel chan<- BlockHeader) {
	defer close(blockChannel)

	delay := streamer.initialDelay
	for {
		if err := streamer.connect(ctx, blockChannel); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("streamer disconnected, reconnecting", "err", err, "in", delay)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(withJitter(delay)):
		}

		delay = min(time.Duration(float64(delay)*1.5), streamer.maxBackoff)
	}
}

func (streamer *Streamer) connect(ctx context.Context, blockChannel chan<- BlockHeader) error {
	connection, _, err := websocket.DefaultDialer.DialContext(ctx, streamer.wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer connection.Close()

	slog.Info("connected to ethereum node", "url", streamer.wsURL)

	// Renew read deadline on every pong — detects stale connections.
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(streamer.readTimeout))
	})

	if err := streamer.subscribe(connection); err != nil {
		return err
	}

	pingStop := streamer.startPingLoop(ctx, connection)
	defer func() { <-pingStop }()

	return streamer.readLoop(ctx, connection, blockChannel)
}

func (streamer *Streamer) subscribe(connection *websocket.Conn) error {
	subscribeMessage := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "eth_subscribe",
		"params":  []string{"newHeads"},
	}
	if err := connection.WriteJSON(subscribeMessage); err != nil {
		return fmt.Errorf("send subscription: %w", err)
	}

	if err := connection.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	_, rawMessage, err := connection.ReadMessage()
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}

	var confirmation struct {
		Result string          `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(rawMessage, &confirmation); err != nil {
		return fmt.Errorf("parse confirmation: %w", err)
	}
	if confirmation.Error != nil && string(confirmation.Error) != "null" {
		return fmt.Errorf("subscription rejected: %s", confirmation.Error)
	}

	slog.Info("subscribed to newHeads", "sub_id", confirmation.Result)
	return nil
}

func (streamer *Streamer) startPingLoop(ctx context.Context, connection *websocket.Conn) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(streamer.pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := connection.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return done
}

func (streamer *Streamer) readLoop(ctx context.Context, connection *websocket.Conn, blockChannel chan<- BlockHeader) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := connection.SetReadDeadline(time.Now().Add(streamer.readTimeout)); err != nil {
			return err
		}

		_, rawMessage, err := connection.ReadMessage()
		if err != nil {
			return fmt.Errorf("read message: %w", err)
		}

		header, ok := parseBlockHeader(rawMessage)
		if !ok {
			continue
		}

		if blockNum, err := eth.HexToUint64(header.Number); err == nil {
			header.NumberInt = blockNum
			if blockNum > streamer.lastBlock.Load() {
				streamer.lastBlock.Store(blockNum)
			}
		}

		select {
		case blockChannel <- header:
		case <-ctx.Done():
			return ctx.Err()
		default:
			slog.Warn("block channel full, dropping block", "number", header.NumberInt)
		}
	}
}

func parseBlockHeader(rawMessage []byte) (BlockHeader, bool) {
	var notification struct {
		Method string `json:"method"`
		Params struct {
			Result BlockHeader `json:"result"`
		} `json:"params"`
	}
	if err := json.Unmarshal(rawMessage, &notification); err != nil {
		return BlockHeader{}, false
	}
	if notification.Method != "eth_subscription" {
		return BlockHeader{}, false
	}
	return notification.Params.Result, true
}

func withJitter(duration time.Duration) time.Duration {
	jitter := time.Duration(rand.Int64N(int64(duration / 5)))
	return duration + jitter
}
