package lib

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

type AppBlock struct {
	Header                *types.Header
	TxesN                 int
	MinerBalanceAtParent  *big.Int
	AppTxes               []AppTx
	TABWithoutMiner       *big.Int
	TABWithoutMinerPretty *big.Float
	TABWithMiner          *big.Int
	TABWithMinerPretty    *big.Float
}

type AppTx struct {
	CTransaction          *types.Transaction
	From                  common.Address
	BalanceAtParent       *big.Int
	BalanceAtParentPretty *big.Float
	Index                 int
}

var etherBig = big.NewFloat(params.Ether)

func PrettyBalance(bal *big.Int) *big.Float {
	return new(big.Float).Quo(new(big.Float).SetInt(bal), etherBig)
}
