package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
	"github.com/t2wu/betterrest/libs/utils/jsontrans"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
)

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
		} else if newF, ok := v.(jsontrans.JSONFields); ok && newF != nil && data[k] != nil { // other object
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

func GuardMiddleWare(typeString string) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		modelObj := models.NewFromTypeString(typeString)
		if m, ok := modelObj.(models.IGuardAPIEntry); ok {
			who := WhoFromContext(r)
			http := models.HTTP{Endpoint: r.URL.Path, Op: models.HTTPMethodToCRUDOp(r.Method)}
			if !m.GuardAPIEntry(who, http) {
				render.Render(w, r, webrender.NewErrPermissionDeniedForAPIEndpoint(nil))
				c.Abort() // abort
				return
			}
		}

		// continues
		return
	}
}
