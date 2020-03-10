package models

import (
	"betterrest/libs/utils"
	"time"
)

// Class model
type Class struct { // interface DomainModel
	OwnershipModel
	Name string `json:"name"`

	Ownerships []Ownership `gorm:"many2many:Class_ownerships;" json:"-"` // store ownership id
}

// Permissions return permission for the role given
func (ro *Class) Permissions(r UserRole) utils.JSONFields {
	if r == Admin {
		return utils.JSONFields{
			"id":        nil,
			"name":      nil,
			"createdAt": createdAtToUnixTime,
		}
	}

	// Guest
	return utils.JSONFields{
		"id":        nil,
		"name":      nil,
		"createdAt": createdAtToUnixTime,
	}
}

// AppendOwnership appends a ownership in the Ownerships field (association)
func (ro *Class) AppendOwnership(g Ownership) {
	ro.Ownerships = append(ro.Ownerships, g)
}

func createdAtToUnixTime(createdAt interface{}) interface{} {
	t, _ := time.Parse(time.RFC3339, createdAt.(string))
	var v interface{}
	v = t.Unix()
	return v
}
