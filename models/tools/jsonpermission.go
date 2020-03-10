package tools

import (
	"betterrest/libs/utils"
	"betterrest/models"
	"encoding/json"
)

// ToJSON pack json into this struct and the role
func ToJSON(typeString string, v models.IModel, r models.UserRole) ([]byte, error) {
	var j []byte
	var err error

	if j, err = json.Marshal(v); err != nil {
		return j, err
	}

	fields := v.Permissions(r)
	// fields := typeregistry.PermissionRegistry[typeString](v, r)
	return utils.JSONCherryPickFields(j, &fields)
}

// FromJSON unpacks json into this struct
func FromJSON(v interface{}, j []byte) error {
	return json.Unmarshal(j, v)
}
