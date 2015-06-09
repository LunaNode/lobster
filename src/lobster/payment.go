package lobster

import "net/http"

type PaymentInterface interface {
	Payment(w http.ResponseWriter, r *http.Request, frameParams FrameParams, userId int, username string, amount float64)
}

var paymentInterfaces map[string]PaymentInterface = make(map[string]PaymentInterface)

func paymentMethodList() []string {
	var methods []string
	for method, _ := range paymentInterfaces {
		methods = append(methods, method)
	}
	return methods
}

func paymentHandle(method string, w http.ResponseWriter, r *http.Request, frameParams FrameParams, userId int, username string, amount float64) {
	payInterface, ok := paymentInterfaces[method]
	if ok {
		payInterface.Payment(w, r, frameParams, userId, username, amount)
	} else {
		redirectMessage(w, r, "/panel/billing", "Error: invalid payment method specified.")
	}
}
