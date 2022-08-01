package routes

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/t2wu/betterrest/hook/rest"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/utils/jsontrans"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/betterrest/registry"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
	uuid "github.com/satori/go.uuid"
)

// JSONBodyWithContent for partial unmarshalling
type JSONBodyWithContent struct {
	Content []json.RawMessage
}

// ModelOrModelsFromJSONBody parses JSON body into array of mdl
// It take care where the case when it is not even an array and there is a "content" in there
func ModelOrModelsFromJSONBody(r *http.Request, typeString string, who mdlutil.UserIDFetchable) ([]mdl.IModel, *bool, render.Renderer) {
	defer r.Body.Close()
	var jsn []byte
	var modelObjs []mdl.IModel
	var err error
	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, nil, webrender.NewErrReadingBody(err)
	}

	var jcmodel JSONBodyWithContent

	modelObj := registry.NewFromTypeString(typeString)

	needTransform := false
	var fields jsontrans.JSONFields
	if modelObjPerm, ok := modelObj.(mdlutil.IHasPermissions); ok {
		_, fields = modelObjPerm.Permissions(userrole.UserRoleAdmin, who)
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

		if err := mdl.ValidateModel(modelObj); err != nil {
			return nil, nil, webrender.NewErrValidation(err)
		}

		if v, ok := modelObj.(mdlutil.IValidate); ok {
			who := WhoFromContext(r)
			http := mdlutil.HTTP{Endpoint: r.URL.Path, Op: rest.HTTPMethodToRESTOp(r.Method)}
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

		modelObj := registry.NewFromTypeString(typeString)
		err = json.Unmarshal(jsnModel, modelObj)
		if err != nil {
			return nil, nil, webrender.NewErrParsingJSON(err)
		}

		if err := mdl.ValidateModel(modelObj); err != nil {
			return nil, nil, webrender.NewErrValidation(err)
		}

		if v, ok := modelObj.(mdlutil.IValidate); ok {
			http := mdlutil.HTTP{Endpoint: r.URL.Path, Op: rest.HTTPMethodToRESTOp(r.Method)}
			if err := v.Validate(who, http); err != nil {
				return nil, nil, webrender.NewErrValidation(err)
			}
		}

		modelObjs = append(modelObjs, modelObj)
	}

	isBatch := true
	return modelObjs, &isBatch, nil
}

// ModelsFromJSONBody parses JSON body into array of mdl
func ModelsFromJSONBody(r *http.Request, typeString string, who mdlutil.UserIDFetchable) ([]mdl.IModel, render.Renderer) {
	defer r.Body.Close()
	var jsn []byte
	var modelObjs []mdl.IModel
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

	modelTest := registry.NewFromTypeString(typeString)
	needTransform := false
	var fields jsontrans.JSONFields
	if modelObjPerm, ok := modelTest.(mdlutil.IHasPermissions); ok {
		_, fields = modelObjPerm.Permissions(userrole.UserRoleAdmin, who)
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

		modelObj := registry.NewFromTypeString(typeString)
		err = json.Unmarshal(jsnModel, modelObj)
		if err != nil {
			return nil, webrender.NewErrParsingJSON(err)
		}

		if err := mdl.ValidateModel(modelObj); err != nil {
			return nil, webrender.NewErrValidation(err)
		}

		if v, ok := modelObj.(mdlutil.IValidate); ok {
			who := WhoFromContext(r)
			http := mdlutil.HTTP{Endpoint: r.URL.Path, Op: rest.HTTPMethodToRESTOp(r.Method)}
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
func ModelFromJSONBody(r *http.Request, typeString string, who mdlutil.UserIDFetchable) (mdl.IModel, render.Renderer) {
	defer r.Body.Close()
	var jsn []byte
	var err error

	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, webrender.NewErrReadingBody(err)
	}

	modelObj := registry.NewFromTypeString(typeString)

	if modelObjPerm, ok := modelObj.(mdlutil.IHasPermissions); ok {
		// removeCreated := false
		_, fields := modelObjPerm.Permissions(userrole.UserRoleAdmin, who)

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

	if err := mdl.ValidateModel(modelObj); err != nil {
		return nil, webrender.NewErrValidation(err)
	}

	if v, ok := modelObj.(mdlutil.IValidate); ok {
		who := WhoFromContext(r)
		http := mdlutil.HTTP{Endpoint: r.URL.Path, Op: rest.HTTPMethodToRESTOp(r.Method)}
		if err := v.Validate(who, http); err != nil {
			return nil, webrender.NewErrValidation(err)
		}
	}

	return modelObj, nil
}

// JSONPatchesFromJSONBody pares an array of JSON patch from the HTTP body
func JSONPatchesFromJSONBody(r *http.Request) ([]mdlutil.JSONIDPatch, render.Renderer) {
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
		Content []mdlutil.JSONIDPatch `json:"content"`
	}

	jsObj := jsonSlice{}
	err = json.Unmarshal(jsn, &jsObj)
	if err != nil {
		return nil, webrender.NewErrParsingJSON(err)
	}

	// if v, ok := modelObj.(mdlutil.IValidate); ok {
	// 	who, path, method := WhoFromContext(r), r.URL.Path, r.Method
	// 	if err := v.Validate(who, path, method); err != nil {
	// 		return nil, NewErrValidation(err)
	// 	}
	// }

	return jsObj.Content, nil
}

// IDFromURLQueryString parses resource ID from the URL query string
func IDFromURLQueryString(c *gin.Context) (*datatype.UUID, render.Renderer) {
	if idstr := c.Param("id"); idstr != "" {

		var err error
		id := datatype.UUID{}
		id.UUID, err = uuid.FromString(idstr)
		if err != nil {
			return nil, webrender.NewErrURLParameter(err)
		}

		return &id, nil
	}

	return nil, webrender.NewErrURLParameter(errors.New("missing ID in URL query"))
}
