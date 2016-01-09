package lobster

import _ "github.com/go-sql-driver/mysql"

import "database/sql"
import "log"

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

func (this *Database) Query(q string, args ...interface{}) Rows {
	if cfg.Default.Debug {
		log.Printf("%s on %v", q, args)
	}
	rows, err := this.db.Query(q, args...)
	checkErr(err)
	return Rows{rows}
}

func (this *Database) QueryRow(q string, args ...interface{}) Row {
	if cfg.Default.Debug {
		log.Printf("%s on %v", q, args)
	}
	row := this.db.QueryRow(q, args...)
	return Row{row}
}

func (this *Database) Exec(q string, args ...interface{}) Result {
	if cfg.Default.Debug {
		log.Printf("%s on %v", q, args)
	}
	result, err := this.db.Exec(q, args...)
	checkErr(err)
	return Result{result}
}

type Rows struct {
	rows *sql.Rows
}

func (r Rows) Close() {
	err := r.rows.Close()
	checkErr(err)
}

func (r Rows) Next() bool {
	return r.rows.Next()
}

func (r Rows) Scan(dest ...interface{}) {
	err := r.rows.Scan(dest...)
	checkErr(err)
}

type Row struct {
	row *sql.Row
}

func (r Row) Scan(dest ...interface{}) {
	err := r.row.Scan(dest...)
	checkErr(err)
}

type Result struct {
	result sql.Result
}

func (r Result) LastInsertId() int {
	id, err := r.result.LastInsertId()
	checkErr(err)
	return int(id)
}

func (r Result) RowsAffected() int {
	count, err := r.result.RowsAffected()
	checkErr(err)
	return int(count)
}
