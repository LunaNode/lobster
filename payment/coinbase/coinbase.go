package coinbase

import "github.com/LunaNode/lobster"

import "github.com/fabioberger/coinbase-go"

import "encoding/json"
import "fmt"
import "io/ioutil"
import "log"
import "net/http"
import "strconv"
import "strings"

type CoinbasePayment struct {
	callbackSecret string
	apiKey         string
	apiSecret      string
}

func MakeCoinbasePayment(callbackSecret string, apiKey string, apiSecret string) *CoinbasePayment {
	this := new(CoinbasePayment)
	this.callbackSecret = callbackSecret
	this.apiKey = apiKey
	this.apiSecret = apiSecret
	lobster.RegisterHttpHandler("/coinbase_callback_"+this.callbackSecret, this.callback, true)
	return this
}

func (this *CoinbasePayment) Payment(w http.ResponseWriter, r *http.Request, frameParams lobster.FrameParams, userId int, username string, amount float64) {
	cfg := lobster.GetConfig()
	if cfg.Default.Debug {
		log.Printf("Creating Coinbase button for %s (id=%d) with amount $%.2f", username, userId, amount)
	}
	params := &coinbase.Button{
		Name:             lobster.L.T("credit_for_username", username),
		PriceString:      fmt.Sprintf("%.2f", amount),
		PriceCurrencyIso: cfg.Billing.Currency,
		Custom:           fmt.Sprintf("lobster%d", userId),
		Description:      fmt.Sprintf("Credit %s", lobster.L.T("currency_format", fmt.Sprintf("%.2f", amount))),
		Type:             "buy_now",
		Style:            "buy_now_large",
		CallbackUrl:      cfg.Default.UrlBase + "/coinbase_callback_" + this.callbackSecret,
	}
	cli := coinbase.ApiKeyClient(this.apiKey, this.apiSecret)
	button, err := cli.CreateButton(params)
	if err != nil {
		lobster.ReportError(err, "failed to create Coinbase button", fmt.Sprintf("username=%s, amount=%.2f", username, amount))
		lobster.RedirectMessage(w, r, "/panel/billing", lobster.L.FormattedError("try_again_later"))
		return
	}
	http.Redirect(w, r, "https://coinbase.com/checkouts/"+button.Code, 303)
}

type CoinbaseDataNative struct {
	Cents       float64 `json:"cents"`
	CurrencyIso string  `json:"currency_iso"`
}

type CoinbaseTransaction struct {
	Id string `json:"id"`
}

type CoinbaseDataOrder struct {
	Id          string               `json:"id"`
	Status      string               `json:"status"`
	TotalNative *CoinbaseDataNative  `json:"total_native"`
	Custom      string               `json:"custom"`
	Transaction *CoinbaseTransaction `json:"transaction"`
}

type CoinbaseData struct {
	Order *CoinbaseDataOrder `json:"order"`
}

type CoinbaseMispaidEmail struct {
	OrderId string
}

func (this *CoinbasePayment) callback(w http.ResponseWriter, r *http.Request) {
	cfg := lobster.GetConfig()

	requestBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		lobster.ReportError(err, "coinbase callback read error", fmt.Sprintf("ip: %s", r.RemoteAddr))
		w.WriteHeader(500)
		return
	}

	var data CoinbaseData
	err = json.Unmarshal(requestBytes, &data)
	if err != nil {
		lobster.ReportError(err, "coinbase callback decoding error", fmt.Sprintf("ip: %s; raw request: %s", r.RemoteAddr, requestBytes))
		w.WriteHeader(400)
		return
	}

	if data.Order.TotalNative.CurrencyIso != cfg.Billing.Currency {
		lobster.ReportError(fmt.Errorf("invalid currency %s", data.Order.TotalNative.CurrencyIso), "coinbase callback error", fmt.Sprintf("ip: %s; raw request: %s", r.RemoteAddr, requestBytes))
		w.WriteHeader(200)
		return
	} else if !strings.HasPrefix(data.Order.Custom, "lobster") {
		lobster.ReportError(fmt.Errorf("invalid payment with custom=%s", data.Order.Custom), "coinbase callback error", fmt.Sprintf("ip: %s; raw request: %s", r.RemoteAddr, requestBytes))
		w.WriteHeader(200)
		return
	}

	userIdStr := strings.Split(data.Order.Custom, "lobster")[1]
	userId, err := strconv.Atoi(userIdStr)
	if err != nil {
		lobster.ReportError(fmt.Errorf("invalid payment with custom=%s", data.Order.Custom), "coinbase callback error", fmt.Sprintf("ip: %s; raw request: %s", r.RemoteAddr, requestBytes))
		w.WriteHeader(200)
		return
	}

	if data.Order.Status == "completed" {
		lobster.TransactionAdd(userId, "coinbase", data.Order.Id, "Bitcoin transaction: "+data.Order.Transaction.Id, int64(data.Order.TotalNative.Cents)*lobster.BILLING_PRECISION/100, 0)
	} else if data.Order.Status == "mispaid" {
		lobster.MailWrap(-1, "coinbaseMispaid", CoinbaseMispaidEmail{OrderId: data.Order.Id}, false)
	}

	w.WriteHeader(200)
}
