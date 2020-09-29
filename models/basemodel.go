package models

import (
	"reflect"
	"strings"
	"time"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/utils/jsontransform"
	"github.com/t2wu/betterrest/libs/utils/letters"

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
	// For MySQL
	// ID        *datatypes.UUID `gorm:"type:binary(16);primary_key;" json:"id"`

	// For Postgres
	ID        *datatypes.UUID `gorm:"type:uuid;primary_key;" json:"id"`
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
	// CherryPickFields(r UserRole) jsontransform.JSONFields

	// The following two avoids having to use reflection to access ID
	GetID() *datatypes.UUID
	SetID(id *datatypes.UUID)
}

// IHasOwnershipLink has a function that returns the ownership table
// usable for OwnershipMapper
type IHasOwnershipLink interface {
	OwnershipType() reflect.Type
}

// IHasOrganizationLink has a function that returns the organization table
// usable for OrganizationMapper
type IHasOrganizationLink interface {
	OrganizationType() reflect.Type
	GetOrganizationID() *datatypes.UUID
	GetOrganizationIDFieldName() string
}

// GetJoinTableName if comforms to IHasOwnershipLink
func GetJoinTableName(modelObj IHasOwnershipLink) string {
	if m, ok := reflect.New(modelObj.OwnershipType()).Interface().(IHasTableName); ok {
		return m.TableName()
	}

	typeName := modelObj.OwnershipType().Name()
	return letters.PascalCaseToSnakeCase(typeName)
}

// GetOrganizationTableName if comforms to IHasOrganizationLink
func GetOrganizationTableName(modelObj IHasOrganizationLink) string {
	if m, ok := reflect.New(modelObj.OrganizationType()).Interface().(IHasTableName); ok {
		return m.TableName()
	}

	typeName := modelObj.OrganizationType().Name()
	return letters.PascalCaseToSnakeCase(typeName)
}

// IDoRealDelete is an interface to customize specification for real db delete
type IDoRealDelete interface {
	DoRealDelete() bool
}

// IGuardAPIEntry supports method which guard access to API based on scope
type IGuardAPIEntry interface {
	GuardAPIEntry(scope *string, endpoint string, method string) bool
}

// ModelCargo is payload between hookpoints
type ModelCargo struct {
	Payload interface{}
}

// IBeforeInsert supports method to be called before data is inserted (created) into the database
type IBeforeInsert interface {
	BeforeInsertDB(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *ModelCargo) error
}

// IBeforeUpdate supports method to be called before data is updated in the database
type IBeforeUpdate interface {
	BeforeUpdateDB(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *ModelCargo) error
}

// IBeforePatch supports method to be called before data is patched in the database
type IBeforePatch interface {
	BeforePatchDB(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *ModelCargo) error
}

// IBeforeDelete supports method to be called before data is deleted from the database
type IBeforeDelete interface {
	BeforeDeleteDB(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *ModelCargo) error
}

// IAfterRead supports method to be called after data is read from the database
type IAfterRead interface {
	AfterReadDB(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, role *UserRole) error
}

// IAfterInsert supports method to be called after data is inserted (created) into the database
type IAfterInsert interface {
	AfterInsertDB(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *ModelCargo) error
}

// IAfterUpdate supports method to be called after data is updated in the database
type IAfterUpdate interface {
	AfterUpdateDB(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *ModelCargo) error
}

// IAfterPatch supports method to be called before data is patched in the database
type IAfterPatch interface {
	AfterPatchDB(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *ModelCargo) error
}

// IAfterDelete supports method to be called after data is deleted from the database
type IAfterDelete interface {
	AfterDeleteDB(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *ModelCargo) error
}

// IValidate supports validation with govalidator
type IValidate interface {
	Validate(scope *string, endpoint string, method string) error
}

// ------------------------------------

// IOwnership is what OwnershipModelBase tables should satisfy.
type IOwnership interface {
	GetRole() UserRole
	SetRole(UserRole)

	GetUserID() *datatypes.UUID
	SetUserID(*datatypes.UUID)

	GetModelID() *datatypes.UUID
	SetModelID(*datatypes.UUID)
}

// OwnershipModelBase has a role
type OwnershipModelBase struct {
	gorm.Model // uses standard int id (cuz I started with it and it works)

	UserID  *datatypes.UUID
	ModelID *datatypes.UUID

	Role UserRole // an int
}

// GetUserID gets the user id of the model, comforms to IOwnership
func (o *OwnershipModelBase) GetUserID() *datatypes.UUID {
	return o.UserID
	// v := reflect.ValueOf(o)
	// return reflect.Indirect(v).FieldByName("ID").Interface().(*datatypes.UUID)
}

// SetUserID sets the user id of the model, comforms to IOwnership
func (o *OwnershipModelBase) SetUserID(id *datatypes.UUID) {
	o.UserID = id
}

// SetModelID sets the id of the model, comforms to IOwnership
func (o *OwnershipModelBase) SetModelID(id *datatypes.UUID) {
	o.ModelID = id
}

// GetModelID gets the id of the model, comforms to IOwnership
func (o *OwnershipModelBase) GetModelID() *datatypes.UUID {
	return o.ModelID
}

// GetRole gets the role field of the model, comforms to IOwnership
func (o *OwnershipModelBase) GetRole() UserRole {
	return o.Role
}

// SetRole sets the role field of the model, comforms to IOwnership
func (o *OwnershipModelBase) SetRole(r UserRole) {
	o.Role = r
}

// ---------------

// IHasTableName we know if there is Gorm's defined custom TableName
type IHasTableName interface {
	TableName() string
}

// GetTableNameFromIModel gets table name from an IModel
func GetTableNameFromIModel(model IModel) string {
	var tableName string
	if m, ok := model.(IHasTableName); ok {
		tableName = m.TableName()
	} else {
		tableName = reflect.TypeOf(model).String()
		// If it is something like "models.XXX", we only want the stuff ater "."
		if strings.Contains(tableName, ".") {
			tableName = strings.Split(tableName, ".")[1]
		}

		tableName = letters.PascalCaseToSnakeCase(tableName)
	}

	// If it's a pointer, get rid of "*"
	if strings.HasPrefix(tableName, "*") {
		tableName = tableName[1:]
	}

	return tableName
}
