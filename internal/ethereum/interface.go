package ethereum

import (
	"math/big"

	"github.com/SIMPLYBOYS/trading_ace/internal/types"
)

// EthereumService interface defines the methods we need from the Ethereum client
type EthereumService interface {
	GetEthereumPrice() (*big.Float, error)
	GetLatestBlockNumber() (uint64, error)
	FetchSwapEvents(fromBlock, toBlock uint64) ([]types.SwapEvent, error)
	ParseSwapEvent(log interface{}) (*types.SwapEvent, error)
	Close()
}

// SwapEvent represents a swap event from the Ethereum blockchain
type SwapEvent struct {
	TxHash    string
	Sender    string
	Recipient string
	USDValue  float64
	// Add other necessary fields
}
