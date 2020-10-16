package db

import (
	"sync"

	"github.com/jinzhu/gorm"
	// _ "github.com/jinzhu/gorm/dialects/sqlite"
	// _ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

var once sync.Once
var db *gorm.DB

var sqlDbName string
var username string
var passwd string
var host string
var port string
var toLog string

// DatabaseConfig is database configuration
type DatabaseConfig struct {
	SQLDbName string
	Username  string
	Passwd    string
	Host      string
	Port      string
	ToLog     string
}

// Setup setup the db
func Setup(c *DatabaseConfig) {
	sqlDbName = c.SQLDbName
	username = c.Username
	passwd = c.Passwd
	host = c.Host
	port = c.Port
	toLog = c.ToLog
}

// Shared is a singleton call
func Shared() *gorm.DB {
	// For thread safety
	// Does not allow repeating (FIXME: what if database goes down?)
	once.Do(func() {
		// Somehow I have to set _db first, otherwise return db will have nil
		// _db, err := gorm.Open("sqlite3", "./test.db")
		// _db, err := gorm.Open("mysql", username+":"+passwd+"@tcp("+host+":"+port+")/"+sqlDbName+"?charset=utf8mb4&parseTime=True&loc=Local")
		_db, err := gorm.Open("postgres", "host="+host+" port="+port+" user="+username+" dbname="+sqlDbName+" password="+passwd+" sslmode=disable")

		if toLog == "true" {
			_db.LogMode(true)
		}

		if err != nil {
			panic("failed to connect database:" + err.Error())
		}
		// defer db.Close()
		_db.SingularTable(true)

		db = _db
	})

	return db
}
