package mdlutil

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/hook/rest"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/utils/jsontrans"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"
)

// JSONIDPatch is the stuff inside "content" for PatchMany operation
type JSONIDPatch struct {
	ID    *datatype.UUID  `json:"id"`
	Patch json.RawMessage `json:"patch"` // json.RawMessage is actually just typedefed to []byte
}

// ----------------------------

// IDoRealDelete is an interface to customize specification for real db delete
type IDoRealDelete interface {
	DoRealDelete() bool
}

// ------------------------------------------------------------------------------------
// Old Rest Op
// CRUPDOp designates the type of operations for BeforeCRUPD and AfterCRUPD hookpoints
type CRUPDOp int

const (
	CRUPDOpOther CRUPDOp = iota // should not be used
	CRUPDOpRead
	CRUPDOpCreate
	CRUPDOpUpdate
	CRUPDOpPatch
	CRUPDOpDelete
)

// HTTP stores HTTP request information
type HTTP struct {
	Endpoint string
	Op       rest.Op
}

func HTTPMethodToRESTOp(method string) rest.Op {
	switch method {
	case "GET":
		return rest.OpRead
	case "POST":
		return rest.OpCreate
	case "UPDATE":
		return rest.OpUpdate
	case "PATCH":
		return rest.OpPatch
	case "DELETE":
		return rest.OpDelete
	default:
		return rest.OpOther // shouldn't be here
	}
}

// End old Rest Op
// ------------------------------------------------------------------------------------

// IGuardAPIEntry supports method which guard access to API based on scope
type IGuardAPIEntry interface {
	GuardAPIEntry(who UserIDFetchable, http HTTP) bool
}

// ------------------------------------------------------------------------------------------

// ModelCargo is payload between hookpoints
type ModelCargo struct {
	Payload interface{}
}

// BatchHookCargo is payload between batch update and batch delete hookpoints
type BatchHookCargo struct {
	Payload interface{}
}

type UserIDFetchable interface {
	GetUserID() *datatype.UUID
}

// HookPointData is the data send to single model hookpoints (deprecated)
type HookPointData struct {
	// DB handle (not available for AfterTransact)
	DB *gorm.DB
	// Who is the user information, who is operating this CRUPD right now
	Who UserIDFetchable
	// TypeString is the typeString (model string) of this model
	TypeString string
	// Cargo between Before and After hookpoints (not used in IAfterRead since there is no IBeforeRead.)
	// Currently not supported in the AfterTransact hookpoint
	Cargo *ModelCargo
	// Role of this user in relation to this data, only available during read
	Role *userrole.UserRole
	// URL parameters
	URLParams map[urlparam.Param]interface{}
}

// BatchHookPointData is the data send to batch model hookpoints  (deprecated)
type BatchHookPointData struct {
	// Ms is the slice of IModels
	Ms []mdl.IModel
	// DB is the DB handle
	DB *gorm.DB
	// Who is operating this CRUPD right now
	Who UserIDFetchable
	// TypeString
	TypeString string
	// Cargo between Before and After hookpoints (not used in AfterRead since there is before read hookpoint.)
	Cargo *BatchHookCargo
	// Role of this user in relation to this data, only available during read
	Roles []userrole.UserRole
	// URL parameters
	URLParams map[urlparam.Param]interface{}
}

// IBeforeCreate supports method to be called before data is inserted (created) into the database
type IBeforeCreate interface {
	BeforeCreateDB(hpdata HookPointData) error
}

// IBeforeUpdate supports method to be called before data is updated in the database
type IBeforeUpdate interface {
	BeforeUpdateDB(hpdata HookPointData) error
}

// IBeforePatchApply supports method to be called before data is patched in the database
// And also before the patch is applied. This comes before BeforePatchDB
type IBeforePatchApply interface {
	BeforePatchApplyDB(hpdata HookPointData) error
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
	AfterCreateDB(hpdata HookPointData) error
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

// IBeforeCUPD supprots method to be called before data is after all CRUPD operations
// This is called before the individual ops
type IBeforeCUPD interface {
	BeforeCUPDDB(hpdata HookPointData, op CRUPDOp) error
}

// IAfterCRUPD supprots method to be called after data is after all CRUPD operations
// This is called before the individual ops
type IAfterCRUPD interface {
	AfterCRUPDDB(hpdata HookPointData, op CRUPDOp) error
}

// IAfterTransact is the method to be called after data is after the entire CRUPD
// transaction is done.
type IAfterTransact interface {
	AfterTransact(hpdata HookPointData, op CRUPDOp)
}

// IValidate supports validation with govalidator
type IValidate interface {
	Validate(who UserIDFetchable, http HTTP) error
}

// IHasPermissions is for IModel with a custom permission field to cherry pick json fields
// default is to return all but the dates
type IHasPermissions interface {
	Permissions(role userrole.UserRole, who UserIDFetchable) (jsontrans.Permission, jsontrans.JSONFields)
}

// IHasRenderer is for formatting IModel with a custom function
// basically do your own custom output
// If return false, use the default JSON output
// For batch renderer, register a Render(r UserRole, who modelutil.UserIDFetchable, modelObjs []IModel) bool
type IHasRenderer interface {
	Render(c *gin.Context, hpdata *HookPointData, op CRUPDOp) bool
}

// ------------------------------------------------

// ------------------------------------

// IOwnership is what OwnershipModelBase tables should satisfy.
// Except OwnershipType, that's for struct which embed OwnershipModelBase
type IOwnership interface {
	GetRole() userrole.UserRole
	SetRole(userrole.UserRole)

	GetUserID() *datatype.UUID
	SetUserID(*datatype.UUID)

	GetModelID() *datatype.UUID
	SetModelID(*datatype.UUID)

	GetID() *datatype.UUID
	SetID(*datatype.UUID)

	// OwnershipType() IOwnership
}

// OwnershipModelBase has a role. Intended to be embedded
// by table serving as link from resource to user
type OwnershipModelBase struct {
	ID *datatype.UUID `gorm:"type:uuid;primary_key;" json:"id"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `sql:"index" json:"deletedAt"`

	Role userrole.UserRole `json:"role"` // an int
}

// BeforeCreate sets a UUID if no ID is set
// (this is Gorm's hookpoint)
func (o *OwnershipModelBase) BeforeCreate(scope *gorm.Scope) error {
	if o.ID == nil {
		uuid := datatype.NewUUID()
		return scope.SetColumn("ID", uuid)
	}

	return nil
}

// GetRole gets the role field of the model, comforms to IOwnership
func (o *OwnershipModelBase) GetRole() userrole.UserRole {
	return o.Role
}

// SetRole sets the role field of the model, comforms to IOwnership
func (o *OwnershipModelBase) SetRole(r userrole.UserRole) {
	o.Role = r
}

// GetID Get the ID field of the model (useful when using interface)
func (o *OwnershipModelBase) GetID() *datatype.UUID {
	return o.ID
}

// SetID Set the ID field of the model (useful when using interface)
func (o *OwnershipModelBase) SetID(id *datatype.UUID) {
	o.ID = id
}

// OwnershipModelWithIDBase is one with ID, if you don't need unique index
// for userID and modelID (if you don't expose the link table via LinkTableMapper)
// You can use this.
type OwnershipModelWithIDBase struct {
	OwnershipModelBase

	UserID  *datatype.UUID `gorm:"index" json:"userID"` // I guess the user's table has to be named "User" then.
	ModelID *datatype.UUID `gorm:"index" json:"modelID"`
}

// To comform to IModel, embedding functions don't work

// GetID Get the ID field of the model (useful when using interface)
func (o *OwnershipModelWithIDBase) GetID() *datatype.UUID {
	return o.ID
}

// SetID Set the ID field of the model (useful when using interface)
func (o *OwnershipModelWithIDBase) SetID(id *datatype.UUID) {
	o.ID = id
}

// GetUserID gets the user id of the model, comforms to IOwnership
func (o *OwnershipModelWithIDBase) GetUserID() *datatype.UUID {
	return o.UserID
	// v := reflect.ValueOf(o)
	// return reflect.Indirect(v).FieldByName("ID").Interface().(*datatype.UUID)
}

// SetUserID sets the user id of the model, comforms to IOwnership
func (o *OwnershipModelWithIDBase) SetUserID(id *datatype.UUID) {
	o.UserID = id
}

// SetModelID sets the id of the model, comforms to IOwnership
func (o *OwnershipModelWithIDBase) SetModelID(id *datatype.UUID) {
	o.ModelID = id
}

// GetModelID gets the id of the model, comforms to IOwnership
func (o *OwnershipModelWithIDBase) GetModelID() *datatype.UUID {
	return o.ModelID
}

// GetCreatedAt gets the time stamp the record is created
func (b *OwnershipModelWithIDBase) GetCreatedAt() *time.Time {
	return &b.CreatedAt
}

// GetUpdatedAt gets the time stamp the record is updated
func (b *OwnershipModelWithIDBase) GetUpdatedAt() *time.Time {
	return &b.UpdatedAt
}

// ----------------

func GetTagValueFromModelByTagKeyBetterRestAndValueKey(modelObj interface{}, valueKey string) *string {
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	for i := 0; i < v.NumField(); i++ {
		// t := v.Type().Field(i).Tag.Get(tag) // if no tag, empty string
		if tagVal, ok := v.Type().Field(i).Tag.Lookup("betterrest"); ok {
			pairs := strings.Split(tagVal, ";")
			for _, pair := range pairs {
				if strings.HasPrefix(pair, valueKey) {
					return &pair
				}
			}
		}
	}
	return nil
}

// GetFieldNameFromModelByTagKey get's the name of the tagged field
// If it's a slice, it returns the element type
// It's an interface{} because it's not necessarily IModel
func GetFieldNameFromModelByTagKey(modelObj interface{}, valueKey string) *string {
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	for i := 0; i < v.NumField(); i++ {
		if tagVal, ok := v.Type().Field(i).Tag.Lookup("betterrest"); ok {
			pairs := strings.Split(tagVal, ";")
			for _, pair := range pairs {
				if strings.HasPrefix(pair, valueKey) {
					s := v.Type().Field(i).Name
					return &s
				}
			}

		}
	}
	return nil
}

// GetFieldValueFromModelByTagKeyBetterRestAndValueKey fetches value of the variable tagged in tag
func GetFieldValueFromModelByTagKeyBetterRestAndValueKey(modelObj mdl.IModel, valueKey string) interface{} {
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	for i := 0; i < v.NumField(); i++ {
		if tagVal, ok := v.Type().Field(i).Tag.Lookup("betterrest"); ok {
			pairs := strings.Split(tagVal, ";")
			for _, pair := range pairs {
				if strings.HasPrefix(pair, valueKey) {
					return v.Field(i).Interface()
				}
			}
		}
	}
	return nil
}

// GetFieldTypeFromModelByTagKeyBetterRestAndValueKey fetches the datatype of the variable tagged in tag
func GetFieldTypeFromModelByTagKeyBetterRestAndValueKey(modelObj mdl.IModel, valueKey string, recurseIntoEmbedded bool) reflect.Type {
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	return getFieldTypeFromModelByTagKeyBetterRestAndValueKeyCore(v, valueKey, recurseIntoEmbedded)
}

func getFieldTypeFromModelByTagKeyBetterRestAndValueKeyCore(v reflect.Value, valueKey string, recurseIntoEmbedded bool) reflect.Type {
	for i := 0; i < v.NumField(); i++ {
		if v.Type().Field(i).Anonymous && recurseIntoEmbedded {
			embeddedModel := v.Field(i)
			ret := getFieldTypeFromModelByTagKeyBetterRestAndValueKeyCore(embeddedModel, valueKey, recurseIntoEmbedded)
			if ret != nil {
				return ret
			} // else continues
		} else if tagVal, ok := v.Type().Field(i).Tag.Lookup("betterrest"); ok {
			pairs := strings.Split(tagVal, ";")
			for _, pair := range pairs {
				if strings.HasPrefix(pair, valueKey) {
					fieldVal := v.Field(i)
					switch fieldVal.Kind() {
					case reflect.Slice:
						return v.Type().Field(i).Type.Elem() // This work even when slice is empty
					default:
						// return fieldVal.Type()
						return v.Type().Field(i).Type
					}
				}
			}
		}
	}
	return nil
}
