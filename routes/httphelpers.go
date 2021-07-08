package routes

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/utils/jsontrans"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	uuid "github.com/satori/go.uuid"
)

// WhoFromContext fetches struct Who from request context
func WhoFromContext(r *http.Request) models.Who {
	ownerID, scope, client := OwnerIDFromContext(r), ScopeFromContext(r), ClientFromContext(r)
	return models.Who{
		Client: client,
		Oid:    ownerID,
		Scope:  scope,
	}
}

// ClientFromContext gets Client from context
func ClientFromContext(r *http.Request) *models.Client {
	var client *models.Client
	item := r.Context().Value(ContextKeyClient)
	if item != nil {
		client = item.(*models.Client)
	}
	return client
}

// OwnerIDFromContext gets id from context
func OwnerIDFromContext(r *http.Request) *datatypes.UUID {
	var ownerID *datatypes.UUID
	item := r.Context().Value(ContextKeyOwnerID)
	if item != nil {
		ownerID = item.(*datatypes.UUID)
	}
	return ownerID
}

// ScopeFromContext gets scope from context
func ScopeFromContext(r *http.Request) *string {
	var scope *string
	item := r.Context().Value(ContextKeyScope)
	if item != nil {
		s := item.(string)
		scope = &s
	}
	return scope
}

// IatFromContext gets iat from context
func IatFromContext(r *http.Request) float64 {
	var iat float64
	item := r.Context().Value(ContextKeyIat)
	if item != nil {
		iat = item.(float64)
	}
	return iat
}

// ExpFromContext gets iat from context
func ExpFromContext(r *http.Request) float64 {
	var exp float64
	item := r.Context().Value(ContextKeyExp)
	if item != nil {
		exp = item.(float64)
	}
	return exp
}

// TokenHoursFromContext gets hours from context
func TokenHoursFromContext(r *http.Request) *float64 {
	item := r.Context().Value(ContextKeyTokenHours)
	if item != nil {
		tokenHours := item.(float64)
		return &tokenHours
	}
	return nil
}

// JSONBodyWithContent for partial unmarshalling
type JSONBodyWithContent struct {
	Content []json.RawMessage
}

// ModelOrModelsFromJSONBody parses JSON body into array of models
// It take care where the case when it is not even an array and there is a "content" in there
func ModelOrModelsFromJSONBody(r *http.Request, typeString string, who models.Who) ([]models.IModel, *bool, render.Renderer) {
	defer r.Body.Close()
	var jsn []byte
	var modelObjs []models.IModel
	var err error
	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, nil, webrender.NewErrReadingBody(err)
	}

	var jcmodel JSONBodyWithContent

	modelObj := models.NewFromTypeString(typeString)

	needTransform := false
	var fields jsontrans.JSONFields
	if modelObjPerm, ok := modelObj.(models.IHasPermissions); ok {
		_, fields = modelObjPerm.Permissions(models.UserRoleAdmin, who)
		needTransform = jsontrans.ContainsIFieldTransformModelToJSON(&fields)
	}

	err = json.Unmarshal(jsn, &jcmodel)
	if err != nil {
		return nil, nil, webrender.NewErrParsingJSON(err)
	}

	if len(jcmodel.Content) == 0 {
		// then it's not a batch insert

		if needTransform {
			var modelInMap map[string]interface{}
			if err = json.Unmarshal(jsn, &modelInMap); err != nil {
				return nil, nil, webrender.NewErrParsingJSON(err)
			}

			if err = transformJSONToModel(modelInMap, &fields); err != nil {
				return nil, nil, webrender.NewErrParsingJSON(err)
			}

			if jsn, err = json.Marshal(modelInMap); err != nil {
				return nil, nil, webrender.NewErrParsingJSON(err)
			}
		}

		err = json.Unmarshal(jsn, modelObj)
		if err != nil {
			return nil, nil, webrender.NewErrParsingJSON(err)
		}

		err := models.Validate.Struct(modelObj)
		if err != nil {
			errs := err.(validator.ValidationErrors)
			return nil, nil, webrender.NewErrValidation(errs)
		}

		if v, ok := modelObj.(models.IValidate); ok {
			who := WhoFromContext(r)
			http := models.HTTP{Endpoint: r.URL.Path, Method: r.Method}
			if err := v.Validate(who, http); err != nil {
				return nil, nil, webrender.NewErrValidation(err)
			}
		}

		modelObjs = append(modelObjs, modelObj)
		isBatch := false
		return modelObjs, &isBatch, nil
	}

	for _, jsnModel := range jcmodel.Content {

		if needTransform {
			var modelInMap map[string]interface{}
			if err = json.Unmarshal(jsnModel, &modelInMap); err != nil {
				return nil, nil, webrender.NewErrParsingJSON(err)
			}

			if err = transformJSONToModel(modelInMap, &fields); err != nil {
				return nil, nil, webrender.NewErrParsingJSON(err)
			}

			if jsnModel, err = json.Marshal(modelInMap); err != nil {
				return nil, nil, webrender.NewErrParsingJSON(err)
			}
		}

		modelObj := models.NewFromTypeString(typeString)
		err = json.Unmarshal(jsnModel, modelObj)
		if err != nil {
			return nil, nil, webrender.NewErrParsingJSON(err)
		}

		// err := models.Validate.Struct(modelObj)
		// if err != nil {
		// 	errs := err.(validator.ValidationErrors)
		// 	return nil, nil, NewErrValidation(errs)
		// }

		if v, ok := modelObj.(models.IValidate); ok {
			who := WhoFromContext(r)
			http := models.HTTP{Endpoint: r.URL.Path, Method: r.Method}
			if err := v.Validate(who, http); err != nil {
				return nil, nil, webrender.NewErrValidation(err)
			}
		}
		// return nil, nil, NewErrValidation(errors.New("test"))

		modelObjs = append(modelObjs, modelObj)
	}

	isBatch := true
	return modelObjs, &isBatch, nil
}

// ModelsFromJSONBody parses JSON body into array of models
func ModelsFromJSONBody(r *http.Request, typeString string, who models.Who) ([]models.IModel, render.Renderer) {
	defer r.Body.Close()
	var jsn []byte
	var modelObjs []models.IModel
	var err error
	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, webrender.NewErrReadingBody(err)
	}

	// Previously I don't know about partial marshalling
	// So I had to unmarshal to the array of reflected type
	// And then create an []IModel an assign it one by one.
	// Now I can unmarshal each record one by one from json.RawMessage
	var jcmodel JSONBodyWithContent

	err = json.Unmarshal(jsn, &jcmodel)
	if err != nil {
		return nil, webrender.NewErrParsingJSON(err)
	}

	modelTest := models.NewFromTypeString(typeString)
	needTransform := false
	var fields jsontrans.JSONFields
	if modelObjPerm, ok := modelTest.(models.IHasPermissions); ok {
		_, fields = modelObjPerm.Permissions(models.UserRoleAdmin, who)
		needTransform = jsontrans.ContainsIFieldTransformModelToJSON(&fields)
	}

	for _, jsnModel := range jcmodel.Content {
		if needTransform {
			var modelInMap map[string]interface{}
			if err = json.Unmarshal(jsnModel, &modelInMap); err != nil {
				return nil, webrender.NewErrParsingJSON(err)
			}

			if err = transformJSONToModel(modelInMap, &fields); err != nil {
				return nil, webrender.NewErrParsingJSON(err)
			}

			if jsnModel, err = json.Marshal(modelInMap); err != nil {
				return nil, webrender.NewErrParsingJSON(err)
			}
		}

		modelObj := models.NewFromTypeString(typeString)
		err = json.Unmarshal(jsnModel, modelObj)
		if err != nil {
			return nil, webrender.NewErrParsingJSON(err)
		}

		// err := models.Validate.Struct(modelObj)
		// if err != nil {
		// 	errs := err.(validator.ValidationErrors)
		// 	return nil, NewErrValidation(errs)
		// }

		if v, ok := modelObj.(models.IValidate); ok {
			who := WhoFromContext(r)
			http := models.HTTP{Endpoint: r.URL.Path, Method: r.Method}
			if err := v.Validate(who, http); err != nil {
				return nil, webrender.NewErrValidation(err)
			}
		}

		modelObjs = append(modelObjs, modelObj)
	}

	return modelObjs, nil
}

// ModelFromJSONBody parses JSON body into a model
// FIXME:
// Validation should not be done here because empty field does not pass validation,
// but sometimes we need empty fields such as patch
func ModelFromJSONBody(r *http.Request, typeString string, who models.Who) (models.IModel, render.Renderer) {
	defer r.Body.Close()
	var jsn []byte
	var err error

	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, webrender.NewErrReadingBody(err)
	}

	modelObj := models.NewFromTypeString(typeString)

	if modelObjPerm, ok := modelObj.(models.IHasPermissions); ok {
		// removeCreated := false
		_, fields := modelObjPerm.Permissions(models.UserRoleAdmin, who)

		// black list or white list all the same, transform is transform
		if jsontrans.ContainsIFieldTransformModelToJSON(&fields) {
			// First extract into map interface, then convert it
			var modelInMap map[string]interface{}
			if err = json.Unmarshal(jsn, &modelInMap); err != nil {
				return nil, webrender.NewErrParsingJSON(err)
			}

			if err = transformJSONToModel(modelInMap, &fields); err != nil {
				return nil, webrender.NewErrParsingJSON(err)
			}

			if jsn, err = json.Marshal(modelInMap); err != nil {
				return nil, webrender.NewErrParsingJSON(err)
			}
		}
	}

	if err = json.Unmarshal(jsn, modelObj); err != nil {
		return nil, webrender.NewErrParsingJSON(err)
	}

	if v, ok := modelObj.(models.IValidate); ok {
		who := WhoFromContext(r)
		http := models.HTTP{Endpoint: r.URL.Path, Method: r.Method}
		if err := v.Validate(who, http); err != nil {
			return nil, webrender.NewErrValidation(err)
		}
	}

	return modelObj, nil
}

func ModelFromJSONBodyNoWhoNoCheckPermissionNoTransform(r *http.Request, typeString string) (models.IModel, render.Renderer) {
	defer r.Body.Close()
	var jsn []byte
	var err error

	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, webrender.NewErrReadingBody(err)
	}

	modelObj := models.NewFromTypeString(typeString)

	if err = json.Unmarshal(jsn, modelObj); err != nil {
		return nil, webrender.NewErrParsingJSON(err)
	}

	if v, ok := modelObj.(models.IValidate); ok {
		who := WhoFromContext(r)
		http := models.HTTP{Endpoint: r.URL.Path, Method: r.Method}
		if err := v.Validate(who, http); err != nil {
			return nil, webrender.NewErrValidation(err)
		}
	}

	return modelObj, nil
}

// JSONPatchesFromJSONBody pares an array of JSON patch from the HTTP body
func JSONPatchesFromJSONBody(r *http.Request) ([]models.JSONIDPatch, render.Renderer) {
	defer r.Body.Close()
	var jsn []byte
	var err error

	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, webrender.NewErrReadingBody(err)
	}

	// One jsonPath is an array of patches
	// {
	// content:[
	// {
	//   "id": "2f9795fd-fb39-4ea5-af69-14bfa69840aa",
	//   "patches": [
	// 	  { "op": "test", "path": "/a/b/c", "value": "foo" },
	// 	  { "op": "remove", "path": "/a/b/c" },
	//   ]
	// }
	// ]
	// }

	type jsonSlice struct {
		Content []models.JSONIDPatch `json:"content"`
	}

	jsObj := jsonSlice{}
	err = json.Unmarshal(jsn, &jsObj)
	if err != nil {
		return nil, webrender.NewErrParsingJSON(err)
	}

	// if v, ok := modelObj.(models.IValidate); ok {
	// 	who, path, method := WhoFromContext(r), r.URL.Path, r.Method
	// 	if err := v.Validate(who, path, method); err != nil {
	// 		return nil, NewErrValidation(err)
	// 	}
	// }

	return jsObj.Content, nil
}

// IDFromURLQueryString parses resource ID from the URL query string
func IDFromURLQueryString(c *gin.Context) (*datatypes.UUID, render.Renderer) {
	if idstr := c.Param("id"); idstr != "" {

		var err error
		id := datatypes.UUID{}
		id.UUID, err = uuid.FromString(idstr)
		if err != nil {
			return nil, webrender.NewErrURLParameter(err)
		}

		return &id, nil
	}

	return nil, webrender.NewErrURLParameter(errors.New("missing ID in URL query"))
}
