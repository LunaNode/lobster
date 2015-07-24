package lobster

import _ "github.com/go-sql-driver/mysql"

import "database/sql"
import "log"
import "net/http"

type Database struct {
	db *sql.DB
}

func GetDatabaseString() string {
	s := cfg.Database.Username + ":" + cfg.Database.Password + "@"
	if cfg.Database.Host != "localhost" {
		s += cfg.Database.Host
	}
	s += "/" + cfg.Database.Name + "?charset=utf8&parseTime=true"
	return s
}

func MakeDatabase() *Database {
	this := new(Database)
	db, err := sql.Open("mysql", GetDatabaseString())
	checkErr(err)
	this.db = db
	return this
}

func (this *Database) Query(q string, args ...interface{}) *sql.Rows {
	if cfg.Default.Debug {
		log.Printf("%s on %v", q, args)
	}
	rows, err := this.db.Query(q, args...)
	checkErr(err)
	return rows
}

func (this *Database) QueryRow(q string, args ...interface{}) *sql.Row {
	if cfg.Default.Debug {
		log.Printf("%s on %v", q, args)
	}
	row := this.db.QueryRow(q, args...)
	return row
}

func (this *Database) Exec(q string, args ...interface{}) sql.Result {
	if cfg.Default.Debug {
		log.Printf("%s on %v", q, args)
	}
	result, err := this.db.Exec(q, args...)
	checkErr(err)
	return result
}

func (this *Database) WrapHandler(handler func(http.ResponseWriter, *http.Request, *Database)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer errorHandler(w, r, true)
		handler(w, r, this)
	}
}
