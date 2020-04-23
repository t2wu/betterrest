package models

import (
	"time"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/utils/jsontransform"

	"github.com/asaskevich/govalidator"
	"github.com/jinzhu/gorm"
)

// UserRole type with enum
type UserRole int

const (
	// Admin is admin UserRole
	Admin UserRole = 0
	// Guest is guest UserRole (screw go-lint man)
	Guest UserRole = 1
)

// BaseModel is the base class domain model which has standard ID
type BaseModel struct {
	ID        *datatypes.UUID `gorm:"type:binary(16);primary_key;" json:"id"`
	CreatedAt time.Time       `sql:"index" json:"createdAt"`
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`

	// Ownership with the most previledged permission can delete the device and see every field.
	// So there can be an ownership number, say 3, and that maps to a permission type
	// (within the ownership table), say "admin ownership" (int 0), and whoever is a member of ownership
	// 3 thus has the admin priviledge
	// The "guest" of model "device" and "guest" of model of "scene" is vastly different, because
	// the fields are different, and specific field permission is based on priviledge -> field mapping
	// defined when getting permission()
	// Ownership []int64
}

// GetID Get the ID field of the model (useful when using interface)
func (b *BaseModel) GetID() *datatypes.UUID {
	return b.ID
}

// SetID Set the ID field of the model (useful when using interface)
func (b *BaseModel) SetID(id *datatypes.UUID) {
	b.ID = id
}

// BeforeCreate sets a UUID if no ID is set
// (this is Gorm's hookpoint)
func (b *BaseModel) BeforeCreate(scope *gorm.Scope) error {
	if b.ID == nil {
		uuid := datatypes.NewUUID()
		return scope.SetColumn("ID", uuid)
	}

	return nil
}

// Validate validates the model
func (b *BaseModel) Validate() error {
	if ok, err := govalidator.ValidateStruct(b); ok == false && err != nil {
		return err
	}
	return nil
}

// IModel is the interface for all domain models
type IModel interface {
	Permissions(r UserRole) jsontransform.JSONFields

	// The following two avoids having to use reflection to access ID
	GetID() *datatypes.UUID
	SetID(id *datatypes.UUID)
}

// ModelCargo is payload between hookpoints
type ModelCargo struct {
	Payload interface{}
}

// IBeforeInsert supports method to be called before data is inserted (created) into the database
type IBeforeInsert interface {
	BeforeInsertDB(db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *ModelCargo) error
}

// IBeforeUpdate supports method to be called before data is updated in the database
type IBeforeUpdate interface {
	BeforeUpdateDB(db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *ModelCargo) error
}

// IBeforePatch supports method to be called before data is patched in the database
type IBeforePatch interface {
	BeforePatchDB(db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *ModelCargo) error
}

// IBeforeDelete supports method to be called before data is deleted from the database
type IBeforeDelete interface {
	BeforeDeleteDB(db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *ModelCargo) error
}

// IAfterInsert supports method to be called after data is inserted (created) into the database
type IAfterInsert interface {
	AfterInsertDB(db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *ModelCargo) error
}

// IAfterUpdate supports method to be called after data is updated in the database
type IAfterUpdate interface {
	AfterUpdateDB(db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *ModelCargo) error
}

// IAfterPatch supports method to be called before data is patched in the database
type IAfterPatch interface {
	AfterPatchDB(db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *ModelCargo) error
}

// IAfterDelete supports method to be called after data is deleted from the database
type IAfterDelete interface {
	AfterDeleteDB(db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *ModelCargo) error
}

// IValidate supports validation with govalidator
type IValidate interface {
	Validate() error
}

// ------------------------------------

// IRole is what IRole tables should satisfy.
type IRole interface {
	GetRole() UserRole
	SetRole(UserRole)
}

// OwnershipModelBase has a role
type OwnershipModelBase struct {
	gorm.Model          // uses standard int id (cuz I started with it and it works)
	Role       UserRole // an int
}

// GetRole gets the role field of the model, comforms to IRole
func (o *OwnershipModelBase) GetRole() UserRole {
	return o.Role
}

// SetRole sets the role field of the model, comforms to IRole
func (o *OwnershipModelBase) SetRole(r UserRole) {
	o.Role = r
}

// ------------------------------------
