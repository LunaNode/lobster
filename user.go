package lobster

import "fmt"
import "time"

type User struct {
	Id int
	Username string
	Email string
	CreateTime time.Time
	Credit int64
	VmLimit int
	LastBillingNotify time.Time
	Status string
	Admin bool
}

type Charge struct {
	Id int
	UserId int
	Name string
	Detail string
	Key string
	Time time.Time
	Amount int64
}

func UserList(db *Database) []*User {
	var users []*User
	rows := db.Query("SELECT id, username, email, time_created, credit, vm_limit, last_billing_notify, status, admin FROM users ORDER BY id")
	defer rows.Close()
	for rows.Next() {
		user := &User{}
		rows.Scan(&user.Id, &user.Username, &user.Email, &user.CreateTime, &user.Credit, &user.VmLimit, &user.LastBillingNotify, &user.Status, &user.Admin)
		users = append(users, user)
	}
	return users
}

func UserDetails(db *Database, userId int) *User {
	user := &User{}
	rows := db.Query("SELECT id, username, email, time_created, credit, vm_limit, last_billing_notify, status, admin FROM users WHERE id = ?", userId)
	if !rows.Next() {
		return nil
	}
	rows.Scan(&user.Id, &user.Username, &user.Email, &user.CreateTime, &user.Credit, &user.VmLimit, &user.LastBillingNotify, &user.Status, &user.Admin)
	rows.Close()
	return user
}

func ChargeList(db *Database, userId int, year int, month time.Month) []*Charge {
	timeStart := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	timeEnd := timeStart.AddDate(0, 1, 0)
	var charges []*Charge
	rows := db.Query("SELECT id, user_id, name, detail, k, time, amount FROM charges WHERE user_id = ? AND time >= ? AND time < ? ORDER BY time", userId, timeStart.Format(MYSQL_TIME_FORMAT), timeEnd.Format(MYSQL_TIME_FORMAT))
	defer rows.Close()
	for rows.Next() {
		charge := Charge{}
		rows.Scan(&charge.Id, &charge.UserId, &charge.Name, &charge.Detail, &charge.Key, &charge.Time, &charge.Amount)
		charges = append(charges, &charge)
	}
	return charges
}

func UserApplyCredit(db *Database, userId int, amount int64, detail string) {
	db.Exec("INSERT INTO charges (user_id, name, time, amount, detail) VALUES (?, ?, CURDATE(), ?, ?)", userId, "Credit updated", -amount, detail);
	db.Exec("UPDATE users SET status = 'active' WHERE id = ? AND status = 'new'", userId);
	userAdjustCredit(db, userId, amount)

	user := UserDetails(db, userId)
	if user.Credit > 0 {
		vms := vmList(db, userId)
		for _, vm := range vms {
			if vm.Suspended == "auto" {
				reportError(vm.Unsuspend(), "failed to unsuspend VM", fmt.Sprintf("user_id: %d, vm_id: %d", userId, vm.Id))
				mailWrap(db, userId, "vmUnsuspend", VmUnsuspendEmail{Name: vm.Name}, false)
			}
		}
	}
}

func UserApplyCharge(db *Database, userId int, name string, detail string, k string, amount int64) {
	rows := db.Query("SELECT id FROM charges WHERE user_id = ? AND k = ? AND time = CURDATE()", userId, k)

	if rows.Next() {
		var chargeId int
		rows.Scan(&chargeId)
		rows.Close()
		db.Exec("UPDATE charges SET amount = amount + ? WHERE id = ?", amount, chargeId)
	} else {
		db.Exec("INSERT INTO charges (user_id, name, amount, time, detail, k) VALUES (?, ?, ?, CURDATE(), ?, ?)", userId, name, amount, detail, k)
	}

	userAdjustCredit(db, userId, -amount)
}

func userAdjustCredit(db *Database, userId int, amount int64) {
	db.Exec("UPDATE users SET credit = credit + ? WHERE id = ?", amount, userId)
}

type CreditSummary struct {
	Credit int64
	Hourly int64
	Daily int64
	Monthly int64
	DaysRemaining string
	Status string
}

func UserCreditSummary(db *Database, userId int) *CreditSummary {
	user := UserDetails(db, userId)
	if user == nil {
		return nil
	}

	summary := CreditSummary{Credit: user.Credit}
	vms := vmList(db, userId)
	for _, vm := range vms {
		summary.Hourly += vm.Plan.Price
	}
	summary.Daily = summary.Hourly * 24
	summary.Monthly = summary.Daily * 30

	// calculate days remaining
	if summary.Daily > 0 {
		daysRemaining := int(summary.Credit / summary.Daily)
		summary.DaysRemaining = fmt.Sprintf("%d", daysRemaining)

		if daysRemaining < 1 {
			summary.Status = "danger"
		} else if daysRemaining < 7 {
			summary.Status = "warning"
		} else {
			summary.Status = "success"
		}
	} else {
		summary.DaysRemaining = "infinite"
		summary.Status = "success"
	}

	return &summary
}

type BandwidthSummary struct {
	Used int64
	Allocated int64
	Billed int64
	NotifiedPercent int
	ActualPercent float64
}

func UserBandwidthSummary(db *Database, userId int) map[string]*BandwidthSummary {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	bw := make(map[string]*BandwidthSummary)
	rows := db.Query("SELECT region, bandwidth_used, bandwidth_additional, bandwidth_billed, bandwidth_notified_percent FROM region_bandwidth WHERE user_id = ?", userId)
	defer rows.Close()
	for rows.Next() {
		var region string
		summary := BandwidthSummary{}
		rows.Scan(&region, &summary.Used, &summary.Allocated, &summary.Billed, &summary.NotifiedPercent)
		bw[region] = &summary

		// add in bandwidth from active vms in this region
		// each one adds bandwidth proportional to the time it was provisioned relative to beginning of month
		for _, vm := range vmListRegion(db, userId, region) {
			planBandwidthBytes := gigaToBytes(vm.Plan.Bandwidth)
			if vm.CreatedTime.After(monthStart) {
				timeRemaining := monthEnd.Sub(vm.CreatedTime)
				if timeRemaining > 0 {
					activeRatio := float64(timeRemaining) / float64(monthEnd.Sub(monthStart))
					bw[region].Allocated += int64(float64(planBandwidthBytes) * activeRatio)
				}
			} else {
				bw[region].Allocated += planBandwidthBytes
			}
		}

		bw[region].ActualPercent = 100 * float64(bw[region].Used) / float64(bw[region].Allocated)
	}
	return bw
}

func userBilling(db *Database, userId int) {
	// bill/notify for bandwidth usage
	creditPerGB := int64(cfg.Billing.BandwidthOverageFee * BILLING_PRECISION)

	for region, summary := range UserBandwidthSummary(db, userId) {
		if summary.Used > gigaToBytes(200) {
			// first do billing
			// we provide some leeway on bandwidth to avoid confusion in edge cases
			//  for example, if user provisions new VM, installs packages, and then deletes it,
			//  then they might go over their bandwidth limit (since it's proportional)
			// it is correct to charge them in this case, but if it's a small amount of bandwidth then the possible confusion isn't worth it
			// also we only bill in multiples of 1 GB (TODO: maybe store it as GB instead of bytes?)
			if summary.Used > summary.Allocated + gigaToBytes(50) {
				gbOver := int((summary.Used - summary.Allocated - summary.Billed) / 1024 / 1024 / 1024)
				if gbOver > 0 {
					UserApplyCharge(db, userId, "Bandwidth", fmt.Sprintf("Bandwidth usage overage charge %s ($%.4f/GB)", region, cfg.Billing.BandwidthOverageFee), "bw-" + region, creditPerGB * int64(gbOver))
					db.Exec("UPDATE region_bandwidth SET bandwidth_billed = bandwidth_billed + ? WHERE user_id = ? AND region = ?", gigaToBytes(gbOver), userId, region)
				}
			}

			// now bandwidth usage notifications
			// we notify on:
			//  changes in percentage exceeding 5
			//  actual overage event (>100%)
			//  also if user provisioned more VM, was no longer over bandwidth, but now is going over again
			if summary.Allocated != 0 {
				utilPercent := int((100 * summary.Used) / summary.Allocated)
				if summary.NotifiedPercent < 100 && utilPercent > 85 && (utilPercent - summary.NotifiedPercent >= 5 || (summary.NotifiedPercent < 100 && utilPercent >= 100) || utilPercent < summary.NotifiedPercent) {
					db.Exec("UPDATE region_bandwidth SET bandwidth_notified_percent = ? WHERE user_id = ? AND region = ?", utilPercent, userId, region)
					tmpl := "bandwidthNotify"
					if utilPercent >= 100 {
						tmpl = "bandwidthOverage"
					}
					emailParams := BandwidthUsageEmail{
						UtilPercent: utilPercent,
						Region: region,
						Fee: creditPerGB,
					}
					mailWrap(db, userId, tmpl, emailParams, false)
				}
			}
		}
	}

	// check for low account balance, possibly suspend/terminate virtual machines
	rows := db.Query("SELECT credit, email, TIMESTAMPDIFF(HOUR, last_billing_notify, NOW()), billing_low_count FROM users WHERE id = ? AND last_billing_notify < DATE_SUB(NOW(), INTERVAL 24 HOUR) AND (SELECT COUNT(*) FROM vms WHERE vms.user_id = users.id) > 0", userId)
	if !rows.Next() {
		return
	}

	var credit int64
	var email string
	var lastBilledHoursAgo int // how many hours ago this user was last billed
	var billingLowCount int // how many reminders regarding low account balance have been sent
	rows.Scan(&credit, &email, &lastBilledHoursAgo, &billingLowCount)
	rows.Close()
	hourly := UserCreditSummary(db, userId).Hourly

	if credit <= hourly * 168 {
		if credit < 0 && billingLowCount >= 5 {
			if credit < -168 * hourly && lastBilledHoursAgo > 0 && lastBilledHoursAgo <= 48 {
				// terminte the account
				vms := vmList(db, userId)
				for _, vm := range vms {
					reportError(vm.Delete(userId), "failed to delete VM", fmt.Sprintf("user_id: %d, vm_id: %d", userId, vm.Id))
				}
				mailWrap(db, userId, "userTerminate", nil, false)
			} else {
				// suspend
				vms := vmList(db, userId)
				for _, vm := range vms {
					reportError(vm.Suspend(true), "failed to suspend VM", fmt.Sprintf("user_id: %d, vm_id: %d", userId, vm.Id))
				}
				mailWrap(db, userId, "userSuspend", nil, false)
			}
		} else {
			// send low credit warning
			remainingHours := int(credit / hourly)
			params := LowCreditEmail{
				Credit: credit,
				Hourly: hourly,
				RemainingHours: remainingHours,
			}
			tmpl := "userLowCredit"
			if credit < 0 {
				tmpl = "userNegativeCredit"
			}
			mailWrap(db, userId, tmpl, params, false)
		}

		db.Exec("UPDATE users SET last_billing_notify = NOW(), billing_low_count = billing_low_count + 1 WHERE id = ?", userId)
	} else {
		db.Exec("UPDATE users SET last_billing_notify = NOW(), billing_low_count = 0 WHERE id = ?", userId)
	}
}
