package routes

import (
	"net/http"

	"github.com/t2wu/betterrest/models"
)

// Register Who handler
// func WhoFromContext(r *http.Request) models.Who
var WhoFromContext func(r *http.Request) models.UserIDFetchable
