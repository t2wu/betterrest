package routes

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	uuid "github.com/satori/go.uuid"
)

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
func ScopeFromContext(r *http.Request) string {
	var scope string
	item := r.Context().Value(ContextKeyScope)
	if item != nil {
		scope = item.(string)
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
func TokenHoursFromContext(r *http.Request) int {
	tokenHours := -1
	item := r.Context().Value(ContextKeyTokenHours)
	if item != nil {
		tokenHours = item.(int)
	}
	return tokenHours
}

// JSONBodyWithContent for partial unmarshalling
type JSONBodyWithContent struct {
	Content []json.RawMessage
}

// ModelOrModelsFromJSONBody parses JSON body into array of models
// It take care where the case when it is not even an array and there is a "content" in there
func ModelOrModelsFromJSONBody(r *http.Request, typeString string, scope *string) ([]models.IModel, *bool, render.Renderer) {
	defer r.Body.Close()
	var jsn []byte
	var modelObjs []models.IModel
	var err error
	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, nil, NewErrReadingBody(err)
	}

	var jcmodel JSONBodyWithContent

	canHaveCreatedAt := false
	modelObj := models.NewFromTypeString(typeString)
	if _, ok := modelObj.Permissions(models.Admin, scope)["createdAt"]; ok {
		canHaveCreatedAt = true
	}

	err = json.Unmarshal(jsn, &jcmodel)
	if err != nil {
		return nil, nil, NewErrParsingJSON(err)
	}

	if len(jcmodel.Content) == 0 {
		// then it's not a batch insert

		// parsing error...try again with one body
		if canHaveCreatedAt {
			jsnModel2, err := removeCreatedAtFromModel(jsn)
			// ignore error, so if there is no createdAt in the field it will be fine, too
			if err == nil {
				jsn = jsnModel2
			}
		}

		err = json.Unmarshal(jsn, modelObj)
		if err != nil {
			return nil, nil, NewErrParsingJSON(err)
		}

		err := models.Validate.Struct(modelObj)
		if err != nil {
			errs := err.(validator.ValidationErrors)
			return nil, nil, NewErrValidation(errs)
		}

		if v, ok := modelObj.(models.IValidate); ok {
			scope, path, method := ScopeFromContext(r), r.URL.Path, r.Method
			if err := v.Validate(&scope, path, method); err != nil {
				return nil, nil, NewErrValidation(err)
			}
		}

		modelObjs = append(modelObjs, modelObj)
		isBatch := false
		return modelObjs, &isBatch, nil
	}

	for _, jsnModel := range jcmodel.Content {
		if canHaveCreatedAt {
			jsnModel2, err := removeCreatedAtFromModel(jsnModel)
			// ignore error, so if there is no createdAt in the field it will be fine, too
			if err == nil {
				jsnModel = jsnModel2
			}
		}

		modelObj := models.NewFromTypeString(typeString)
		err = json.Unmarshal(jsnModel, modelObj)
		if err != nil {
			return nil, nil, NewErrParsingJSON(err)
		}

		// err := models.Validate.Struct(modelObj)
		// if err != nil {
		// 	errs := err.(validator.ValidationErrors)
		// 	return nil, nil, NewErrValidation(errs)
		// }

		if v, ok := modelObj.(models.IValidate); ok {
			scope, path, method := ScopeFromContext(r), r.URL.Path, r.Method
			if err := v.Validate(&scope, path, method); err != nil {
				return nil, nil, NewErrValidation(err)
			}
		}
		// return nil, nil, NewErrValidation(errors.New("test"))

		modelObjs = append(modelObjs, modelObj)
	}

	isBatch := true
	return modelObjs, &isBatch, nil
}

// ModelsFromJSONBody parses JSON body into array of models
func ModelsFromJSONBody(r *http.Request, typeString string, scope *string) ([]models.IModel, render.Renderer) {
	defer r.Body.Close()
	var jsn []byte
	var modelObjs []models.IModel
	var err error
	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, NewErrReadingBody(err)
	}

	// Previously I don't know about partial marshalling
	// So I had to unmarshal to the array of reflected type
	// And then create an []IModel an assign it one by one.
	// Now I can unmarshal each record one by one from json.RawMessage
	var jcmodel JSONBodyWithContent

	err = json.Unmarshal(jsn, &jcmodel)
	if err != nil {
		return nil, NewErrParsingJSON(err)
	}

	modelTest := models.NewFromTypeString(typeString)
	removeCreated := false
	if _, ok := modelTest.Permissions(models.Admin, scope)["createdAt"]; ok {
		// there is created_at field, so we remove it because it's suppose to be
		// time object and I have int which is not unmarshable
		removeCreated = true
	}

	for _, jsnModel := range jcmodel.Content {
		if removeCreated {
			jsnModel2, err := removeCreatedAtFromModel(jsnModel)
			// ignore error, so if there is no createdAt in the field it will be fine, too
			if err == nil {
				jsnModel = jsnModel2
			}
		}

		modelObj := models.NewFromTypeString(typeString)
		err = json.Unmarshal(jsnModel, modelObj)
		if err != nil {
			return nil, NewErrParsingJSON(err)
		}

		// err := models.Validate.Struct(modelObj)
		// if err != nil {
		// 	errs := err.(validator.ValidationErrors)
		// 	return nil, NewErrValidation(errs)
		// }

		if v, ok := modelObj.(models.IValidate); ok {
			scope, path, method := ScopeFromContext(r), r.URL.Path, r.Method
			if err := v.Validate(&scope, path, method); err != nil {
				return nil, NewErrValidation(err)
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
func ModelFromJSONBody(r *http.Request, typeString string, scope *string) (models.IModel, render.Renderer) {
	defer r.Body.Close()
	var jsn []byte
	var err error

	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, NewErrReadingBody(err)
	}

	modelObj := models.NewFromTypeString(typeString)

	if _, ok := modelObj.Permissions(models.Admin, scope)["createdAt"]; ok {
		// there is created_at field, so we remove it because it's suppose to be
		// time object and I have int which is not unmarshable
		jsn2, err := removeCreatedAtFromModel(jsn)
		// ignore error, so if there is no createdAt in the field it will be fine, too
		if err == nil {
			jsn = jsn2
		}
	}

	err = json.Unmarshal(jsn, modelObj)
	if err != nil {
		return nil, NewErrParsingJSON(err)
	}

	if v, ok := modelObj.(models.IValidate); ok {
		scope, path, method := ScopeFromContext(r), r.URL.Path, r.Method
		if err := v.Validate(&scope, path, method); err != nil {
			return nil, NewErrValidation(err)
		}
	}

	return modelObj, nil
}

// IDFromURLQueryString parses resource ID from the URL query string
func IDFromURLQueryString(c *gin.Context) (*datatypes.UUID, render.Renderer) {
	if idstr := c.Param("id"); idstr != "" {

		var err error
		id := datatypes.UUID{}
		id.UUID, err = uuid.FromString(idstr)
		if err != nil {
			return nil, NewErrURLParameter(err)
		}

		return &id, nil
	}

	return nil, NewErrURLParameter(errors.New("missing ID in URL query"))
}
