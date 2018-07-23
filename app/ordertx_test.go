package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	abci "github.com/tendermint/tendermint/abci/types"

	o "github.com/BiJie/BinanceChain/plugins/dex/order"
)

func Test_handleNewOrder_CheckTx(t *testing.T) {
	assert := assert.New(t)
	ctx := testApp.NewContext(true, abci.Header{})
	InitAccounts(ctx, testApp)
	add := Account(0).GetAddress()
	msg := o.NewNewOrderMsg(add, "order1", 1, "BTC_BNB", 355e8, 100e8)
	res, e := testClient.CheckTxSync(msg, testApp.Codec)
	assert.NotEqual(uint32(0), res.Code)
	assert.Nil(e)
	assert.Regexp(".*do not have enough token to lock.*", res.GetLog())
	assert.Equal(int64(500e8), GetAvail(ctx, add, "BNB"))
	assert.Equal(int64(0), GetLocked(ctx, add, "BNB"))
	assert.Equal(int64(200e8), GetAvail(ctx, add, "BTC"))
	assert.Equal(int64(0), GetLocked(ctx, add, "BTC"))

	msg = o.NewNewOrderMsg(add, "order1.2", 1, "BTC_BNB", 355e8, 1e8)
	res, e = testClient.CheckTxSync(msg, testApp.Codec)
	assert.Equal(uint32(0), res.Code)
	assert.Nil(e)
	assert.Equal(int64(145e8), GetAvail(ctx, add, "BNB"))
	assert.Equal(int64(355e8), GetLocked(ctx, add, "BNB"))
	assert.Equal(int64(200e8), GetAvail(ctx, add, "BTC"))
	assert.Equal(int64(0), GetLocked(ctx, add, "BTC"))

	add = Account(1).GetAddress()
	msg = o.NewNewOrderMsg(add, "order2.1", 2, "BTC_BNB", 355e8, 250e8)
	res, e = testClient.CheckTxSync(msg, testApp.Codec)
	assert.NotEqual(uint32(0), res.Code)
	assert.Nil(e)
	assert.Regexp(".*do not have enough token to lock.*", res.GetLog())
	assert.Equal(int64(500e8), GetAvail(ctx, add, "BNB"))
	assert.Equal(int64(0), GetLocked(ctx, add, "BNB"))
	assert.Equal(int64(200e8), GetAvail(ctx, add, "BTC"))
	assert.Equal(int64(0), GetLocked(ctx, add, "BTC"))

	msg = o.NewNewOrderMsg(add, "order2.2", 2, "BTC_BNB", 355e8, 200e8)
	res, e = testClient.CheckTxSync(msg, testApp.Codec)
	assert.Equal(uint32(0), res.Code)
	assert.Nil(e)
	assert.Equal(int64(500e8), GetAvail(ctx, add, "BNB"))
	assert.Equal(int64(0), GetLocked(ctx, add, "BNB"))
	assert.Equal(int64(0), GetAvail(ctx, add, "BTC"))
	assert.Equal(int64(200e8), GetLocked(ctx, add, "BTC"))
}

type level struct {
	price int64
	qty   int64
}

func getOrderBook(pair string) ([]level, []level) {
	buys := make([]level, 0)
	sells := make([]level, 0)
	testApp.OrderKeeper.GetOrderBookUnSafe(pair, 5,
		func(price, qty int64) {
			buys = append(buys, level{price, qty})
		},
		func(price, qty int64) {
			sells = append(sells, level{price, qty})
		})
	return buys, sells
}

func Test_handleNewOrder_DeliverTx(t *testing.T) {
	assert := assert.New(t)
	testClient.cl.BeginBlockSync(abci.RequestBeginBlock{})
	ctx := testApp.NewContext(false, abci.Header{})
	InitAccounts(ctx, testApp)
	testApp.OrderKeeper.ClearOrderBook("BTC_BNB")
	add := Account(0).GetAddress()
	msg := o.NewNewOrderMsg(add, "order1.2", 1, "BTC_BNB", 355e8, 1e8)

	res, e := testClient.DeliverTxSync(msg, testApp.Codec)
	t.Logf("res is %v and error is %v", res, e)
	assert.Equal(uint32(0), res.Code)
	assert.Nil(e)
	buys, sells := getOrderBook("BTC_BNB")
	assert.Equal(1, len(buys))
	assert.Equal(0, len(sells))
	assert.Equal(int64(355e8), buys[0].price)
	assert.Equal(int64(1e8), buys[0].qty)
}