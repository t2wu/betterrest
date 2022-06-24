package routes

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
	"github.com/t2wu/betterrest/hookhandler"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/utils/jsontrans"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry"
)

// transformJSONToModel transforms fields when there is IFieldTransformJSONToModel
func transformJSONToModel(data map[string]interface{}, f *jsontrans.JSONFields) error {
	fi := *f
	for k, v := range *f {
		// For our own JSON string type there could be cases where it's an array
		// but I want to store in as the JSON data type in the Postgres column
		// in that case jsontrans.JSONFields should be jsontrans.FieldPass
		if t, ok := fi[k].(jsontrans.Field); ok && t == jsontrans.FieldPass {
			continue
		}

		if datv, ok := data[k].([]interface{}); ok { // is slice after this
			for i := range datv { // loop the slice

				newdat := datv[i].(map[string]interface{})
				if newF, ok := fi[k].(jsontrans.JSONFields); ok {
					if err := transformJSONToModel(newdat, &newF); err != nil {
						return err
					}
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
			c.Abort() // abort
			return
		}
		OptionToContext(c, options)

		who := WhoFromContext(r)

		// Old, deprecated
		if !registry.ModelRegistry[typeString].HandlerMap.HasAttemptRegisteringAnyHandler() {
			modelObj := registry.NewFromTypeString(typeString)
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

		ep := hookhandler.EndPointInfo{
			URL:         c.Request.URL.String(),
			Op:          hookhandler.HTTPMethodToRESTOp(r.Method),
			Cardinality: hookhandler.APICardinalityOne,
			TypeString:  typeString,
			URLParams:   options,
			Who:         who,
		}

		for _, guard := range registry.ModelRegistry[typeString].GuardMethods {
			if retErr := guard(&ep); retErr != nil {
				defer c.Abort() // abort

				if retErr.Renderer == nil {
					render.Render(w, r, webrender.NewErrPermissionDeniedForAPIEndpoint(retErr.Error))
					return
				}
				render.Render(w, r, retErr.Renderer)
				return
			}
		}
	}
}
