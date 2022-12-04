package kline

type Listener struct {
	errcount    uint8
	identifiers []uint8
	callback    chan KLineMsg
}
