package routes

import (
	"betterrest/db"
	"betterrest/libs/security"
	"betterrest/libs/utils"
	"betterrest/models"
	"context"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/render"
	"github.com/jinzhu/gorm"
)

type contextKey int

const (
	// contextKeyOwnerID is the id that's given in jwt's iss field
	contextKeyOwnerID contextKey = iota
	contextKeyClient  contextKey = iota
)

// http.NotFound
// func NotFound(w ResponseWriter, r *Request) { Error(w, "404 page not found", StatusNotFound) }
// error can be found in https://golang.org/pkg/net/http/#Error

// func NotFoundHandler() Handler { return HandlerFunc(NotFound) }

// ClientBasicAuthMiddleware make users that the software access this API has
// basic client ID
// Insert a test one:
// Insert into client (secret) values ("123");
func ClientBasicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var clientID, secret string
		var ok bool

		clientID, secret, ok = utils.BasicAuth(r)
		if !ok {
			// could not get basic authentication credentials
			render.Render(w, r, ErrClientNotAuthorized)
			return
		}

		// Verify clientID and secret
		client := new(models.Client)
		id, err := strconv.Atoi(clientID)
		if err != nil {
			render.Render(w, r, ErrClientNotAuthorized)
			return
		}

		if err := db.Shared().First(&client, id).Error; gorm.IsRecordNotFoundError(err) {
			// Record not found here.
			render.Render(w, r, ErrClientNotAuthorized)
			return
		} else if err != nil { // Other type of error
			render.Render(w, r, ErrClientNotAuthorized)
			return
		}

		if client.Secret != secret {
			// Unauthorzed
			render.Render(w, r, ErrClientNotAuthorized)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyClient, client)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// BearerAuthMiddleware make sure the Bearer token exits and validate it
// And also get the user ID into the context
func BearerAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string
		var ok bool

		if token, ok = utils.BearerToken(r); !ok {
			render.Render(w, r, ErrTokenInvalid) // Not pass, no token
			return
		}

		ownerID, err := security.GetISSIfTokenIsValid(token)
		if err != nil {
			log.Println(err)
			render.Render(w, r, ErrTokenInvalid) // Token invalid (either expired or just wrong)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyOwnerID, ownerID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
