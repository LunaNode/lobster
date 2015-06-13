package lobster

func antifloodAction(db *Database, ip string, action string) {
	rows := db.Query("SELECT id FROM antiflood WHERE ip = ? AND action = ? AND time > DATE_SUB(NOW(), INTERVAL 1 HOUR)", ip, action)
	defer rows.Close()

	if rows.Next() {
		var rowId int
		rows.Scan(&rowId)
		db.Exec("UPDATE antiflood SET count = count + 1, time = NOW() WHERE id = ?", rowId)
	} else {
		db.Exec("INSERT INTO antiflood (ip, action, count, time) VALUES (?, ?, 1, NOW())", ip, action)
	}
}

func antifloodCheck(db *Database, ip string, action string, maxCount int) bool {
	var countBad int
	db.QueryRow("SELECT COUNT(*) FROM antiflood WHERE ip = ? AND action = ? AND count >= ? AND time > DATE_SUB(NOW(), INTERVAL 1 HOUR)", ip, action, maxCount).Scan(&countBad)
	return countBad == 0
}
