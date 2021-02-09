package tools

import (
	"encoding/json"

	"github.com/t2wu/betterrest/libs/utils/jsontransform"
	"github.com/t2wu/betterrest/models"
)

// ToJSON pack json into this struct and the role
func ToJSON(typeString string, v models.IModel, r models.UserRole, who models.Who) ([]byte, error) {
	var j []byte
	var err error

	if j, err = json.Marshal(v); err != nil {
		return j, err
	}

	if modelObjPerm, ok := v.(models.IHasPermissions); ok {
		fields := modelObjPerm.Permissions(r, who.Scope)
		return jsontransform.Transform(j, &fields)
	}

	panic("Haven't implement default permission function yet")
	// return jsontransform.Transform(j, &fields)
}

// FromJSON unpacks json into this struct
func FromJSON(v interface{}, j []byte) error {
	return json.Unmarshal(j, v)
}
