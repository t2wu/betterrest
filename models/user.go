package models

import (
	"betterrest/libs/utils"
)

// User model
type User struct {
	BaseModel                // Includes ID, CreatedAt, UpdatedAt, DeletedAt
	FirstName    string      `gorm:"not null" json:"firstName"`
	LastName     string      `gorm:"not null" json:"lastName"`
	MiddleName   string      `json:"middleName"`
	Email        string      `gorm:"not null;type:varchar(100);unique_index" json:"email"`
	Password     string      `gorm:"-"` // Unencrypted password, never store or translate to JSON
	PasswordHash string      `json:"-"` // Encrpyted password
	Ownerships   []Ownership `gorm:"many2many:user_ownerships;" json:"-"`
}

// Permissions return permission for the role given
func (u *User) Permissions(r UserRole) utils.JSONFields {
	if r == Admin {
		return utils.JSONFields{
			"id":         nil,
			"firstName":  nil,
			"lastName":   nil,
			"middleName": nil,
			"email":      nil,
		}
	}

	// Guest
	return utils.JSONFields{
		"id":         nil,
		"firstName":  nil,
		"lastName":   nil,
		"middleName": nil,
		"email":      nil,
	}
}

// AppendOwnership is bogus for user. Just to satisfy the interface
func (u *User) AppendOwnership(g Ownership) {
	// does nothing
}

// Rey's
// type User struct {
// 	*Super
// 	Active       int64 // this is for when deleted it's set to some number
// 	CreateDate   int64
// 	ID           int64
// 	LastModified int64
// 	Logins       int64 // not used
// 	Mobile       string
// 	PWD          string
// 	MCID		     	string						`gorm:"column:mcid"`
// 	Token01      string //why 3 of them I have no idea..
// 	Token02      string
// 	Token03      string
// 	Name      	 string
// }
