package fake

import "github.com/LunaNode/lobster"
import "github.com/LunaNode/lobster/utils"

import "net/http"

type FakePayment struct{}

func (this *FakePayment) Payment(w http.ResponseWriter, r *http.Request, db *lobster.Database, frameParams lobster.FrameParams, userId int, username string, amount float64) {
	lobster.TransactionAdd(db, userId, "fake", utils.Uid(16), "Fake credit", int64(amount*100)*lobster.BILLING_PRECISION/100, 0)
	lobster.RedirectMessage(w, r, "/panel/billing", lobster.LA("payment_fake").Success("credit_added"))
}
