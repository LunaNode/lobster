package lobster

import "testing"

func TestAntiflood(t *testing.T) {
	// we run actions and make sure it only fails after the correct number
	// we then make sure we can expire the antiflood
	// we run this twice to ensure the expiration isn't conflicting with future checks
	db := TestReset()
	for iteration := 0; iteration < 2; iteration++ {

		for i := 1; i <= 5; i++ {
			if !AntifloodCheck(db, "127.0.0.1", "test", 5) {
				t.Fatalf("Antiflood check failed on round %d", i)
			}
			AntifloodAction(db, "127.0.0.1", "test")
		}

		if AntifloodCheck(db, "127.0.0.1", "test", 5) {
			t.Fatalf("Antiflood check succeeded unexpectedly")
		}

		db.Exec("UPDATE antiflood SET time = DATE_SUB(NOW(), INTERVAL 70 MINUTE)")
		if !AntifloodCheck(db, "127.0.0.1", "test", 5) {
			t.Fatalf("Antiflood check after expiring")
		}
	}
}
