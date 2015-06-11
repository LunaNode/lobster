package lobster

import "bytes"
import "errors"
import "fmt"
import "io/ioutil"
import "net/http"
import "net/url"
import "strconv"
import "strings"

const PAYPAL_URL = "https://www.paypal.com/cgi-bin/webscr"
const PAYPAL_CALLBACK = "/paypal_notify"

type PaypalTemplateParams struct {
	Frame FrameParams
	Business string
	Amount float64
	UserId int
	NotifyUrl string
	ReturnUrl string
	Currency string
}

type PaypalPayment struct {
	business string
	returnUrl string
}

func MakePaypalPayment(lobster *Lobster, business string, returnUrl string) *PaypalPayment {
	this := new(PaypalPayment)
	this.business = business
	this.returnUrl = returnUrl
	lobster.RegisterHttpHandler(PAYPAL_CALLBACK, lobster.GetDatabase().wrapHandler(this.Callback), true)
	return this
}

func (this *PaypalPayment) Payment(w http.ResponseWriter, r *http.Request, db *Database, frameParams FrameParams, userId int, username string, amount float64) {
	frameParams.Scripts = append(frameParams.Scripts, "paypal")
	params := &PaypalTemplateParams{
		Frame: frameParams,
		Business: this.business,
		Amount: amount,
		UserId: userId,
		NotifyUrl: cfg.Default.UrlBase + PAYPAL_CALLBACK,
		ReturnUrl: this.returnUrl,
		Currency: cfg.Billing.Currency,
	}
	renderTemplate(w, "panel", "paypal", params)
}

func (this *PaypalPayment) Callback(w http.ResponseWriter, r *http.Request, db *Database) {
	requestBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		reportError(err, "paypal callback read error", fmt.Sprintf("ip: %s", r.RemoteAddr))
		splashNotFoundHandler(w, r)
		return
	}

	// decode the post data manually since there may be encoding issues
	requestParts := strings.Split(string(requestBytes), "&")
	myPost := make(map[string]string)
	for _, part := range requestParts {
		keyval := strings.Split(part, "=")
		if len(keyval) == 2 {
			myPost[keyval[0]], _ = url.QueryUnescape(keyval[1])
		}
	}

	// post back to Paypal system to validate the IPN data
	validateReq := "cmd=_notify-validate"
	for key, value := range myPost {
		validateReq += fmt.Sprintf("&%s=%s", key, url.QueryEscape(value))
	}

	resp, err := http.Post(PAYPAL_URL, "application/x-www-form-urlencoded", bytes.NewBufferString(validateReq))
	if err != nil {
		reportError(err, "paypal callback validation error", fmt.Sprintf("ip: %s; requestmap: %v", r.RemoteAddr, myPost))
		splashNotFoundHandler(w, r)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		reportError(err, "paypal callback validation error", fmt.Sprintf("ip: %s; requestmap: %v", r.RemoteAddr, myPost))
		splashNotFoundHandler(w, r)
		return
	}

	if string(body) != "VERIFIED" || myPost["payment_status"] == "" || myPost["mc_gross"] == "" || myPost["mc_currency"] == "" || myPost["txn_id"] == "" || myPost["receiver_email"] == "" || myPost["payment_status"] == "" || myPost["payer_email"] == "" || myPost["custom"] == "" {
		reportError(errors.New("missing field or not verified"), "paypal callback bad input or validation", fmt.Sprintf("ip: %s; verify body: %s; requestmap: %v", r.RemoteAddr, body, myPost))
		splashNotFoundHandler(w, r)
		return
	}

	w.WriteHeader(200)

	if myPost["payment_status"] != "Completed" {
		return
	} else if !strings.HasPrefix(myPost["custom"], "lobster") {
		reportError(errors.New(fmt.Sprintf("invalid payment with custom=%s", myPost["custom"])), "paypal callback error", fmt.Sprintf("ip: %s; requestmap: %v", r.RemoteAddr, myPost))
		return
	} else if strings.TrimSpace(strings.ToLower(myPost["receiver_email"])) != strings.TrimSpace(strings.ToLower(this.business)) {
		reportError(errors.New(fmt.Sprintf("invalid payment with receiver_email=%s", myPost["receiver_email"])), "paypal callback error", fmt.Sprintf("ip: %s; requestmap: %v", r.RemoteAddr, myPost))
		return
	} else if myPost["mc_currency"] != cfg.Billing.Currency {
		reportError(errors.New(fmt.Sprintf("invalid payment with currency=%s", myPost["mc_currency"])), "paypal callback error", fmt.Sprintf("ip: %s; requestmap: %v", r.RemoteAddr, myPost))
		return
	}

	paymentAmount, _ := strconv.ParseFloat(myPost["mc_gross"], 64)
	transactionId := myPost["txn_id"]
	userIdStr := strings.Split(myPost["custom"], "lobster")[1]
	userId, err := strconv.ParseInt(userIdStr, 10, 32)
	if err != nil {
		reportError(errors.New(fmt.Sprintf("invalid payment with custom=%s", myPost["custom"])), "paypal callback error", fmt.Sprintf("ip: %s; requestmap: %v", r.RemoteAddr, myPost))
		return
	}

	transactionAdd(db, int(userId), "paypal", transactionId, "Transaction " + transactionId, int64(paymentAmount * BILLING_PRECISION), 0)
}
