package dex

import (
	"fmt"
	"strconv"

	abci "github.com/tendermint/tendermint/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	app "github.com/BiJie/BinanceChain/common/types"
	"github.com/BiJie/BinanceChain/plugins/dex/store"
)

const OB_LEVELS = 20

func createAbciQueryHandler(keeper *DexKeeper) app.AbciQueryHandler {
	return func(app app.ChainApp, req abci.RequestQuery, path []string) (res *abci.ResponseQuery) {
		// expects at least two query path segments.
		if path[0] != abciQueryPrefix || len(path) < 2 {
			return nil
		}
		switch path[1] {
		case "pairs": // args: ["dex", "pairs", <offset>, <limit>]
			if len(path) < 4 {
				return &abci.ResponseQuery{
					Code: uint32(sdk.CodeUnknownRequest),
					Log: fmt.Sprintf(
						"%s %s query requires offset and limit in the path",
						abciQueryPrefix, path[1]),
				}
			}
			ctx := app.GetContextForCheckState()
			pairs := keeper.PairMapper.ListAllTradingPairs(ctx)
			offset, err := strconv.Atoi(path[2])
			if err != nil || offset < 0 || offset > len(pairs)-1 {
				return &abci.ResponseQuery{
					Code: uint32(sdk.CodeInternal),
					Log:  "unable to parse offset",
				}
			}
			limit, err := strconv.Atoi(path[3])
			if err != nil || limit <= 0 {
				return &abci.ResponseQuery{
					Code: uint32(sdk.CodeInternal),
					Log:  "unable to parse limit",
				}
			}
			end := offset + limit
			if end > len(pairs) {
				end = len(pairs)
			}
			if end <= 0 || end <= offset {
				return &abci.ResponseQuery{
					Code: uint32(sdk.CodeInternal),
					Log:  "malformed range",
				}
			}
			bz, err := app.GetCodec().MarshalBinary(
				pairs[offset:end],
			)
			if err != nil {
				return &abci.ResponseQuery{
					Code: uint32(sdk.CodeInternal),
					Log:  err.Error(),
				}
			}
			return &abci.ResponseQuery{
				Code:  uint32(sdk.ABCICodeOK),
				Value: bz,
			}
		case "orderbook": // args: ["dex", "orderbook"]
			//TODO: sync lock, validate pair, level number
			if len(path) < 3 {
				return &abci.ResponseQuery{
					Code: uint32(sdk.CodeUnknownRequest),
					Log:  "OrderBook query requires the pair symbol",
				}
			}
			pair := path[2]
			height := app.GetContextForCheckState().BlockHeight()
			levels := keeper.GetOrderBookLevels(pair, OB_LEVELS)
			book := store.OrderBook{
				Height: height,
				Levels: levels,
			}
			bz, err := app.GetCodec().MarshalBinary(book)
			if err != nil {
				return &abci.ResponseQuery{
					Code: uint32(sdk.CodeInternal),
					Log:  err.Error(),
				}
			}
			return &abci.ResponseQuery{
				Code:  uint32(sdk.ABCICodeOK),
				Value: bz,
			}
		default:
			return &abci.ResponseQuery{
				Code: uint32(sdk.ABCICodeOK),
				Info: fmt.Sprintf(
					"Unknown `%s` query path: %v",
					abciQueryPrefix, path),
			}
		}
	}
}