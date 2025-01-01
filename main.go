package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
	"github.com/rivo/tview"
	"golang.org/x/exp/maps"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"pmmbot/database"
	"pmmbot/ds"
	"pmmbot/exchanges/gateio"
	"pmmbot/exchanges/msamex"
)

type PanelSetting struct {
	Market   string
	Currency string
	Exchange string
}

var exchange msamex.Msamex
var gateio_exchange gateio.Gateio
var panel_setting PanelSetting

var quit_panel_chan chan bool
var quit_gateio_chan chan bool

func main() {
	var err error
	database.DB, err = gorm.Open(sqlite.Open("pmm.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect to database")
	}

	database.DB.AutoMigrate(&database.Order{})
	database.DB.AutoMigrate(&database.Best{})
	database.DB.AutoMigrate(&database.Last{})

	logfile, _ := os.OpenFile("pmm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	defer logfile.Close()

	panel_flag := true
	tui_init()
	defer tui_close()

	log.SetDefault(log.NewWithOptions(io.MultiWriter(logfile, tview.ANSIWriter(tui_logs_view)), log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		TimeFormat:      time.RFC3339,
		Prefix:          "PMM",
	}))
	log.SetColorProfile(termenv.ANSI256)

	log.Info("Starting pure-market-maker ...")

	gateio_exchange = gateio.Gateio{
		Api_key:    "2ad23d5052420a0554dc8bd578ab9809",
		Api_secret: "3e03a8b6b878649ae006b2023de56a5fe31023f4985f64a112fa96cb4c994e7e",
		Pairs:      []string{"ETH_USDT"},
		Pask:       0.16,
		Pbid:       0.16,
	}

	exchange = msamex.Msamex{
		AccessKey: "dd05a63dbf4fe8eb",
		SecretKey: "539b1493a6054c31a26b17e4e5750128",
	}

	// b := exchange.Msamex_cancel_order(281959300)
	// log.Info(b)
	// go cancel_orders("ethusdt")

	// return
	// msamex.Msamex_open_orders("btcusdt")
	// _, err = exchange.Msamex_order("ethusdt", "sell", 0.01, 4_500)
	// if err != nil {
	// 	log.Error(err.Error())
	// }

	sigs := make(chan os.Signal, 1)
	// signal.Notify(sigs, syscall.SIGINT)

	go func() {
		for {
			_ = <-sigs
			log.Info("signal")
			if panel_flag {
				panel_flag = false
				tui_close()
			} else {
				panel_flag = true
				tui_run()
			}
		}
	}()

	panel_setting = PanelSetting{
		Market:   "ethusdt",
		Currency: "eth",
		Exchange: "gateio",
	}

	quit_panel_chan = make(chan bool)
	quit_gateio_chan = make(chan bool)

	go func() {
		for {
			do_last_order()
			time.Sleep(5 * time.Second)
		}
	}()

	go func() {
		for {
			select {
			case <-quit_panel_chan:
				return
			default:
				show_panel(panel_setting)
				time.Sleep(2 * time.Second)
			}
		}
	}()

	gateio_handler()

	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
}

func show_panel(setting PanelSetting) {
	text := ""
	text += "[yellow]EXCHANGE[-]: msamex\n"
	text += "[yellow]MARKET[-]: " + setting.Market + "\n"
	balance, err := exchange.Msamex_balance(setting.Currency)
	balance_str := "[red]unknown[-]"
	if err == nil {
		balance_str = fmt.Sprintf("%.9f", balance)
	}
	text += "[yellow]BALANCE[-]: " + balance_str + "\n\n"
	text += "[yellow]CURRENT ORDERS[-]:\n"
	orders, err := exchange.Msamex_open_orders(setting.Market)
	var orders_text strings.Builder
	if err == nil {
		writer := tabwriter.NewWriter(&orders_text, 0, 0, 2, ' ', 0)
		fmt.Fprintln(writer, "\tUID\tSIDE\tPRICE\tVOLUME\tREMAINING\tDATE")
		for _, order := range orders {
			fmt.Fprintf(writer, "\t%d\t%s\t%s\t%s\t%s\t%s\n", int(order["id"].(float64)), order["side"].(string),
				order["price"].(string),
				order["origin_volume"].(string), order["remaining_volume"].(string),
				order["created_at"].(string))
		}
		writer.Flush()
	}
	text += orders_text.String() + "\n\n"

	// bests := []database.Best{}
	// query := `
	// 		WITH RankedRows AS (
	// 			SELECT
	// 				id, created_at, updated_at, deleted_at, exchange, market, side, volume, price,
	// 				ROW_NUMBER() OVER (PARTITION BY side ORDER BY price DESC) AS rank
	// 			FROM bests
	// 		)
	// 		SELECT * FROM RankedRows WHERE rank <= 3 AND exchange = ? AND market = ?;
	// `

	// err = database.DB.Raw(query, setting.Exchange, setting.Market).Scan(&bests).Error

	// text += "[yellow]EXCHANGE[-]: " + setting.Exchange + "\n"
	// text += "[yellow]BEST ORDERS[-]:\n"
	// var best_orders_text strings.Builder
	// if err == nil {
	// 	writer := tabwriter.NewWriter(&best_orders_text, 0, 0, 2, ' ', 1)
	// 	fmt.Fprintln(writer, "\tSIDE\tRPICE\tVOLUME")
	// 	for _, order := range bests {
	// 		fmt.Fprintf(writer, "\t%s\t%f\t%f\n", order.Side, order.Price, order.Volume)
	// 	}
	// 	writer.Flush()
	// }
	// text += best_orders_text.String() + "\n"

	lasts := []database.Last{}
	query := `
	WITH RankedRows AS ( SELECT
	    id,
	    created_at,
	    updated_at,
	    deleted_at,
	    exchange,
	    market,
	    side,
	    volume,
	    price,
		ROW_NUMBER() OVER (
			PARTITION BY side
			ORDER BY
			CASE WHEN side = 'buy' THEN -price
			ELSE price
	        END ASC
		) AS rank FROM lasts
        WHERE exchange = ? AND market = ?
        )
	    SELECT *
	    FROM RankedRows
	    WHERE rank <= 10;`

	err = database.DB.Raw(query, setting.Exchange, setting.Market).Scan(&lasts).Error

	text += "[yellow]LAST ORDERS[-]:\n"
	var last_orders_text strings.Builder
	if err == nil {
		writer := tabwriter.NewWriter(&last_orders_text, 0, 0, 2, ' ', 1)
		fmt.Fprintln(writer, "\tSIDE\tRPICE\tVOLUME")
		for _, order := range lasts {
			fmt.Fprintf(writer, "\t%s\t%f\t%f\n", order.Side, order.Price, order.Volume)
		}
		writer.Flush()
	}
	text += last_orders_text.String()

	tui_write_panel(text)
}

func input_handler(input string) {
	splitted := strings.Split(input, " ")
	if strings.ToLower(splitted[0]) == "market" {
		if len(splitted) < 3 {
			log.Error("Invalid argument")
			return
		}
		quit_panel_chan <- true
		time.Sleep(time.Second)
		go func() {
			for {
				select {
				case <-quit_panel_chan:
					return
				default:
					panel_setting.Market = splitted[1]
					panel_setting.Currency = splitted[2]

					show_panel(panel_setting)
				}
			}
		}()
	} else if strings.ToLower(splitted[0]) == "exchange" {
		if len(splitted) < 2 {
			log.Error("Invalid argument")
			return
		}
		quit_panel_chan <- true
		time.Sleep(time.Second)
		go func() {
			for {
				select {
				case <-quit_panel_chan:
					return
				default:
					panel_setting.Exchange = splitted[1]

					show_panel(panel_setting)
				}
			}
		}()
	} else if strings.ToLower(splitted[0]) == "gateio" {
		if len(splitted) < 3 {
			log.Error("Invalid argument")
			return
		}

		quit_gateio_chan <- true

		if splitted[1] == "add" {
			gateio_exchange.Pairs = append(gateio_exchange.Pairs, splitted[2])
		}

		time.Sleep(time.Second)

		gateio_handler()
	} else if strings.ToLower(splitted[0]) == "order" {
		if len(splitted) < 4 {
			log.Error("Invalid argument")
			return
		}

		price, err := strconv.ParseFloat(splitted[3], 64)
		if err != nil {
			log.Error("Invalid argument", "error", err.Error())
		}

		volume, err := strconv.ParseFloat(splitted[4], 64)
		if err != nil {
			log.Error("Invalid argument", "error", err.Error())
			return
		}

		_, err = exchange.Msamex_order(splitted[1], splitted[2], volume, price)
		if err != nil {
			log.Error("Creating order failed", "error", err.Error())
			return
		}
	}
}

func cancel_orders(market string) {
	log.Info("Canceling old orders")
	var orders []database.Order
	err := database.DB.Where("market = ?", market).Find(&orders).Error
	if err != nil {
		log.Error(err.Error())
	}

	log.Info(orders)
	for _, order := range orders {
		ok := exchange.Msamex_cancel_order(int64(order.MsamexID))
		if !ok {
			log.Error("Canceling failed", "order", order.MsamexID)
		}
		database.DB.Where("market = ? AND msamex_id = ?", market, order.MsamexID).Unscoped().Delete(&database.Order{})
	}
}

func check_opp_order(orders []map[string]interface{}, order map[string]interface{}) bool {
	for _, t_order := range orders {
		if t_order["remaining_volume"].(string) == order["remaining_volume"].(string) &&
			t_order["side"].(string) != order["side"].(string) {
			return true
		}
	}
	return false
}

func do_last_order() {
	for _, market := range gateio_exchange.Pairs {
		market = strings.ToLower(strings.ReplaceAll(market, "_", ""))
		orders, err := exchange.Msamex_open_orders(market)
		if err != nil {
			log.Error("Error while doing the order", "market", market, "error", err.Error())
			continue
		}

		if len(orders) < 1 {
			continue
		}

		for _, order := range orders {
			if check_opp_order(orders, order) {
				return
			}
			target_side := "sell"
			if order["side"] == "sell" {
				target_side = "buy"
			}

			volume, err := strconv.ParseFloat(order["remaining_volume"].(string), 64)
			price, err := strconv.ParseFloat(order["price"].(string), 64)

			_, err = exchange.Msamex_order(market, target_side, volume, price)
			if err != nil {
				log.Error("Error while doing the order", "market", market, "id", int(order["id"].(float64)), "volume", volume, "price", price)
				continue
			}
			break
		}
	}
}

func gateio_handler() {
	// go func() {
	// 	data_pair := make(map[string]ds.ChanData, len(gateio_exchange.Pairs))
	// 	for x := range gateio_exchange.Best_Order_handler() {
	// 		select {
	// 		case <-quit_gateio_chan:
	// 			return
	// 		default:
	// 			data_pair[x.Pair] = x.Data
	// 			if len(data_pair) == len(gateio_exchange.Pairs) {
	// 				database.DB.Where("exchange = ?", "gateio").Unscoped().Delete(&database.Best{})
	// 				for k, v := range data_pair {
	// 					for _, o := range v.Bids {
	// 						database.DB.Create(&database.Best{
	// 							Exchange: "gateio",
	// 							Market:   strings.ToLower(strings.ReplaceAll(k, "_", "")),
	// 							Side:     "buy",
	// 							Volume:   o[1],
	// 							Price:    o[0],
	// 						})
	// 					}
	// 					for _, o := range v.Asks {
	// 						database.DB.Create(&database.Best{
	// 							Exchange: "gateio",
	// 							Market:   strings.ToLower(strings.ReplaceAll(k, "_", "")),
	// 							Side:     "sell",
	// 							Volume:   o[1],
	// 							Price:    o[0],
	// 						})
	// 					}
	// 				}
	// 				maps.Clear(data_pair)
	// 				time.Sleep(15 * time.Second)
	// 			}
	// 		}
	// 	}
	// }()

	go func() {
		data_pair := make(map[string]ds.ChanData, len(gateio_exchange.Pairs))
		for x := range gateio_exchange.Last_Order_handler() {
			select {
			case <-quit_gateio_chan:
				return
			default:
				data_pair[x.Pair] = x.Data
				if len(data_pair) == len(gateio_exchange.Pairs) {
					database.DB.Where("exchange = ?", "gateio").Unscoped().Delete(&database.Last{})
					for k, v := range data_pair {
						market := strings.ToLower(strings.ReplaceAll(k, "_", ""))
						go cancel_orders(market)

						max_length := int(math.Max(float64(len(v.Bids)), float64(len(v.Asks))))

						for i := 0; i < max_length; i++ {
							if i < len(v.Bids) {
								o := v.Bids[i]
								log.Info("Placing new order", "market", market, "side", "sell", "volume", o[1], "price", o[0])
								new_order, err := exchange.Msamex_order(market, "sell", o[1], o[0])
								if err == nil {
									database.DB.Create(&database.Order{
										Exchange:   "gateio",
										Market:     market,
										Side:       "sell",
										Volume:     o[1],
										Price:      o[0],
										MsamexID:   new_order.Uid,
										MsamexUUID: new_order.Uuid,
									})
								} else {
									log.Error(err.Error())
								}

								database.DB.Create(&database.Last{
									Exchange: "gateio",
									Market:   market,
									Side:     "sell",
									Volume:   o[1],
									Price:    o[0],
								})
							}

							if i < len(v.Asks) {
								o := v.Asks[i]
								log.Info("Placing new order", "market", market, "side", "buy", "volume", o[1], "price", o[0])
								new_order, err := exchange.Msamex_order(market, "buy", o[1], o[0])
								if err == nil {
									database.DB.Create(&database.Order{
										Exchange:   "gateio",
										Market:     market,
										Side:       "buy",
										Volume:     o[1],
										Price:      o[0],
										MsamexID:   new_order.Uid,
										MsamexUUID: new_order.Uuid,
									})
								} else {
									log.Error(err.Error())
								}

								database.DB.Create(&database.Last{
									Exchange: "gateio",
									Market:   market,
									Side:     "buy",
									Volume:   o[1],
									Price:    o[0],
								})
							}
						}
					}
					maps.Clear(data_pair)
					time.Sleep(15 * time.Second)
				}
			}
		}
	}()
}
