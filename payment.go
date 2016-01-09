package lobster

import "net/http"

type PaymentInterface interface {
	Payment(w http.ResponseWriter, r *http.Request, frameParams FrameParams, userId int, username string, amount float64)
}

var paymentInterfaces map[string]PaymentInterface = make(map[string]PaymentInterface)

func paymentMethodList() []string {
	var methods []string
	for method := range paymentInterfaces {
		methods = append(methods, method)
	}
	return methods
}

func paymentHandle(method string, w http.ResponseWriter, r *http.Request, frameParams FrameParams, userId int, username string, amount float64) {
	if amount < cfg.Billing.DepositMinimum || amount > cfg.Billing.DepositMaximum {
		RedirectMessage(w, r, "/panel/billing", L.FormattedErrorf("amount_between", cfg.Billing.DepositMinimum, cfg.Billing.DepositMaximum))
		return
	}

	payInterface, ok := paymentInterfaces[method]
	if ok {
		payInterface.Payment(w, r, frameParams, userId, username, amount)
	} else {
		RedirectMessage(w, r, "/panel/billing", L.FormattedError("invalid_payment_method"))
	}
}
