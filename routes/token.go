package routes

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/security"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
)

type tokenRefresh struct {
	RefreshToken string `json:"refresh_token"`
}

// Token handles refresh token
func Token(c *gin.Context) {
	w, r := c.Writer, c.Request
	// verify refresh token
	var jsn []byte
	var err error

	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		render.Render(w, r, NewErrReadingBody(err))
		return
	}

	m := &tokenRefresh{}
	err = json.Unmarshal(jsn, m)
	if err != nil {
		render.Render(w, r, NewErrParsingJSON(err))
		return
	}

	claims, err := security.GetClaimsIfRefreshTokenIsValid(m.RefreshToken)
	if err != nil {
		log.Println(err)
		render.Render(w, r, NewErrInvalidRefreshToken(errors.New("invalid refresh token"))) // Token invalid (either expired or just wrong)
		return
	}
	ownerID, err := datatypes.NewUUIDFromString((*claims)["iss"].(string)) // should always be ok
	if err != nil {
		render.Render(w, r, NewErrGeneratingToken(err))
		return
	}

	scope := (*claims)["scope"].(string) // should always be ok

	// if ident, ok := claims["iss"]; ok {
	// 	if ident, ok := ident.(string); ok {
	// 		ownerID datatypes.NewUUIDFromString(ident)
	// 	}
	// 	render.Render(w, r, NewErrInvalidRefreshToken(errors.New("getting ISS from token error")))
	// } else {
	// 	render.Render(w, r, NewErrInvalidRefreshToken(errors.New("getting ISS from token error")))
	// }

	// Issue new token
	var payload map[string]interface{}
	payload, err = createTokenPayloadForScope(ownerID, &scope)
	if err != nil {
		render.Render(w, r, NewErrGeneratingToken(err))
		return
	}

	if jsn, err = json.Marshal(payload); err != nil {
		render.Render(w, r, NewErrGenJSON(err))
		return
	}

	w.Write(jsn)
}
