package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type SwapEvent struct {
	Sender     common.Address
	Recipient  common.Address
	Amount0In  *big.Int
	Amount1In  *big.Int
	Amount0Out *big.Int
	Amount1Out *big.Int
	To         common.Address
	USDValue   *big.Float
	TxHash     common.Hash
}
