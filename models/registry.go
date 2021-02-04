package models

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/t2wu/betterrest/libs/datatypes"

	"github.com/jinzhu/gorm"
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
	// MapperTypeUser is user itself
	MapperTypeUser MapperType = iota

	// MapperTypeViaOwnership is for type which user owns something
	MapperTypeViaOwnership

	// MapperTypeViaOrganization is for type where an organization owns something
	MapperTypeViaOrganization

	// MapperTypeGlobal is for type where data is public to all
	MapperTypeGlobal

	// MapperTypeLinkTable is for table linking user and regular models
	MapperTypeLinkTable
)

// ModelRegistryOptions is options when you want to add a model to registry
type ModelRegistryOptions struct {
	BatchEndpoints string // Batch endpoints, "CRUD" for create, batch read, batch update, batch delete
	IDEndPoints    string //  ID end points, "RUD" for read one, update one, and delete one
	Mapper         MapperType
}

// BatchHookPointData is the data send to batch model hookpoints
type BatchHookPointData struct {
	// Ms is the slice of IModels
	Ms []IModel
	// DB is the DB handle
	DB *gorm.DB
	// OID is owner ID, the user accessing the API right now
	OID *datatypes.UUID
	// Scope included in the token who is accessing right now
	Scope *string
	// Scope included in the token who is accessing right now
	TypeString string
	// Cargo between Before and After hookpoints (not used in AfterRead since there is before read hookpoint.)
	Cargo *BatchHookCargo
	// Role of this user in relation to this data, only available during read
	Roles []UserRole
}

// Reg is a registry item
type Reg struct {
	Typ reflect.Type

	// If type is link to user type, store type of ownership table (the one
	// that links to user)
	OwnershipType reflect.Type

	// OrgTypeString reflect.Type // If type has link to organization type, store organization type

	OrgTypeString string // If type has link to organization type, store organization typestring

	BatchEndpoints string     // Batch endpoints, "CRUD" for create, batch read, batch update, batch delete
	IDEndPoints    string     //  ID end points, "RUD" for read one, update one, and delete one
	Mapper         MapperType // Custmized mapper, default to datamapper.SharedOwnershipMapper

	AfterRead func(bhpData BatchHookPointData) error

	BeforeInsert func(bhpData BatchHookPointData) error
	AfterInsert  func(bhpData BatchHookPointData) error

	BeforeUpdate func(bhpData BatchHookPointData) error
	AfterUpdate  func(bhpData BatchHookPointData) error

	BeforePatch func(bhpData BatchHookPointData) error
	AfterPatch  func(bhpData BatchHookPointData) error

	BeforeDelete func(bhpData BatchHookPointData) error
	AfterDelete  func(bhpData BatchHookPointData) error
}

/*
 * Registration
 */

// RegisterOwnershipModel register the ownership table so the library can init base on it
// func RegisterOwnershipModel(ownership reflect.Type) {
// 	OwnershipTyp = ownership
// }

// AddOwnerToModelRegistry adds a New function for an owner
func AddOwnerToModelRegistry(typeString string, modelObj IModel) {
	AddModelRegistry(typeString, modelObj)
	OwnerTyp = reflect.TypeOf(modelObj)
}

// AddUserToModelRegistry adds a New function for a user
func AddUserToModelRegistry(typeString string, modelObj IModel) {

	options := ModelRegistryOptions{BatchEndpoints: "CRUPD", IDEndPoints: "RUPD", Mapper: MapperTypeUser}
	AddModelRegistryWithOptions(typeString, modelObj, options)
	UserTyp = reflect.TypeOf(modelObj).Elem()
}

// AddModelReg adds a New function for an IModel
// func AddModelReg(typeString string, Reg) {
// 	AddModelRegistryWithOptions(typeString, typ, "CRUD", "RUPD")
// }

// AddModelRegistry adds a New function for an IModel
func AddModelRegistry(typeString string, modelObj IModel) {
	options := ModelRegistryOptions{BatchEndpoints: "CRUPD", IDEndPoints: "RUPD", Mapper: MapperTypeViaOwnership}
	AddModelRegistryWithOptions(typeString, modelObj, options)
}

// AddModelRegistryWithOptions adds a New function for an IModel
func AddModelRegistryWithOptions(typeString string, modelObj IModel, options ModelRegistryOptions) {
	if _, ok := ModelRegistry[typeString]; ok {
		panic(fmt.Sprintf("%s should not register the same type string twice:", typeString))
	}

	ModelRegistry[typeString] = &Reg{}

	reg := ModelRegistry[typeString] // pointer type
	reg.Typ = reflect.TypeOf(modelObj).Elem()

	if options.BatchEndpoints == "" {
		reg.BatchEndpoints = "CRUPD"
	} else {
		reg.BatchEndpoints = options.BatchEndpoints
	}

	if options.IDEndPoints == "" {
		reg.IDEndPoints = "RUPD"
	} else {
		reg.IDEndPoints = options.IDEndPoints
	}

	// Default 0 is ownershipmapper
	reg.Mapper = options.Mapper

	switch options.Mapper {
	case MapperTypeViaOwnership:
		if m, ok := modelObj.(IHasOwnershipLink); !ok {
			panic(fmt.Sprintf("struct for typeString %s does not comform to IOwnership", typeString))
		} else {
			reg.OwnershipType = reflect.TypeOf(m.OwnershipType())
		}
	case MapperTypeViaOrganization:
		// We want the model type. So we get that by getting name first
		// since the foreign key field name is always nameID
		v := GetValueFromModelByTagKeyBetterRestAndValueKey(modelObj, "org")
		if v == nil {
			panic(fmt.Sprintf("%s missing betterrest:\"org:typeString\" tag", typeString))
		}
		val := *v
		if !strings.Contains(val, "org:") {
			panic(fmt.Sprintf("%s missing tag value for betterrest:\"org:typeString\"", typeString))
		}

		toks := strings.Split(val, "org:")
		reg.OrgTypeString = toks[1]
	}

}

// AddBatchInsertBeforeAndAfterHookPoints adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func AddBatchInsertBeforeAndAfterHookPoints(typeString string,
	before func(bhpData BatchHookPointData) error,
	after func(bhpData BatchHookPointData) error) {

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].BeforeInsert = before
	ModelRegistry[typeString].AfterInsert = after
}

// AddBatchReadAfterHookPoint adds hookpoints which are called after
// and read, can be left as nil
func AddBatchReadAfterHookPoint(typeString string,
	after func(bhpData BatchHookPointData) error) {

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].AfterRead = after
}

// AddBatchUpdateBeforeAndAfterHookPoints adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func AddBatchUpdateBeforeAndAfterHookPoints(typeString string,
	before func(bhpData BatchHookPointData) error,
	after func(bhpData BatchHookPointData) error) {

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].BeforeUpdate = before
	ModelRegistry[typeString].AfterUpdate = after
}

// AddBatchPatchBeforeAndAfterHookPoints adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func AddBatchPatchBeforeAndAfterHookPoints(typeString string,
	before func(bhpData BatchHookPointData) error,
	after func(bhpData BatchHookPointData) error) {

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].BeforePatch = before
	ModelRegistry[typeString].AfterPatch = after
}

// AddBatchDeleteBeforeAndAfterHookPoints adds hookpoints which are called before
// and after batch delete. Either one can be left as nil
func AddBatchDeleteBeforeAndAfterHookPoints(typeString string,
	before func(bhpData BatchHookPointData) error,
	after func(bhpData BatchHookPointData) error) {

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].BeforeDelete = before
	ModelRegistry[typeString].AfterDelete = after
}

// func (g *Gateway) AfterInsertDB(db *gorm.DB, typeString string) error {

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
