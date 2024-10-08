package main

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	InfuraURL            = "https://mainnet.infura.io/v3/ff484e5e9e3b45829dff73464bc78b26" // Replace with your Infura project ID
	UniswapV2PairAddress = "0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc"                    // WETH/USDC pair
)

var (
	Client *ethclient.Client
)

func InitEthereumClient() error {
	var err error
	Client, err = ethclient.Dial(InfuraURL)
	if err != nil {
		return fmt.Errorf("failed to connect to the Ethereum client: %v", err)
	}
	return nil
}

func FetchSwapEvents(fromBlock, toBlock *big.Int) ([]*types.Log, error) {
	contractAddress := common.HexToAddress(UniswapV2PairAddress)
	query := ethereum.FilterQuery{
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		Addresses: []common.Address{contractAddress},
	}

	logs, err := Client.FilterLogs(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to filter logs: %v", err)
	}

	logPointers := make([]*types.Log, len(logs))
	for i, log := range logs {
		logPointers[i] = &log
	}

	return logPointers, nil
}

func ProcessSwapEvents(logs []*types.Log) {
	for _, vLog := range logs {
		fmt.Printf("Log Block Number: %d\n", vLog.BlockNumber)
		fmt.Printf("Log Index: %d\n", vLog.Index)

		// Here you would typically decode the log data
		// and process the swap event
	}
}
