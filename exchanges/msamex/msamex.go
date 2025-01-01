package msamex

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
)

var msamex_api = "https://test.msamex.com/api/v2/peatio"

type Msamex struct {
	AccessKey string
	SecretKey string
}

type MsamexData struct {
	Market   string
	Side     string
	Volume   float64
	Ord_type string
	Price    float64
	Uuid     string
	Uid      int64
}

func (msamex Msamex) request(method string, endpoint string, body map[string]interface{}) (map[string]interface{}, []byte, error) {
	var (
		req *http.Request
		err error
	)

	client := &http.Client{Timeout: time.Minute}
	if body != nil {
		json_data, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}

		req, err = http.NewRequest(method, msamex_api+endpoint, bytes.NewBuffer(json_data))
		if err != nil {
			return nil, nil, err
		}

	} else {
		req, err = http.NewRequest(method, msamex_api+endpoint, nil)
		if err != nil {
			return nil, nil, err
		}
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; OpenBSD i386)")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Apikey", msamex.AccessKey)

	nonce := strconv.FormatInt(time.Now().UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(msamex.SecretKey))
	mac.Write([]byte(nonce + msamex.AccessKey))

	req.Header.Set("X-Auth-Nonce", nonce)
	req.Header.Set("X-Auth-Signature", hex.EncodeToString(mac.Sum(nil)))

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	body_bytes, _ := io.ReadAll(resp.Body)

	var resp_data map[string]interface{}
	if err = json.Unmarshal(body_bytes, &resp_data); err != nil {
		return nil, body_bytes, err
	}

	return resp_data, body_bytes, nil
}

func (msamex Msamex) Msamex_order(market string, side string, volume float64, price float64) (MsamexData, error) {
	response_json, _, err := msamex.request(http.MethodPost, "/market/orders", map[string]interface{}{
		"market":   market,
		"side":     side,
		"volume":   volume,
		"ord_type": "limit",
		"price":    price,
	})

	if err != nil {
		return MsamexData{}, err
	}

	errstr, ok := response_json["errors"].([]interface{})
	if ok == true {
		return MsamexData{}, errors.New(errstr[0].(string))
	}

	uuid, ok := response_json["uuid"].(string)
	if ok == false {
		return MsamexData{}, errors.New("There is no uuid")
	}

	uid, ok := response_json["id"].(float64)
	if ok == false {
		return MsamexData{}, errors.New("There is no id")
	}

	log.Info("New order", "market", market, "side", side, "volume", volume, "ord_type", "limit", "price", price, "uuid", uuid)

	return MsamexData{
		Market:   market,
		Side:     side,
		Volume:   volume,
		Ord_type: "limit",
		Price:    price,
		Uuid:     uuid,
		Uid:      int64(uid),
	}, nil
}

func (msamex Msamex) Msamex_cancel_order(id int64) bool {
	response_json, _, err := msamex.request(http.MethodPost, "/market/orders/"+strconv.FormatInt(id, 10)+"/cancel", nil)
	if err != nil {
		log.Error(err.Error())
		return false
	}

	errstr, ok := response_json["errors"].([]interface{})
	if ok == true {
		log.Error("Error while canceling the order", "error", errstr[0].(string), "id", id)
		return false
	}

	return true
}

func (msamex Msamex) Msamex_open_orders(market string) ([]map[string]interface{}, error) {
	var response_json []map[string]interface{}
	query := url.Values{}
	query.Add("market", market)
	query.Add("state", "wait")

	_, body_bytes, _ := msamex.request(http.MethodGet, "/market/orders/?"+query.Encode(), nil)

	if err := json.Unmarshal(body_bytes, &response_json); err != nil {
		return nil, err
	}

	return response_json, nil
}

func (msamex Msamex) Msamex_balance(currency string) (float64, error) {
	response_json, _, err := msamex.request(http.MethodGet, "/account/balances/"+currency, nil)
	balance, ok := response_json["balance"].(string)
	if !ok {
		return 0, errors.New("No balance")
	}

	balance_float, _ := strconv.ParseFloat(balance, 64)
	return balance_float, err
}
