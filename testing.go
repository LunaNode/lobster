package lobster

import "github.com/gorilla/mux"

import "github.com/LunaNode/lobster/utils"

const TEST_BANDWIDTH = 1000

var testTables []string = []string{"users", "region_bandwidth", "vms", "plans", "charges", "sessions", "form_tokens", "antiflood"}

func TestReset() *Database {
	cfg = &Config{
		Default: ConfigDefault{
			Debug: true,
		},
		Billing: ConfigBilling{
			BandwidthOverageFee: 0.003,
		},
		Database: ConfigDatabase{
			Host: "localhost",
			Username: "lobstertest",
			Password: "",
			Name: "lobstertest",
		},
		Novnc: ConfigNovnc{
			Listen: "127.0.0.1:6080",
			Url: "TOKEN",
		},
	}
	db := MakeDatabase()

	// clear all tables
	for _, table := range testTables {
		db.Exec("DELETE FROM " + table)
	}

	return db
}

func TestSetup() {
	router = mux.NewRouter()
	db = MakeDatabase()
}

// Creates user and returns user id.
func TestUser(db *Database) int {
	result := db.Exec("INSERT INTO users (username, password, credit) VALUES (?, '', 1000000)", utils.Uid(8))
	userId, _ := result.LastInsertId()
	return int(userId)
}

func TestVm(db *Database, userId int) int {
	result := db.Exec("INSERT INTO plans (name, price, ram, cpu, storage, bandwidth) VALUES ('', 6000, 512, 1, 15, ?)", TEST_BANDWIDTH)
	planId, _ := result.LastInsertId()
	result = db.Exec("INSERT INTO vms (user_id, region, plan_id, name, status) VALUES (?, 'test', ?, '', 'active')", userId, planId)
	vmId, _ := result.LastInsertId()
	return int(vmId)
}
