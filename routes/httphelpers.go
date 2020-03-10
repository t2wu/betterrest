package routes

import (
	"betterrest/models"
	"betterrest/typeregistry"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
)

// OwnerIDFromContext parses JSON body into array of models
func OwnerIDFromContext(r *http.Request) uint {
	var ownerID uint
	if r.Context().Value(contextKeyOwnerID) != nil {
		ownerID = r.Context().Value(contextKeyOwnerID).(uint)
	}
	return ownerID
}

// ModelsFromJSONBody parses JSON body into array of models
func ModelsFromJSONBody(r *http.Request, typeString string) ([]models.IModel, *ErrResponse) {
	defer r.Body.Close()
	var jsn []byte
	var modelObjs []models.IModel
	var err error
	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, ErrReadingBody
	}

	modelObjs, err = typeregistry.NewSliceFromJSONRegistry[typeString](jsn)
	if err != nil {
		return nil, ErrParsingJSON
	}

	return modelObjs, nil
}

// ModelFromJSONBody parses JSON body into a model
func ModelFromJSONBody(r *http.Request, typeString string) (models.IModel, *ErrResponse) {
	defer r.Body.Close()
	var jsn []byte
	var modelObj models.IModel
	var err error

	if jsn, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, ErrReadingBody
	}

	modelObj, err = typeregistry.NewFromJSONRegistry[typeString](jsn)
	if err != nil {
		return nil, ErrParsingJSON
	}

	return modelObj, nil
}

// IDFromURLQueryString parses resource ID from the URL query string
func IDFromURLQueryString(r *http.Request) (id uint64, httperr *ErrResponse) {
	if idstr := chi.URLParam(r, "id"); idstr != "" {
		var err error
		if id, err = strconv.ParseUint(idstr, 10, 64); err != nil {
			return 0, ErrParameter
		}

		return id, nil
	}

	return 0, ErrParameter
}
