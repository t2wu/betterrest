package routes

import (
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/security"
	"github.com/t2wu/betterrest/models"
)

// createTokenPayloadForScope creates token JSON payload
// Follow oauth's
// { acceess_token: acces_token, token_type: "Bearer", refresh_token: refresh_token, scope: ""}
func createTokenPayloadForScope(id *datatypes.UUID, scope *string) (map[string]interface{}, error) {
	var accessToken, refreshToken string
	var err error
	accessToken, err = security.CreateAccessToken(id, time.Hour*time.Duration(3), scope) // 3 hours
	if err != nil {
		return nil, err
	}

	refreshToken, err = security.CreateRefreshToken(id, time.Hour*24*time.Duration(60), scope) // 60 days
	if err != nil {
		return nil, err
	}

	retval := map[string]interface{}{
		"code": 0,
		"content": map[string]interface{}{
			"accessToken":  accessToken,
			"tokenType":    "Bearer",
			"refreshToken": refreshToken,
			"id":           id,
		},
	}

	return retval, nil
}

func removeCreatedAtFromModel(original []byte) ([]byte, error) {
	jsonPatch := []byte("[{ \"op\": \"remove\", \"path\": \"/createdAt\" }]")
	patch, err := jsonpatch.DecodePatch(jsonPatch)
	if err != nil {
		return nil, err
	}

	return patch.Apply(original)
}

func guardMiddleWare(typeString string) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		modelObj := models.NewFromTypeString(typeString)
		if m, ok := modelObj.(models.IGuardAPIEntry); ok {
			scope := ScopeFromContext(r)
			if !m.GuardAPIEntry(&scope, r.URL.Path, r.Method) {
				render.Render(w, r, NewErrPermissionDeniedForAPIEndpoint(nil))
				c.Abort() // abort
				return
			}
		}

		// continues
		return
	}
}
