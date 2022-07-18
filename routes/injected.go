package routes

import (
	"net/http"

	"github.com/t2wu/betterrest/mdlutil"
)

// Register Who handler
// func WhoFromContext(r *http.Request) mdl.Who
var WhoFromContext func(r *http.Request) mdlutil.UserIDFetchable
