package routes

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/security"
	"github.com/t2wu/betterrest/libs/utils/jsontrans"
	"github.com/t2wu/betterrest/models"
)

// createTokenPayloadForScope creates token JSON payload
// Follow oauth's
// { acceess_token: acces_token, token_type: "Bearer", refresh_token: refreshToken, scope: ""}
func createTokenPayloadForScope(id *datatypes.UUID, scope *string, tokenHours *float64) (map[string]interface{}, error) {
	var accessToken, refreshToken string
	var err error

	// 3 hours by default or by X-DEBUG-TOKEN-DURATION-HOURS
	tokenTime := time.Minute * 60 * 3 // default to three hours
	if tokenHours != nil {
		tokenTime = time.Minute * time.Duration((*tokenHours)*60)
	}
	accessToken, err = security.CreateAccessToken(id, tokenTime, scope)
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

// transformJSONToModel transforms fields when there is IFieldTransformJSONToModel
func transformJSONToModel(data map[string]interface{}, f *jsontrans.JSONFields) error {
	fi := *f
	for k, v := range *f {
		if datv, ok := data[k].([]interface{}); ok { // is slice after this
			for i := range datv { // loop the slice

				newdat := datv[i].(map[string]interface{})
				newF := fi[k].(jsontrans.JSONFields)
				if err := transformJSONToModel(newdat, &newF); err != nil {
					return err
				}
			}
		} else if newF, ok := v.(jsontrans.JSONFields); ok { // other object
			// embeddedStruct := make(map[string]interface{})
			if err := transformJSONToModel(data[k].(map[string]interface{}), &newF); err != nil {
				return err
			}
			// data[k] = embeddedStruct
		} else { // data field
			if transformStruct, ok := v.(jsontrans.IFieldTransformJSONToModel); ok {
				transV, err := transformStruct.TransformJSONToModel(data[k])
				if err != nil {
					return err
				}
				data[k] = transV
			}
		}
	}
	return nil
}

// func transformJSONToModelCore(modelInMap map[string]interface{}, fields jsontrans.JSONFields) (map[string]interface{}, error) {
// }

func guardMiddleWare(typeString string) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		modelObj := models.NewFromTypeString(typeString)
		if m, ok := modelObj.(models.IGuardAPIEntry); ok {
			who := WhoFromContext(r)
			http := models.HTTP{Endpoint: r.URL.Path, Method: r.Method}
			if !m.GuardAPIEntry(who, http) {
				render.Render(w, r, NewErrPermissionDeniedForAPIEndpoint(nil))
				c.Abort() // abort
				return
			}
		}

		// continues
		return
	}
}
