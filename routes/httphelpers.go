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
	uuid "github.com/satori/go.uuid"
)

// OwnerIDFromContext parses JSON body into array of models
func OwnerIDFromContext(r *http.Request) *datatypes.UUID {
	var ownerID *datatypes.UUID
	if r.Context().Value(contextKeyOwnerID) != nil {
		ownerID = r.Context().Value(contextKeyOwnerID).(*datatypes.UUID)
	}
	return ownerID
}

// ModelsFromJSONBody parses JSON body into array of models
func ModelsFromJSONBody(r *http.Request, typeString string) ([]models.IModel, render.Renderer) {
	defer r.Body.Close()
	var jsn []byte
	var modelObjs []models.IModel
	var err error
	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, NewErrReadingBody(err)
	}

	modelObjs, err = models.NewSliceStructFromTypeStringAndJSON(typeString, jsn)
	if err != nil {
		return nil, NewErrParsingJSON(err)
	}

	if err != nil {
		return nil, NewErrParsingJSON(err)
	}

	return modelObjs, nil
}

// ModelFromJSONBody parses JSON body into a model
// FIXME:
// Validation should not be done here because empty field does not pass validation,
// but sometimes we need empty fields such as patch
func ModelFromJSONBody(r *http.Request, typeString string) (models.IModel, render.Renderer) {
	defer r.Body.Close()
	var jsn []byte
	var err error

	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, NewErrReadingBody(err)
	}

	modelObj := models.NewFromTypeString(typeString)
	err = json.Unmarshal(jsn, modelObj)
	if err != nil {
		return nil, NewErrParsingJSON(err)
	}

	if v, ok := modelObj.(models.IValidate); ok {
		if err := v.Validate(); err != nil {
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
