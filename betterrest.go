package betterrest

import (
	"net/http"

	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/libs/settings"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/betterrest/registry"
	"github.com/t2wu/betterrest/routes"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/db"
)

type Config struct {
	Log           bool
	TransactDebug bool
}

func SetConfig(cfg Config) {
	settings.Log = cfg.Log
	settings.TransactDebug = cfg.TransactDebug
}

/*
 * DB
 */

// RegDB register the db
func RegDB(dbi *gorm.DB) {
	db.SetUpDB(dbi)
}

/*
 * RegisterContextFunction
 */

func RegisterContextFunction(f func(r *http.Request) mdlutil.UserIDFetchable) {
	routes.WhoFromContext = f
}

var For func(typeString string) *registry.Registrar = registry.For

var Sorter func(sorter hook.IRoleSorter) = registry.RegRoleSorter
