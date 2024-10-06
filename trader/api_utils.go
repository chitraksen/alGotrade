package trader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

const (
	baseURL         = "https://api-fxpractice.oanda.com"
	pricingEndpoint = "/v3/accounts/{accountID}/pricing"
	orderEndpoint   = "/v3/accounts/{accountID}/orders"
)

type Credentials struct {
	AccountID   string `json:"accountID"`
	BearerToken string `json:"bearerToken"`
}

type RawPricingResponse struct {
	Time   string `json:"time"`
	Prices []struct {
		Instrument string `json:"instrument"`
		Tradeable  bool   `json:"tradeable"`
		Bids       []struct {
			Price float32 `json:"price,string"`
		} `json:"bids"`
		Asks []struct {
			Price float32 `json:"price,string"`
		} `json:"asks"`
	} `json:"prices"`
}

type PricingResponse struct {
	Time   string  `json:"time"`
	Prices []Price `json:"prices"`
}

type Price struct {
	Instrument string
	Tradeable  bool
	Bid        float32
	Ask        float32
}

type MarketOrderRequest struct {
	Order MarketOrder `json:"order"`
}

type MarketOrder struct {
	Units        string `json:"units"`
	Instrument   string `json:"instrument"`
	PriceBound   string `json:"priceBound"`
	TimeInForce  string `json:"timeInForce"`
	Type         string `json:"type"`
	PositionFill string `json:"positionFill"`
}

type OrderResponse struct {
	LastTransactionID      string                 `json:"lastTransactionID"`
	OrderCreateTransaction OrderCreateTransaction `json:"orderCreateTransaction"`
	OrderFillTransaction   OrderFillTransaction   `json:"orderFillTransaction"`
	RelatedTransactionIDs  []string               `json:"relatedTransactionIDs"`
}

type OrderCreateTransaction struct {
	AccountID    string `json:"accountID"`
	BatchID      string `json:"batchID"`
	ID           string `json:"id"`
	Instrument   string `json:"instrument"`
	PositionFill string `json:"positionFill"`
	Reason       string `json:"reason"`
	Time         string `json:"time"`
	TimeInForce  string `json:"timeInForce"`
	Type         string `json:"type"`
	Units        string `json:"units"`
	UserID       int    `json:"userID"`
}

type OrderFillTransaction struct {
	AccountBalance string      `json:"accountBalance"`
	AccountID      string      `json:"accountID"`
	BatchID        string      `json:"batchID"`
	Financing      string      `json:"financing"`
	ID             string      `json:"id"`
	Instrument     string      `json:"instrument"`
	OrderID        string      `json:"orderID"`
	Pl             string      `json:"pl"`
	Price          string      `json:"price"`
	Reason         string      `json:"reason"`
	Time           string      `json:"time"`
	TradeOpened    TradeOpened `json:"tradeOpened"`
	Type           string      `json:"type"`
	Units          string      `json:"units"`
	UserID         int         `json:"userID"`
}

type TradeOpened struct {
	TradeID string `json:"tradeID"`
	Units   string `json:"units"`
}

func getCreds() *Credentials {
	file, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	var creds Credentials
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&creds)
	if err != nil {
		log.Fatal((err))
	}

	return &creds
}

func parseRawResponse(rawResponse *RawPricingResponse) (*PricingResponse, error) {
	response := PricingResponse{
		Time:   rawResponse.Time,
		Prices: make([]Price, len(rawResponse.Prices)),
	}

	for i, rawPrice := range rawResponse.Prices {
		price := Price{
			Instrument: rawPrice.Instrument,
			Tradeable:  rawPrice.Tradeable,
		}

		if len(rawPrice.Bids) > 0 {
			price.Bid = rawPrice.Bids[0].Price
		} else {
			return nil, fmt.Errorf("No bid prices recieved.")
		}
		if len(rawPrice.Asks) > 0 {
			price.Ask = rawPrice.Asks[0].Price
		} else {
			return nil, fmt.Errorf("No ask prices recieved.")
		}

		response.Prices[i] = price
	}

	return &response, nil
}

func getPrices(instruments []string) (*PricingResponse, error) {
	creds := getCreds()
	url := strings.Replace(baseURL+pricingEndpoint, "{accountID}", creds.AccountID, 1)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+creds.BearerToken)
	q := req.URL.Query()
	q.Add("instruments", strings.Join(instruments, ","))
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		err := fmt.Errorf("%d response code received, Prices API request not working as expected", resp.StatusCode)
		return nil, err
	}

	var rawResponse RawPricingResponse
	err = json.Unmarshal(body, &rawResponse)
	if err != nil {
		return nil, err
	}
	response, err := parseRawResponse(&rawResponse)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func placeMarketOrder(units int, instrument string, priceBound float32) (*OrderResponse, error) {
	creds := getCreds()
	url := strings.Replace(baseURL+orderEndpoint, "{accountID}", creds.AccountID, 1)

	orderRequest := MarketOrderRequest{
		Order: MarketOrder{
			Units:        fmt.Sprintf("%d", units),
			Instrument:   instrument,
			PriceBound:   fmt.Sprintf("%.5f", priceBound),
			TimeInForce:  "FOK",
			Type:         "MARKET",
			PositionFill: "DEFAULT",
		},
	}

	jsonBody, err := json.Marshal(orderRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.BearerToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s",
			resp.StatusCode,
			string(body))
	}

	var orderResponse OrderResponse
	err = json.Unmarshal(body, &orderResponse)
	if err != nil {
		return nil, err
	}

	return &orderResponse, nil
}

func EntryPoint() {
	// Example usage of getPrices
	instruments := []string{"GBP_USD", "EUR_GBP", "GBP_JPY"}
	pricesResponse, err := getPrices(instruments)
	if err != nil {
		log.Fatalf("Error retrieving prices: %v", err)
	} else {
		fmt.Println("Prices retrieved successfully.")
		spew.Dump(pricesResponse)

	}

	// Example usage of placeMarketOrder
	if pricesResponse.Prices[0].Tradeable {
		orderResponse, err := placeMarketOrder(1, "GBP_USD", pricesResponse.Prices[0].Ask)
		if err != nil {
			log.Printf("Error placing market order: %v", err)
		} else {
			fmt.Println("Market order placed successfully.")
			spew.Dump(orderResponse)
		}
	} else {
		fmt.Println("Error executing market order! Instrument currently not tradeable.")
	}
}
