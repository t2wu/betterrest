package tools

import (
	"encoding/json"

	"github.com/t2wu/betterrest/libs/utils/jsontransform"
	"github.com/t2wu/betterrest/models"
)

// ToJSON pack json into this struct and the role
func ToJSON(typeString string, v models.IModel, r models.UserRole) ([]byte, error) {
	var j []byte
	var err error

	if j, err = json.Marshal(v); err != nil {
		return j, err
	}

	fields := v.Permissions(r)
	return jsontransform.Transform(j, &fields)
}

// FromJSON unpacks json into this struct
func FromJSON(v interface{}, j []byte) error {
	return json.Unmarshal(j, v)
}
