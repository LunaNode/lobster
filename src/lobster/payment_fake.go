package lobster

import "net/http"

type FakePayment struct {}

func (this *FakePayment) Payment(w http.ResponseWriter, r *http.Request, db *Database, frameParams FrameParams, userId int, username string, amount float64) {
	transactionAdd(db, userId, "fake", uid(16), "Fake credit", int64(amount * 100) * BILLING_PRECISION / 100, 0)
	redirectMessage(w, r, "/panel/billing", "Credit added successfully.")
}
