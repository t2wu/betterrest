package models

import (
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/urlparam"
)

// BatchHookCargo is payload between batch update and batch delete hookpoints
type BatchHookCargo struct {
	Payload interface{}
}

// ModelRegistry is model registry
var ModelRegistry = make(map[string]*Reg)

// OwnershipTyp is the model of ownership table, the table that has many to many links users with other models
// var OwnershipTyp reflect.Type

// OwnerTyp is the model of the Owner table
var OwnerTyp reflect.Type

// UserTyp is the model of the User table
var UserTyp reflect.Type

// MapperType is the mapper type
type MapperType int

const (
	// MapperTypeViaOwnership is for type which user owns something
	MapperTypeViaOwnership MapperType = iota

	// MapperTypeUser is user itself
	MapperTypeUser

	// MapperTypeViaOrganization is for type where an organization owns something
	MapperTypeViaOrganization

	// MapperTypeGlobal is for type where data is public to all
	MapperTypeGlobal

	// MapperTypeLinkTable is for table linking user and regular models
	MapperTypeLinkTable
)

// RegOptions is options when you want to add a model to registry
type RegOptions struct {
	BatchMethods string // Batch endpoints, "CRUD" for create, batch read, batch update, batch delete
	IdvMethods   string //  ID end points, "RUD" for read one, update one, and delete one
	Mapper       MapperType
}

// BatchHookPointData is the data send to batch model hookpoints
type BatchHookPointData struct {
	// Ms is the slice of IModels
	Ms []IModel
	// DB is the DB handle
	DB *gorm.DB
	// Who is operating this CRUPD right now
	Who Who
	// Scope included in the token who is accessing right now
	TypeString string
	// Cargo between Before and After hookpoints (not used in AfterRead since there is before read hookpoint.)
	Cargo *BatchHookCargo
	// Role of this user in relation to this data, only available during read
	Roles []UserRole
	// URL parameters
	URLParams *map[urlparam.Param]interface{}
}

// Reg is a registry item
type Reg struct {
	Typ        reflect.Type
	TypVersion string // TypVersion is the Version of this model
	// CreateObj is by default the one passed in when calling RegModel*
	// It could be overriden with RegCustomCreate()
	CreateObj IModel

	// If type is link to user type, store type of ownership table (the one
	// that links to user)
	OwnershipType      reflect.Type
	OwnershipTableName *string
	// If custom ownership table is registered, store here
	OwnershipModelObjPtr IModel

	// OrgTypeString reflect.Type // If type has link to organization type, store organization type

	OrgTypeString string // If type has link to organization type, store organization typestring

	// CreateMethod can be defined with RegCustomCreate()
	CreateMethod func(db *gorm.DB) (*gorm.DB, error)

	BatchMethods string     // Batch endpoints, "CRUD" for create, batch read, batch update, batch delete
	IdvMethods   string     //  ID end points, "RUD" for read one, update one, and delete one
	Mapper       MapperType // Custmized mapper, default to datamapper.SharedOwnershipMapper

	BeforeCUPD func(bhpData BatchHookPointData, op CRUPDOp) error // no R since model doens't exist yet
	AfterCRUPD func(bhpData BatchHookPointData, op CRUPDOp) error

	AfterRead func(bhpData BatchHookPointData) error

	BeforeCreate func(bhpData BatchHookPointData) error
	AfterCreate  func(bhpData BatchHookPointData) error

	BeforeUpdate func(bhpData BatchHookPointData) error
	AfterUpdate  func(bhpData BatchHookPointData) error

	BeforePatchApply func(bhpData BatchHookPointData) error // This comes before patch is applied. Before "BeforePatch"
	BeforePatch      func(bhpData BatchHookPointData) error
	AfterPatch       func(bhpData BatchHookPointData) error

	BeforeDelete func(bhpData BatchHookPointData) error
	AfterDelete  func(bhpData BatchHookPointData) error

	BatchRenderer func(roles []UserRole, who Who, modelObj []IModel) []byte
}

// func (g *Gateway) AfterCreateDB(db *gorm.DB, typeString string) error {

/*
 * New*() functions
 */

// NewFromTypeString instantiate a new IModel object from type registry
func NewFromTypeString(typeString string) IModel {
	return reflect.New(ModelRegistry[typeString].Typ).Interface().(IModel)
}

// NewSliceStructFromTypeString :
// return something originally like this
// obj := make(map[string][]Room)
// obj["content"] = make([]Room, 0, 0)
// https://stackoverflow.com/questions/50233285/create-a-map-in-go-using-reflection
// func NewSliceStructFromTypeString(typeString string) map[string][]IModel {
func NewSliceStructFromTypeString(typeString string) []IModel {
	modelType := ModelRegistry[typeString].Typ
	mapType := reflect.MapOf(reflect.TypeOf(""), reflect.SliceOf(modelType)) // string -> model
	obj := reflect.MakeMap(mapType)
	obj.SetMapIndex(reflect.ValueOf("content"), reflect.New(reflect.SliceOf(modelType)).Elem())

	// this is reflect.Value, and I cannot map it to map[string]interface{}, no Obj.Map()
	// panic: interface conversion: interface {} is map[string][]Device, not map[string][]IModel
	// return obj.Interface().(map[string][]IModel)

	// v.SetMapIndex(reflect.ValueOf(mKey), elemV)
	modelObjs := make([]IModel, obj.MapIndex(reflect.ValueOf("content")).Len(),
		obj.MapIndex(reflect.ValueOf("content")).Len())

	arr := obj.MapIndex(reflect.ValueOf("content"))
	for i := 0; i < arr.Len(); i++ {
		modelObjs[i] = arr.Index(i).Interface().(IModel)
	}

	// But...cannot unmarshal once returned
	// json: cannot unmarshal object into Go value of type []IModel
	return modelObjs
}

// NewSliceFromDBByTypeString queries the database for an array of models based on typeString
// func(dest interface{}) *gorm.DB
func NewSliceFromDBByTypeString(typeString string, f func(interface{}, ...interface{}) *gorm.DB) ([]IModel, error) {

	// func NewSliceFromDB(typeString string, f func(dest interface{}) *gorm.DB) ([]IModel, []models.Role, error) {
	modelType := ModelRegistry[typeString].Typ
	return NewSliceFromDBByType(modelType, f)
}

// NewSliceFromDBByType queries the database for an array of models based on modelType
// func(dest interface{}) *gorm.DB
func NewSliceFromDBByType(modelType reflect.Type, f func(interface{}, ...interface{}) *gorm.DB) ([]IModel, error) {
	// func NewSliceFromDB(typeString string, f func(dest interface{}) *gorm.DB) ([]IModel, []models.Role, error) {
	modelObjs := reflect.New(reflect.SliceOf(modelType))

	if err := f(modelObjs.Interface()).Error; err != nil {
		return nil, err
	}

	modelObjs = modelObjs.Elem()

	y := make([]IModel, modelObjs.Len())
	for i := 0; i < modelObjs.Len(); i++ {
		ptr2 := reflect.New(modelType)
		ptr2.Elem().Set(modelObjs.Index(i))
		y[i] = ptr2.Interface().(IModel)
	}

	return y, nil
}

// -------------------
