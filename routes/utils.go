package routes

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
	"github.com/t2wu/betterrest/controller"
	"github.com/t2wu/betterrest/libs/urlparam"
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

type contextKey string

const (
	ContextKeyOption contextKey = "option"
)

func OptionFromContext(r *http.Request) map[urlparam.Param]interface{} {
	var options map[urlparam.Param]interface{}
	item := r.Context().Value(ContextKeyOption)
	if item != nil {
		options = item.(map[urlparam.Param]interface{})
	}
	return options
}

func OptionToContext(c *gin.Context, options map[urlparam.Param]interface{}) {
	ctx := context.WithValue(c.Request.Context(), ContextKeyOption, options)
	c.Request = c.Request.WithContext(ctx)
}

func GuardMiddleWare(typeString string) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		options, err := GetOptionByParsingURL(c.Request)
		if err != nil {
			render.Render(w, r, webrender.NewErrQueryParameter(err))
		}
		OptionToContext(c, options)

		who := WhoFromContext(r)

		// Old, deprecated
		if models.ModelRegistry[typeString].Controller == nil {
			modelObj := models.NewFromTypeString(typeString)
			if m, ok := modelObj.(models.IGuardAPIEntry); ok {
				http := models.HTTP{Endpoint: r.URL.Path, Op: models.HTTPMethodToCRUDOp(r.Method)}
				if !m.GuardAPIEntry(who, http) {
					render.Render(w, r, webrender.NewErrPermissionDeniedForAPIEndpoint(nil))
					c.Abort() // abort
					return
				}
			}
			return // continues
		}
		// End Old, deprecated

		data := controller.Data{Ms: nil, DB: nil, Who: who,
			TypeString: typeString, Roles: nil, URLParams: options, Cargo: nil}
		info := controller.EndPointInfo{
			Op:          controller.HTTPMethodToRESTOp(r.Method),
			Cardinality: controller.APICardinalityOne,
		}

		if ctrl, ok := models.ModelRegistry[typeString].Controller.(controller.IGuardAPIEntry); ok {
			if guardRetVal := ctrl.GuardAPIEntry(&data, &info); !guardRetVal.ToPass {
				defer c.Abort() // abort

				if guardRetVal.Renderer == nil {
					render.Render(w, r, webrender.NewErrPermissionDeniedForAPIEndpoint(nil))
					return
				}
				render.Render(w, r, guardRetVal.Renderer)
				return
			}
		}
	}
}
