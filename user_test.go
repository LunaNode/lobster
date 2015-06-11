package lobster

import "testing"
import "time"

func testForceUserBilling(db *Database, userId int) {
	db.Exec("UPDATE users SET last_billing_notify = DATE_SUB(NOW(), INTERVAL 25 HOUR) WHERE id = ?", userId)
	userBilling(db, userId)
}

func testVerifyChargeApprox(db *Database, userId int, k string, amountMin int64, amountMax int64) bool {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM charges WHERE user_id = ? AND k = ? AND amount >= ? AND amount <= ?", userId, k, amountMin, amountMax).Scan(&count)
	return count > 0
}

func testVerifyCharge(db *Database, userId int, k string, amount int64) bool {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM charges WHERE user_id = ? AND k = ? AND amount = ?", userId, k, amount).Scan(&count)
	return count > 0
}

func TestBillingBandwidth(t *testing.T) {
	db := TestReset()
	userId := TestUser(db)

	// basic overage billing
	var gbUsage int64 = 1000
	result := db.Exec("INSERT INTO region_bandwidth (user_id, region, bandwidth_used) VALUES (?, 'test', ?)", userId, gbUsage * 1024 * 1024 * 1024)
	regionBandwidthId, _ := result.LastInsertId()
	testForceUserBilling(db, userId)
	expectedCharge := int64(cfg.Default.BandwidthOverageFee * BILLING_PRECISION) * gbUsage
	if !testVerifyCharge(db, userId, "bw-test", expectedCharge) {
		t.Fatalf("Overage of %d GB, but didn't bill according to overage fee", gbUsage)
	}

	// make sure we can increase both used and allocated without charging again
	gigaIncrease := 500
	db.Exec("UPDATE region_bandwidth SET bandwidth_used = bandwidth_used + ?, bandwidth_additional = bandwidth_additional + ? WHERE id = ?", gigaToBytes(gigaIncrease), gigaToBytes(gigaIncrease), regionBandwidthId)
	testForceUserBilling(db, userId)
	if !testVerifyCharge(db, userId, "bw-test", expectedCharge) {
		t.Fatal("Billed when used/allocated increased by same amount")
	}

	// begin testing proportional billing
	// so we create a vm and make it provisioned at halfway of month, make sure charge correct amount
	vmId := TestVm(db, userId)
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)
	monthHalf := monthStart.Add(monthEnd.Sub(monthStart) / 2)
	db.Exec("UPDATE vms SET time_created = ? WHERE id = ?", monthHalf.Format(MYSQL_TIME_FORMAT), vmId)
	db.Exec("UPDATE region_bandwidth SET bandwidth_used = bandwidth_used + ? WHERE id = ?", gigaToBytes(TEST_BANDWIDTH / 2), regionBandwidthId)
	testForceUserBilling(db, userId)
	if !testVerifyChargeApprox(db, userId, "bw-test", expectedCharge * 9 / 10, expectedCharge * 11 / 10) {
		t.Fatal("User charged despite proportional virtual machine")
	}

	db.Exec("UPDATE region_bandwidth SET bandwidth_used = bandwidth_used + ? WHERE id = ?", gigaToBytes(TEST_BANDWIDTH / 2), regionBandwidthId)
	expectedCharge += int64(cfg.Default.BandwidthOverageFee * BILLING_PRECISION) * TEST_BANDWIDTH / 2
	testForceUserBilling(db, userId)
	if !testVerifyChargeApprox(db, userId, "bw-test", expectedCharge * 9 / 10, expectedCharge * 11 / 10) {
		t.Fatal("User charged differently than expected with proportional virtual machine")
	}

	// vm provisioned before beginning of the month should add this month's bandwidth only
	anotherVmId := TestVm(db, userId)
	lastMonth := monthStart.AddDate(0, -1, 0)
	db.Exec("UPDATE vms SET time_created = ? WHERE id = ?", lastMonth.Format(MYSQL_TIME_FORMAT), anotherVmId)
	db.Exec("UPDATE region_bandwidth SET bandwidth_used = bandwidth_used + ? WHERE id = ?", gigaToBytes(TEST_BANDWIDTH * 2), regionBandwidthId)
	expectedCharge += int64(cfg.Default.BandwidthOverageFee * BILLING_PRECISION) * TEST_BANDWIDTH
	testForceUserBilling(db, userId)
	if !testVerifyChargeApprox(db, userId, "bw-test", expectedCharge * 9 / 10, expectedCharge * 11 / 10) {
		t.Fatal("User charged differently than expected with long time ago virtual machine")
	}
}
