package models

import (
	"betterrest/libs/utils"

	"github.com/jinzhu/gorm"
)

// Ownership is about the ownership of people
// Same concept as Unix's user/ownership/others. Except we don't have others.
// And other models can have multiple ownership, so users which are in different
// ownership may see different field
type Ownership struct {
	gorm.Model // Includes ID, CreatedAt, UpdatedAt, DeletedAt
	// How to define permission? read, write of every field is what we're concerned here
	// So we need to store

	Role    UserRole // an int
	Users   []User   `gorm:"many2many:user_ownerships;"`
	Classes []Class  `gorm:"many2many:class_ownerships;"`
}

// Permissions return permission for the role given
func (g *Ownership) Permissions(r UserRole) utils.JSONFields {
	return utils.JSONFields{} // actually irrelevant
}
