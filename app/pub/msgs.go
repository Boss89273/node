package pub

import (
	"encoding/json"
	"fmt"

	"github.com/linkedin/goavro"

	sdk "github.com/cosmos/cosmos-sdk/types"

	orderPkg "github.com/BiJie/BinanceChain/plugins/dex/order"
)

var (
	booksCodec            *goavro.Codec
	accountCodec          *goavro.Codec
	executionResultsCodec *goavro.Codec
	blockFeeCodec         *goavro.Codec
)

type msgType int8

const (
	accountsTpe = iota
	booksTpe
	executionResultTpe
	blockFeeTpe
)

// the strings should be keep consistence with top level record name in schemas.go
// !!!NOTE!!! Changes of these strings should notice consumers of kafka publisher
func (this msgType) String() string {
	switch this {
	case accountsTpe:
		return "Accounts"
	case booksTpe:
		return "Books"
	case executionResultTpe:
		return "ExecutionResults"
	case blockFeeTpe:
		return "BlockFee"
	default:
		return "Unknown"
	}
}

type AvroMsg interface {
	ToNativeMap() map[string]interface{}
	String() string
}

func marshal(msg AvroMsg, tpe msgType) ([]byte, error) {
	native := msg.ToNativeMap()
	Logger.Debug("msgDetail", "msg", native)
	var codec *goavro.Codec
	switch tpe {
	case accountsTpe:
		codec = accountCodec
	case booksTpe:
		codec = booksCodec
	case executionResultTpe:
		codec = executionResultsCodec
	case blockFeeTpe:
		codec = blockFeeCodec
	default:
		return nil, fmt.Errorf("doesn't support marshal kafka msg tpe: %s", tpe.String())
	}
	bb, err := codec.BinaryFromNative(nil, native)
	if err != nil {
		Logger.Error("failed to serialize message", "msg", msg, "err", err)
	}
	return bb, err
}

type ExecutionResults struct {
	Height    int64
	Timestamp int64 // milli seconds since Epoch
	NumOfMsgs int   // number of individual messages we published, consumer can verify messages they received against this field to make sure they does not miss messages
	Trades    trades
	Orders    Orders
	Proposals Proposals
}

func (msg *ExecutionResults) String() string {
	return fmt.Sprintf("ExecutionResult at height: %d, numOfMsgs: %d", msg.Height, msg.NumOfMsgs)
}

func (msg *ExecutionResults) ToNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["height"] = msg.Height
	native["timestamp"] = msg.Timestamp
	native["numOfMsgs"] = msg.NumOfMsgs
	if msg.Trades.NumOfMsgs > 0 {
		native["trades"] = map[string]interface{}{"org.binance.dex.model.avro.Trades": msg.Trades.ToNativeMap()}
	}
	if msg.Orders.NumOfMsgs > 0 {
		native["orders"] = map[string]interface{}{"org.binance.dex.model.avro.Orders": msg.Orders.ToNativeMap()}
	}
	if msg.Proposals.NumOfMsgs > 0 {
		native["proposals"] = map[string]interface{}{"org.binance.dex.model.avro.Proposals": msg.Proposals.ToNativeMap()}
	}
	return native
}

type trades struct {
	NumOfMsgs int
	Trades    []*Trade
}

func (msg *trades) String() string {
	return fmt.Sprintf("Trades numOfMsgs: %d", msg.NumOfMsgs)
}

func (msg *trades) ToNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["numOfMsgs"] = msg.NumOfMsgs
	ts := make([]map[string]interface{}, len(msg.Trades), len(msg.Trades))
	for idx, trade := range msg.Trades {
		ts[idx] = trade.toNativeMap()
	}
	native["trades"] = ts
	return native
}

type Trade struct {
	Id     string
	Symbol string
	Price  int64
	Qty    int64
	Sid    string
	Bid    string
	Sfee   string
	Bfee   string
	SAddr  string // string representation of AccAddress
	BAddr  string // string representation of AccAddress
}

func (msg *Trade) MarshalJSON() ([]byte, error) {
	type Alias Trade
	return json.Marshal(&struct {
		*Alias
		SAddr string
		BAddr string
	}{
		Alias: (*Alias)(msg),
		SAddr: sdk.AccAddress(msg.SAddr).String(),
		BAddr: sdk.AccAddress(msg.BAddr).String(),
	})
}

func (msg *Trade) String() string {
	return fmt.Sprintf("Trade: %v", msg.toNativeMap())
}

func (msg *Trade) toNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["id"] = msg.Id
	native["symbol"] = msg.Symbol
	native["price"] = msg.Price
	native["qty"] = msg.Qty
	native["sid"] = msg.Sid
	native["bid"] = msg.Bid
	native["sfee"] = msg.Sfee
	native["bfee"] = msg.Bfee
	native["saddr"] = sdk.AccAddress(msg.SAddr).String()
	native["baddr"] = sdk.AccAddress(msg.BAddr).String()
	return native
}

type Orders struct {
	NumOfMsgs int
	Orders    []*Order
}

func (msg *Orders) String() string {
	return fmt.Sprintf("Orders numOfMsgs: %d", msg.NumOfMsgs)
}

func (msg *Orders) ToNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["numOfMsgs"] = msg.NumOfMsgs
	os := make([]map[string]interface{}, len(msg.Orders), len(msg.Orders))
	for idx, o := range msg.Orders {
		os[idx] = o.toNativeMap()
	}
	native["orders"] = os
	return native
}

type Order struct {
	Symbol               string
	Status               orderPkg.ChangeType
	OrderId              string
	TradeId              string
	Owner                string
	Side                 int8
	OrderType            int8
	Price                int64
	Qty                  int64
	lastExecutedPrice    int64
	lastExecutedQty      int64
	cumQty               int64
	fee                  string
	orderCreationTime    int64
	transactionTime      int64
	timeInForce          int8
	currentExecutionType orderPkg.ExecutionType
	txHash               string
}

func (msg *Order) String() string {
	return fmt.Sprintf("Order: %v", msg.toNativeMap())
}

func (msg *Order) effectQtyToOrderBook() int64 {
	switch msg.Status {
	case orderPkg.Ack:
		return msg.Qty
	case orderPkg.FullyFill, orderPkg.PartialFill:
		return -msg.lastExecutedQty
	case orderPkg.Expired, orderPkg.IocNoFill, orderPkg.Canceled:
		return msg.cumQty - msg.Qty // deliberated be negative value
	default:
		Logger.Error("does not supported order status", "order", msg.String())
		return 0
	}
}

func (msg *Order) toNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["symbol"] = msg.Symbol
	native["status"] = msg.Status.String() //TODO(#66): confirm with all teams to make this uint8 enum
	native["orderId"] = msg.OrderId
	native["tradeId"] = msg.TradeId
	native["owner"] = msg.Owner
	native["side"] = orderPkg.IToSide(msg.Side)                //TODO(#66): confirm with all teams to make this uint8 enum
	native["orderType"] = orderPkg.IToOrderType(msg.OrderType) //TODO(#66): confirm with all teams to make this uint8 enum
	native["price"] = msg.Price
	native["qty"] = msg.Qty
	native["lastExecutedPrice"] = msg.lastExecutedPrice
	native["lastExecutedQty"] = msg.lastExecutedQty
	native["cumQty"] = msg.cumQty
	native["fee"] = msg.fee
	native["orderCreationTime"] = msg.orderCreationTime
	native["transactionTime"] = msg.transactionTime
	native["timeInForce"] = orderPkg.IToTimeInForce(msg.timeInForce)   //TODO(#66): confirm with all teams to make this uint8 enum
	native["currentExecutionType"] = msg.currentExecutionType.String() //TODO(#66): confirm with all teams to make this uint8 enum
	native["txHash"] = msg.txHash
	return native
}

type Proposals struct {
	NumOfMsgs int
	Proposals []*Proposal
}

func (msg *Proposals) String() string {
	return fmt.Sprintf("Proposals numOfMsgs: %d", msg.NumOfMsgs)
}

func (msg *Proposals) ToNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["numOfMsgs"] = msg.NumOfMsgs
	ps := make([]map[string]interface{}, len(msg.Proposals), len(msg.Proposals))
	for idx, p := range msg.Proposals {
		ps[idx] = p.toNativeMap()
	}
	native["proposals"] = ps
	return native
}

type ProposalStatus uint8

const (
	Succeed ProposalStatus = iota
	Failed
)

func (this ProposalStatus) String() string {
	switch this {
	case Succeed:
		return "S"
	case Failed:
		return "F"
	default:
		return "Unknown"
	}
}

type Proposal struct {
	Id     int64
	Status ProposalStatus
}

func (msg *Proposal) String() string {
	return fmt.Sprintf("Proposal: %v", msg.toNativeMap())
}

func (msg *Proposal) toNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["id"] = msg.Id
	native["status"] = msg.Status.String()
	return native
}

type PriceLevel struct {
	Price   int64
	LastQty int64
}

func (msg *PriceLevel) String() string {
	return fmt.Sprintf("priceLevel: %s", msg.ToNativeMap())
}

func (msg *PriceLevel) ToNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["price"] = msg.Price
	native["lastQty"] = msg.LastQty
	return native
}

type OrderBookDelta struct {
	Symbol string
	Buys   []PriceLevel
	Sells  []PriceLevel
}

func (msg *OrderBookDelta) String() string {
	return fmt.Sprintf("orderBookDelta for: %s, num of buys prices: %d, num of sell prices: %d", msg.Symbol, len(msg.Buys), len(msg.Sells))
}

func (msg *OrderBookDelta) ToNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["symbol"] = msg.Symbol
	bs := make([]map[string]interface{}, len(msg.Buys), len(msg.Buys))
	for idx, buy := range msg.Buys {
		bs[idx] = buy.ToNativeMap()
	}
	native["buys"] = bs
	ss := make([]map[string]interface{}, len(msg.Sells), len(msg.Sells))
	for idx, sell := range msg.Sells {
		ss[idx] = sell.ToNativeMap()
	}
	native["sells"] = ss
	return native
}

type Books struct {
	Height    int64
	Timestamp int64
	NumOfMsgs int
	Books     []OrderBookDelta
}

func (msg *Books) String() string {
	return fmt.Sprintf("Books at height: %d, numOfMsgs: %d", msg.Height, msg.NumOfMsgs)
}

func (msg *Books) ToNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["height"] = msg.Height
	native["timestamp"] = msg.Timestamp
	native["numOfMsgs"] = msg.NumOfMsgs
	if msg.NumOfMsgs > 0 {
		bs := make([]map[string]interface{}, len(msg.Books), len(msg.Books))
		for idx, book := range msg.Books {
			bs[idx] = book.ToNativeMap()
		}
		native["books"] = bs
	}
	return native
}

type AssetBalance struct {
	Asset  string
	Free   int64
	Frozen int64
	Locked int64
}

func (msg *AssetBalance) String() string {
	return fmt.Sprintf("AssetBalance: %s", msg.ToNativeMap())
}

func (msg *AssetBalance) ToNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["asset"] = msg.Asset
	native["free"] = msg.Free
	native["frozen"] = msg.Frozen
	native["locked"] = msg.Locked
	return native
}

type Account struct {
	Owner    string // string representation of AccAddress
	Fee      string
	Balances []*AssetBalance
}

func (msg *Account) MarshalJSON() ([]byte, error) {
	type Alias Account
	return json.Marshal(&struct {
		*Alias
		Owner string
	}{
		Alias: (*Alias)(msg),
		Owner: sdk.AccAddress(msg.Owner).String(),
	})
}

func (msg *Account) String() string {
	return fmt.Sprintf("Account of: %s, fee: %s, num of balance changes: %d", msg.Owner, msg.Fee, len(msg.Balances))
}

func (msg *Account) ToNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["owner"] = sdk.AccAddress(msg.Owner).String()
	bs := make([]map[string]interface{}, len(msg.Balances), len(msg.Balances))
	for idx, b := range msg.Balances {
		bs[idx] = b.ToNativeMap()
	}
	native["fee"] = msg.Fee
	native["balances"] = bs
	return native
}

type Accounts struct {
	Height    int64
	NumOfMsgs int
	Accounts  []Account
}

func (msg *Accounts) String() string {
	return fmt.Sprintf("Accounts at height: %d, numOfMsgs: %d", msg.Height, msg.NumOfMsgs)
}

func (msg *Accounts) ToNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["height"] = msg.Height
	native["numOfMsgs"] = msg.NumOfMsgs
	if msg.NumOfMsgs > 0 {
		as := make([]map[string]interface{}, len(msg.Accounts), len(msg.Accounts))
		for idx, a := range msg.Accounts {
			as[idx] = a.ToNativeMap()
		}
		native["accounts"] = as
	}
	return native
}

type BlockFee struct {
	Height     int64
	Fee        string
	Validators []string // slice of string wrappers of bytes representation of sdk.AccAddress
}

func (msg BlockFee) MarshalJSON() ([]byte, error) {
	bech32Strs := make([]string, len(msg.Validators), len(msg.Validators))
	for id, val := range msg.Validators {
		bech32Strs[id] = sdk.AccAddress(val).String()
	}
	type Alias BlockFee
	return json.Marshal(&struct {
		Alias
		Validators []string
	}{
		Alias:      (Alias)(msg),
		Validators: bech32Strs,
	})
}

func (msg BlockFee) String() string {
	return fmt.Sprintf("Blockfee at height: %d, fee: %s, validators: %v", msg.Height, msg.Fee, msg.Validators)
}

func (msg BlockFee) ToNativeMap() map[string]interface{} {
	var native = make(map[string]interface{})
	native["height"] = msg.Height
	native["fee"] = msg.Fee
	validators := make([]string, len(msg.Validators), len(msg.Validators))
	for idx, addr := range msg.Validators {
		validators[idx] = sdk.AccAddress(addr).String()
	}
	native["validators"] = validators
	return native
}

func initAvroCodecs() (err error) {
	if executionResultsCodec, err = goavro.NewCodec(executionResultSchema); err != nil {
		return err
	} else if booksCodec, err = goavro.NewCodec(booksSchema); err != nil {
		return err
	} else if accountCodec, err = goavro.NewCodec(accountSchema); err != nil {
		return err
	} else if blockFeeCodec, err = goavro.NewCodec(blockfeeSchema); err != nil {
		return err
	}
	return nil
}
