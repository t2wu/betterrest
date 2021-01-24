package models

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/utils/jsontransform"

	"github.com/asaskevich/govalidator"
	"github.com/jinzhu/gorm"
)

// UserRole type with enum
type UserRole int

const (
	// Negatives are not stored in DB
	// UserRoleAny not for value in db, but for permission where any is fine (link table)
	UserRoleAny UserRole = -2

	// Invalid for this resource
	Invalid UserRole = -1

	// Admin is admin UserRole
	Admin UserRole = 0

	// Guest is guest UserRole (screw go-lint man)
	Guest UserRole = 1

	// Public to all (global object)
	Public UserRole = 2
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
	Permissions(r UserRole, scope *string) jsontransform.JSONFields
	// CherryPickFields(r UserRole) jsontransform.JSONFields

	// The following two avoids having to use reflection to access ID
	GetID() *datatypes.UUID
	SetID(id *datatypes.UUID)
}

// Inside content is an array of JSONIDPatch
// {
// content:[
// {
//   "id": "2f9795fd-fb39-4ea5-af69-14bfa69840aa",
//   "patches": [
// 	  { "op": "test", "path": "/a/b/c", "value": "foo" },
// 	  { "op": "remove", "path": "/a/b/c" },
//   ]
// }
// ]
// }

// JSONIDPatch is the stuff inside "content" for PatchMany operation
type JSONIDPatch struct {
	ID    *datatypes.UUID `json:"id"`
	Patch json.RawMessage `json:"patch"` // json.RawMessage is actually just typedefed to []byte
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
	return strcase.SnakeCase(typeName)
}

// GetOrganizationTableName if comforms to IHasOrganizationLink
func GetOrganizationTableName(modelObj IHasOrganizationLink) string {
	if m, ok := reflect.New(modelObj.OrganizationType()).Interface().(IHasTableName); ok {
		return m.TableName()
	}

	typeName := modelObj.OrganizationType().Name()
	return strcase.SnakeCase(typeName)
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

// HookPointData is the data send to single model hookpoints
type HookPointData struct {
	// DB handle
	DB *gorm.DB
	// OID is owner ID, the user accessing the API right now
	// Not available in BeforeLogin
	OID *datatypes.UUID
	// Scope included in the token who is accessing right now
	Scope *string
	// TypeString is the typeString (model string) of this model
	TypeString string
	// Cargo between Before and After hookpoints (not used in IAfterRead since there is no IBeforeRead.)
	Cargo *ModelCargo
	// Role of this user in relation to this data, only available during read
	Role *UserRole
}

// IBeforeLogin has a function that is a hookpoint for actions after login but before marshalling
type IBeforeLogin interface {
	BeforeLogin(hpdata HookPointData) error
}

// IAfterLogin has a function that is a hookpoint for actions after login but before marshalling
type IAfterLogin interface {
	AfterLogin(hpdata HookPointData, payload map[string]interface{}) (map[string]interface{}, error)
}

// IAfterLoginFailed has a function that is a hookpoint for actions after login but before marshalling
type IAfterLoginFailed interface {
	AfterLoginFailed(hpdata HookPointData) error
}

// IBeforeCreate supports method to be called before data is inserted (created) into the database
type IBeforeCreate interface {
	BeforeInsertDB(hpdata HookPointData) error
}

// IBeforeUpdate supports method to be called before data is updated in the database
type IBeforeUpdate interface {
	BeforeUpdateDB(hpdata HookPointData) error
}

// IBeforePatch supports method to be called before data is patched in the database
type IBeforePatch interface {
	BeforePatchDB(hpdata HookPointData) error
}

// IBeforeDelete supports method to be called before data is deleted from the database
type IBeforeDelete interface {
	BeforeDeleteDB(hpdata HookPointData) error
}

// IAfterRead supports method to be called after data is read from the database
type IAfterRead interface {
	AfterReadDB(hpdata HookPointData) error
}

// IAfterCreate supports method to be called after data is inserted (created) into the database
type IAfterCreate interface {
	AfterInsertDB(hpdata HookPointData) error
}

// IAfterUpdate supports method to be called after data is updated in the database
type IAfterUpdate interface {
	AfterUpdateDB(hpdata HookPointData) error
}

// IAfterPatch supports method to be called before data is patched in the database
type IAfterPatch interface {
	AfterPatchDB(hpdata HookPointData) error
}

// IAfterDelete supports method to be called after data is deleted from the database
type IAfterDelete interface {
	AfterDeleteDB(hpdata HookPointData) error
}

// IValidate supports validation with govalidator
type IValidate interface {
	Validate(scope *string, endpoint string, method string) error
}

// IBeforePasswordUpdate supports method to be called before data is updated in the database
type IBeforePasswordUpdate interface {
	BeforePasswordUpdateDB(hpdata HookPointData) error
}

// IAfterPasswordUpdate supports method to be called after data is updated in the database
type IAfterPasswordUpdate interface {
	AfterPasswordUpdateDB(hpdata HookPointData) error
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

	GetID() *datatypes.UUID
	SetID(*datatypes.UUID)
}

// OwnershipModelBase has a role
type OwnershipModelBase struct {
	ID *datatypes.UUID `gorm:"type:uuid;primary_key;" json:"id"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`

	Role UserRole `json:"role"` // an int
}

// BeforeCreate sets a UUID if no ID is set
// (this is Gorm's hookpoint)
func (o *OwnershipModelBase) BeforeCreate(scope *gorm.Scope) error {
	if o.ID == nil {
		uuid := datatypes.NewUUID()
		return scope.SetColumn("ID", uuid)
	}

	return nil
}

// GetRole gets the role field of the model, comforms to IOwnership
func (o *OwnershipModelBase) GetRole() UserRole {
	return o.Role
}

// SetRole sets the role field of the model, comforms to IOwnership
func (o *OwnershipModelBase) SetRole(r UserRole) {
	o.Role = r
}

// GetID Get the ID field of the model (useful when using interface)
func (o *OwnershipModelBase) GetID() *datatypes.UUID {
	return o.ID
}

// SetID Set the ID field of the model (useful when using interface)
func (o *OwnershipModelBase) SetID(id *datatypes.UUID) {
	o.ID = id
}

// OwnershipModelWithIDBase is one with ID, if you don't need unique index
// for userID and modelID (if you don't expose the link table via LinkTableMapper)
// You can use this.
type OwnershipModelWithIDBase struct {
	OwnershipModelBase

	UserID  *datatypes.UUID `json:"userID"` // I guess the user's table has to be named "User" then.
	ModelID *datatypes.UUID `json:"modelID"`
}

// GetUserID gets the user id of the model, comforms to IOwnership
func (o *OwnershipModelWithIDBase) GetUserID() *datatypes.UUID {
	return o.UserID
	// v := reflect.ValueOf(o)
	// return reflect.Indirect(v).FieldByName("ID").Interface().(*datatypes.UUID)
}

// SetUserID sets the user id of the model, comforms to IOwnership
func (o *OwnershipModelWithIDBase) SetUserID(id *datatypes.UUID) {
	o.UserID = id
}

// SetModelID sets the id of the model, comforms to IOwnership
func (o *OwnershipModelWithIDBase) SetModelID(id *datatypes.UUID) {
	o.ModelID = id
}

// GetModelID gets the id of the model, comforms to IOwnership
func (o *OwnershipModelWithIDBase) GetModelID() *datatypes.UUID {
	return o.ModelID
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

		tableName = strcase.SnakeCase(tableName)
	}

	// If it's a pointer, get rid of "*"
	if strings.HasPrefix(tableName, "*") {
		tableName = tableName[1:]
	}

	return tableName
}

// GetTableNameFromTypeString get table name from typeString
func GetTableNameFromTypeString(typeString string) string {
	model := NewFromTypeString(typeString)
	return GetTableNameFromIModel(model)
}

// GetTableNameFromType get table name from the model reflect.type
func GetTableNameFromType(typ reflect.Type) string {
	model := reflect.New(typ).Interface().(IModel)
	return GetTableNameFromIModel(model)
}
