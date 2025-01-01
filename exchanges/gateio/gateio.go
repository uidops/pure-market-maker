package gateio

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"pmmbot/ds"

	"github.com/charmbracelet/log"
	"github.com/gorilla/websocket"
)

type Gateio struct {
	Api_key    string
	Api_secret string
	Pairs      []string
	Pask       float64
	Pbid       float64
}

func (gateio Gateio) Gen_sign(channel string, event string, timestamp int64) map[string]interface{} {
	s := fmt.Sprintf("channel=%s&event=%s&time=%d", channel, event, timestamp)
	mac := hmac.New(sha512.New, []byte(gateio.Api_secret))
	mac.Write([]byte(s))
	return map[string]interface{}{
		"method": "api_key",
		"KEY":    gateio.Api_key,
		"SIGN":   hex.EncodeToString(mac.Sum(nil)),
	}
}

func (gateio Gateio) Best_Order_handler() chan ds.ChanType {
	u := url.URL{Scheme: "wss", Host: "api.gateio.ws", Path: "/ws/v4/"} /* wss://api.gateio.ws/ws/v4/ */
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)

	if err != nil {
		log.Fatal("dial: ", err)
		return nil
	}

	log.Info("Subscribing to spot.book_ticker in gateio exchange")

	ws.WriteJSON(map[string]interface{}{
		"time":    time.Now().Unix(),
		"channel": "spot.book_ticker", // the channel doesn't need authentication
		"event":   "subscribe",
		"payload": gateio.Pairs,
	})

	ticker := time.NewTicker(1 * time.Second)
	ch := make(chan ds.ChanType)

	go func() {
		cond := true
		defer ws.Close()
		defer ws.WriteJSON(map[string]interface{}{
			"time":    time.Now().Unix(),
			"channel": "spot.book_ticker",
			"event":   "unsubscribe",
			"payload": gateio.Pairs,
		})

		defer func() {
			if x := recover(); x != nil {
				cond = false
			}
		}()

		for cond {
			select {
			case _ = <-ticker.C:
				var resp_data map[string]interface{}
				ws.ReadJSON(&resp_data)

				event, ok := resp_data["event"].(string)
				if ok == false || event != "update" {
					continue
				}

				result := resp_data["result"].(map[string]interface{})

				ask_volume, _ := strconv.ParseFloat(result["A"].(string), 64)
				ask_price, _ := strconv.ParseFloat(result["a"].(string), 64)

				bid_volume, _ := strconv.ParseFloat(result["B"].(string), 64)
				bid_price, _ := strconv.ParseFloat(result["b"].(string), 64)

				select {
				case ch <- ds.ChanType{
					Data: ds.ChanData{
						Asks: [][]float64{{ask_price, ask_volume}},
						Bids: [][]float64{{bid_price, bid_volume}},
					},
					Pair: result["s"].(string)}:
				default:
				}
			}
		}
	}()

	return ch
}

func (gateio Gateio) Last_Order_handler() chan ds.ChanType {
	u := url.URL{Scheme: "wss", Host: "api.gateio.ws", Path: "/ws/v4/"} /* wss://api.gateio.ws/ws/v4/ */
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)

	if err != nil {
		log.Fatal("dial: ", err)
		return nil
	}

	log.Info("Subscribing to spot.order_book in gateio exchange")
	for _, pair := range gateio.Pairs {
		ws.WriteJSON(map[string]interface{}{
			"time":    time.Now().Unix(),
			"channel": "spot.order_book", // the channel doesn't need authentication
			"event":   "subscribe",
			"payload": []string{pair, "10", "1000ms"},
		})
	}

	ticker := time.NewTicker(1 * time.Second)
	ch := make(chan ds.ChanType)

	go func() {
		cond := true
		defer ws.Close()
		for _, pair := range gateio.Pairs {
			defer ws.WriteJSON(map[string]interface{}{
				"time":    time.Now().Unix(),
				"channel": "spot.order_book",
				"event":   "unsubscribe",
				"payload": []string{pair, "5", "1000ms"},
			})
		}

		defer func() {
			if x := recover(); x != nil {
				cond = false
			}
		}()

		for cond {
			select {
			case _ = <-ticker.C:
				var resp_data map[string]interface{}
				ws.ReadJSON(&resp_data)

				event, ok := resp_data["event"].(string)
				if ok == false || event != "update" {
					continue
				}

				asks := make([][]float64, 0, 5)
				bids := make([][]float64, 0, 5)

				result := resp_data["result"].(map[string]interface{})

				for _, ask := range result["asks"].([]interface{}) {
					a, err := strconv.ParseFloat(ask.([]interface{})[0].(string), 64)
					if err != nil {
						continue
					}

					b, err := strconv.ParseFloat(ask.([]interface{})[1].(string), 64)
					if err != nil {
						continue
					}

					asks = append(asks, []float64{a, b})
				}

				for _, bid := range result["bids"].([]interface{}) {
					a, err := strconv.ParseFloat(bid.([]interface{})[0].(string), 64)
					if err != nil {
						continue
					}

					b, err := strconv.ParseFloat(bid.([]interface{})[1].(string), 64)
					if err != nil {
						continue
					}

					bids = append(bids, []float64{a, b})
				}

				select {
				case ch <- ds.ChanType{
					Data: ds.ChanData{
						Asks: asks,
						Bids: bids,
					},
					Pair: result["s"].(string)}:
				default:
				}
			}
		}
	}()

	return ch
}
