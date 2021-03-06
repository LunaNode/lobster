package lobster

import "fmt"
import "log"
import "time"

type Transaction struct {
	Id                int
	UserId            int
	Gateway           string
	GatewayIdentifier string
	Notes             string
	Amount            int64
	Fee               int64
	Time              time.Time
}

func transactionListHelper(rows Rows) []*Transaction {
	transactions := make([]*Transaction, 0)
	defer rows.Close()
	for rows.Next() {
		transaction := Transaction{}
		rows.Scan(&transaction.Id, &transaction.UserId, &transaction.Gateway, &transaction.GatewayIdentifier, &transaction.Notes, &transaction.Amount, &transaction.Fee, &transaction.Time)
		transactions = append(transactions, &transaction)
	}
	return transactions
}
func TransactionList() []*Transaction {
	return transactionListHelper(
		db.Query(
			"SELECT id, user_id, gateway, gateway_identifier, notes, amount, fee, time " +
				"FROM transactions ORDER BY id",
		),
	)
}
func TransactionGet(transactionId int) *Transaction {
	transactions := transactionListHelper(
		db.Query(
			"SELECT id, user_id, gateway, gateway_identifier, notes, amount, fee, time "+
				"FROM transactions WHERE id = ?",
			transactionId,
		),
	)
	if len(transactions) == 1 {
		return transactions[0]
	} else {
		return nil
	}
}
func TransactionGetByGateway(gateway string, gatewayIdentifier string) *Transaction {
	transactions := transactionListHelper(
		db.Query(
			"SELECT id, user_id, gateway, gateway_identifier, notes, amount, fee, time "+
				"FROM transactions "+
				"WHERE gateway = ? AND gateway_identifier = ?",
			gateway, gatewayIdentifier,
		),
	)
	if len(transactions) >= 1 {
		return transactions[0]
	} else {
		return nil
	}
}

func TransactionAdd(userId int, gateway string, gatewayIdentifier string, notes string, amount int64, fee int64) {
	// verify not duplicate
	if TransactionGetByGateway(gateway, gatewayIdentifier) != nil {
		log.Printf("Duplicate transaction %s/%s (amount=%d)", gateway, gatewayIdentifier, amount)
		return
	}

	// verify amount
	depositMinimum := int64(cfg.Billing.DepositMinimum * BILLING_PRECISION)
	depositMaximum := int64(cfg.Billing.DepositMaximum * BILLING_PRECISION)
	if amount < depositMinimum || amount > depositMaximum {
		ReportError(
			fmt.Errorf("invalid payment of %d cents", amount*100/BILLING_PRECISION),
			"transaction add error",
			fmt.Sprintf("user: %d, gw: %s; gwid: %s", userId, gateway, gatewayIdentifier),
		)
		return
	}

	// verify user
	user := UserDetails(userId)
	if user == nil {
		ReportError(
			fmt.Errorf("invalid user %d", userId),
			"transaction add error",
			fmt.Sprintf("user: %d, gw: %s; gwid: %s", userId, gateway, gatewayIdentifier),
		)
		return
	}

	transaction := Transaction{
		UserId:            userId,
		Gateway:           gateway,
		GatewayIdentifier: gatewayIdentifier,
		Notes:             notes,
		Amount:            amount,
		Fee:               fee,
		Time:              time.Now(),
	}
	db.Exec(
		"INSERT INTO transactions (user_id, gateway, gateway_identifier, notes, amount, fee) "+
			"VALUES (?, ?, ?, ?, ?, ?)",
		transaction.UserId, transaction.Gateway, transaction.GatewayIdentifier,
		transaction.Notes, transaction.Amount, transaction.Fee,
	)
	UserApplyCredit(userId, amount, fmt.Sprintf("Transaction %s/%s", gateway, gatewayIdentifier))
	MailWrap(userId, "paymentProcessed", PaymentProcessedEmail(&transaction), true)
	log.Printf("Processed payment of %d for user %d (%s/%s)", amount, userId, gateway, gatewayIdentifier)

}
