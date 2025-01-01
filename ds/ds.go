package ds

type Exchange interface {
	handle_orders()
}

type ChanData struct {
	Asks [][]float64
	Bids [][]float64
}

type ChanType struct {
	Data ChanData
	Pair string
}
