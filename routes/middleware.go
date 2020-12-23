package routes

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/security"
	"github.com/t2wu/betterrest/models"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
	"github.com/jinzhu/gorm"
)

type contextKey int

const (
	// ContextKeyOwnerID is the id that's given in jwt's iss field
	ContextKeyOwnerID contextKey = iota
	ContextKeyClient
	ContextKeyScope
	ContextKeyIat
	ContextKeyExp
	ContextKeyTokenHours
)

// http.NotFound
// func NotFound(w ResponseWriter, r *Request) { Error(w, "404 page not found", StatusNotFound) }
// error can be found in https://golang.org/pkg/net/http/#Error

// func NotFoundHandler() Handler { return HandlerFunc(NotFound) }

// ClientAuthMiddleware make users that the software access this API has
// basic client ID
// Insert a test one:
// Insert into client (secret) values ("123");
func ClientAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request
		var clientID, secret string
		var ok bool

		clientID, secret, ok = apiKeyAuth(r)
		if !ok {
			render.Render(w, r, NewErrClientNotAuthorized(errors.New("missing client credentials")))
			c.Abort()
			return
		}

		// Verify clientID and secret
		client := new(models.Client)
		id, err := strconv.Atoi(clientID)
		if err != nil {
			render.Render(w, r, NewErrClientNotAuthorized(errors.New("incorrect client credentials")))
			c.Abort()
			return
		}

		if err := db.Shared().First(&client, id).Error; gorm.IsRecordNotFoundError(err) {
			// Record not found here.
			render.Render(w, r, NewErrClientNotAuthorized(errors.New("incorrect client credentials")))
			c.Abort()
			return
		} else if err != nil { // Other type of error
			render.Render(w, r, NewErrClientNotAuthorized(err))
			c.Abort()
			return
		}

		if client.Secret != secret {
			// Unauthorzed
			render.Render(w, r, NewErrClientNotAuthorized(errors.New("incorrect client credentials")))
			c.Abort()
			return
		}

		ctx := context.WithValue(c.Request.Context(), ContextKeyClient, client)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// BearerAuthMiddleware make sure the Bearer token exits and validate it
// And also get the user ID into the context
func BearerAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request
		var token string
		var ok bool

		if token, ok = BearerToken(r); !ok {
			render.Render(w, r, NewErrTokenInvalid(errors.New("missing token"))) // Not pass, no token
			c.Abort()
			return
		}

		claims, err := security.GetClaimsIfAccessTokenIsValid(token)
		if err != nil {
			log.Println(err)
			render.Render(w, r, NewErrTokenInvalid(errors.New("invalid token"))) // Token invalid (either expired or just wrong)
			c.Abort()
			return
		}

		if ident, ok := (*claims)["iss"].(string); ok {
			ownerID, err := datatypes.NewUUIDFromString(ident)
			if err != nil {
				render.Render(w, r, NewErrTokenInvalid(err))
				c.Abort()
				return
			}

			ctx := context.WithValue(c.Request.Context(), ContextKeyOwnerID, ownerID)
			c.Request = c.Request.WithContext(ctx)
		} else {
			render.Render(w, r, NewErrTokenInvalid(errors.New("getting ISS from token error")))
			c.Abort()
			return
		}

		// var scope string
		if scope, ok := (*claims)["scope"].(string); ok {
			if err != nil {
				render.Render(w, r, NewErrTokenInvalid(err))
				c.Abort()
				return
			}

			ctx := context.WithValue(c.Request.Context(), ContextKeyScope, scope)
			c.Request = c.Request.WithContext(ctx)
		} else {
			render.Render(w, r, NewErrTokenInvalid(errors.New("getting ISS from token error")))
			c.Abort()
			return
		}

		if iat, ok := (*claims)["iat"].(string); ok {
			if err != nil {
				render.Render(w, r, NewErrTokenInvalid(err))
				c.Abort()
				return
			}

			ctx := context.WithValue(c.Request.Context(), ContextKeyIat, iat)
			c.Request = c.Request.WithContext(ctx)
		} else {
			render.Render(w, r, NewErrTokenInvalid(errors.New("getting ISS from token error")))
			c.Abort()
			return
		}

		if exp, ok := (*claims)["exp"].(string); ok {
			if err != nil {
				render.Render(w, r, NewErrTokenInvalid(err))
				c.Abort()
				return
			}

			ctx := context.WithValue(c.Request.Context(), ContextKeyExp, exp)
			c.Request = c.Request.WithContext(ctx)
		} else {
			render.Render(w, r, NewErrTokenInvalid(errors.New("getting ISS from token error")))
			c.Abort()
			return
		}

		c.Next()
	}
}

// XDebugMiddleWare parses "X-DEBUG-TOKEN-DURATION-HOURS" if available
func XDebugMiddleWare() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, r := c.Writer, c.Request

		tokenDuration := r.Header.Get("X-DEBUG-TOKEN-DURATION-HOURS")
		if tokenDuration != "" {
			if tokenDurationInHours, err := strconv.Atoi(tokenDuration); err == nil {
				ctx := context.WithValue(c.Request.Context(), ContextKeyTokenHours, tokenDurationInHours)
				c.Request = c.Request.WithContext(ctx)
			}
			// if error, ignore, continue
		}

		c.Next()
	}
}

// BearerToken returns bearer token
func BearerToken(r *http.Request) (token string, ok bool) {
	ok = false

	authstring := r.Header.Get("Authorization")
	fields := strings.Split(authstring, ",")

	for _, v := range fields {
		if ok = strings.HasPrefix(v, "Bearer"); ok {
			fmt.Sscanf(v, "Bearer %s", &token)
			return token, ok
		}
	}

	return "", false
}

// BasicToken()
func BasicToken(r *http.Request) (token string, ok bool) {
	ok = false

	authstring := r.Header.Get("Authorization")
	fields := strings.Split(authstring, ",")

	for _, v := range fields {
		if ok = strings.HasPrefix(v, "Basic"); ok {
			fmt.Sscanf(v, "Basic %s", &token)
			return token, ok
		}
	}

	return "", false
}

// apiKeyAuth gets client id and secret via HTTP header X-API-KEY
func apiKeyAuth(r *http.Request) (username, secret string, ok bool) {
	apiKey := r.Header.Get("X-API-KEY")

	if apiKey == "" {
		return
	}

	c, err := base64.StdEncoding.DecodeString(apiKey)
	if err != nil {
		return
	}

	cs := string(c)
	s := strings.IndexByte(cs, ':')

	if s < 0 {
		return
	}

	return cs[:s], cs[s+1:], true
}
