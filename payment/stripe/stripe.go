package stripe

import "github.com/LunaNode/lobster"

import "github.com/stripe/stripe-go"
import "github.com/stripe/stripe-go/client"

import "fmt"
import "net/http"
import "strconv"

type StripeTemplateParams struct {
	Frame    lobster.FrameParams
	Token    string
	Key      string
	Cents    int64
	Currency string
	Amount   float64
	Email    string
}

type StripePayment struct {
	client         *client.API
	publishableKey string
}

func MakeStripePayment(privateKey string, publishableKey string) *StripePayment {
	sp := &StripePayment{
		client:         &client.API{},
		publishableKey: publishableKey,
	}
	sp.client.Init(privateKey, nil)
	lobster.RegisterPanelHandler("/payment/stripe/form", sp.form, false)
	lobster.RegisterPanelHandler("/payment/stripe/submit", sp.handle, true)
	return sp
}

func (sp *StripePayment) Payment(w http.ResponseWriter, r *http.Request, frameParams lobster.FrameParams, userId int, username string, amount float64) {
	cents := int64(amount * 100)
	http.Redirect(w, r, fmt.Sprintf("/payment/stripe/form?cents=%d", cents), 303)
}

func (sp *StripePayment) form(w http.ResponseWriter, r *http.Request, session *lobster.Session, frameParams lobster.FrameParams) {
	cents, _ := strconv.ParseInt(r.URL.Query().Get("cents"), 10, 64)
	user := lobster.UserDetails(session.UserId)
	cfg := lobster.GetConfig()
	params := &StripeTemplateParams{
		Frame:    frameParams,
		Token:    lobster.CSRFGenerate(session),
		Key:      sp.publishableKey,
		Cents:    cents,
		Currency: cfg.Billing.Currency,
		Amount:   float64(cents) / 100,
		Email:    user.Email,
	}
	lobster.RenderTemplate(w, "panel", "stripe", params)
}

type StripeErrorEmail struct {
	Description string
	Error       error
}

func (sp *StripePayment) handle(w http.ResponseWriter, r *http.Request, session *lobster.Session, frameParams lobster.FrameParams) {
	if !lobster.AntifloodCheck(lobster.ExtractIP(r.RemoteAddr), "payment_stripe_handle", 5) {
		lobster.RedirectMessage(w, r, "/panel/billing", lobster.L.FormattedError("try_again_later"))
		return
	}
	lobster.AntifloodAction(lobster.ExtractIP(r.RemoteAddr), "payment_stripe_handle")

	stripeToken := r.PostFormValue("stripeToken")
	amount, amountErr := strconv.Atoi(r.PostFormValue("amount"))
	currency := r.PostFormValue("currency")

	if stripeToken == "" || amount <= 0 || amountErr != nil || currency == "" {
		lobster.RedirectMessage(w, r, "/panel/billing", lobster.L.FormatError(fmt.Errorf("credit card payment failed due to form submission error")))
		return
	}

	// duplicate amount range check here since user might tamper with the amount in the form
	cfg := lobster.GetConfig()
	if amount < int(cfg.Billing.DepositMinimum*100) || amount > int(cfg.Billing.DepositMaximum*100) {
		lobster.RedirectMessage(w, r, "/panel/billing", lobster.L.FormattedErrorf("amount_between", cfg.Billing.DepositMinimum, cfg.Billing.DepositMaximum))
		return
	}

	chargeParams := &stripe.ChargeParams{
		Amount:   uint64(amount),
		Currency: stripe.Currency(currency),
		Source:   &stripe.SourceParams{Token: stripeToken},
		Desc:     "Lobster credit",
	}
	charge, err := sp.client.Charges.New(chargeParams)

	if err != nil {
		emailParams := StripeErrorEmail{
			fmt.Sprintf("error creating charge for user %d", session.UserId),
			err,
		}
		lobster.MailWrap(-1, "stripeError", emailParams, false)
		lobster.RedirectMessage(w, r, "/panel/billing", lobster.L.FormatError(fmt.Errorf("credit card payment error; make sure that you have entered your credit card details correctly")))
		return
	} else if !charge.Paid {
		emailParams := StripeErrorEmail{
			fmt.Sprintf("created charge for user %d but paid is false", session.UserId),
			nil,
		}
		lobster.MailWrap(-1, "stripeError", emailParams, false)
		lobster.RedirectMessage(w, r, "/panel/billing", lobster.L.FormatError(fmt.Errorf("credit card payment error; make sure that you have entered your credit card details correctly")))
		return
	}

	transaction := charge.Tx
	lobster.TransactionAdd(
		session.UserId,
		"stripe",
		charge.ID,
		"Stripe payment: "+charge.ID,
		int64(charge.Amount)*lobster.BILLING_PRECISION/100,
		transaction.Fee*lobster.BILLING_PRECISION/100,
	)
	lobster.RedirectMessage(w, r, "/panel/billing", lobster.L.Success("payment_made"))
}
