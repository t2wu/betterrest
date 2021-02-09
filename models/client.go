package models

import (
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/datatypes"
)

// Client is the program that makes request to the API
// So iOS would be one client, android would be another
// Any website making API request would have its own client ID
// This needs to be inserted into db beforehand.
// So we can validate the app making the request. Any such app
// has the permission to create the user
type Client struct {
	gorm.Model  // Includes ID, CreatedAt, UpdatedAt, DeletedAt
	Name        string
	Secret      string `gorm:"not null" json:"-"`
	RedirectURI string // TODO: RedirectURI can be multiples
}

// Who is the information about the client or the user
type Who struct {
	Client *Client
	Oid    *datatypes.UUID // ownerid
	Scope  *string
}
