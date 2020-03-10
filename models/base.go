package models

import (
	"betterrest/libs/utils"
	"reflect"
	"time"

	"github.com/jinzhu/gorm"
	uuid "github.com/satori/go.uuid"
)

// UserRole type with enum
type UserRole int

const (
	// Admin is admin UserRole
	Admin UserRole = 0
	// Guest is guest UserRole (screw go-lint man)
	Guest UserRole = 1
)

// ValueFromStringField obtain Value from a string field
func ValueFromStringField(model interface{}, fieldName string) string {
	ps := reflect.ValueOf(&model)
	return ps.Elem().FieldByName(fieldName).Interface().(string) // I should have more error checking here though
}

// BaseModel is the base class domain model which has standard ID
type BaseModel struct {
	// gorm.Model // Includes ID, CreatedAt, UpdatedAt, DeletedAt
	ID        uint      `gorm:"primary_key;AUTO_INCREMENT" json:"id"` //`json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`
}

// IModel is the interface for all domain models
type IModel interface {
	Permissions(r UserRole) utils.JSONFields
	AppendOwnership(g Ownership)
}

/*
 * Every OwnershipModel subclasses BaseModel so it has a Ownership field.
 * Every OwnershipModel comforms to the interface DomainModel and therefor has a ToJSON()
 * so it is able to be serialized to JSON according to some permissions
 */

// OwnershipModel has a ownership association
type OwnershipModel struct {
	BaseModel
	Ownerships []Ownership `json:"-"` // store ownership id
}

/*
https://medium.com/@the.hasham.ali/how-to-use-uuid-key-type-with-gorm-cc00d4ec7100
*/

// UUIDBaseModel is the base model with UUID as key
type UUIDBaseModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`
}

// BeforeCreate will set a UUID rather than numeric ID.
func (base *UUIDBaseModel) BeforeCreate(scope *gorm.Scope) error {
	uuid := uuid.NewV4()

	return scope.SetColumn("ID", uuid)
}
