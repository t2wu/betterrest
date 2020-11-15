package models

import (
	"reflect"

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
	// MapperTypeViaOwnership is for type which user owns something
	MapperTypeViaOwnership = 0

	// MapperTypeViaOrganization is for type where an organization owns something
	MapperTypeViaOrganization = 1

	// MapperTypeGlobal is for type where data is public to all
	MapperTypeGlobal = 2
)

// ModelRegistryOptions is options when you want to add a model to registry
type ModelRegistryOptions struct {
	BatchEndpoints string // Batch endpoints, "CRUD" for create, batch read, batch update, batch delete
	IDEndPoints    string //  ID end points, "RUD" for read one, update one, and delete one
	Mapper         MapperType
}

// Reg is a registry item
type Reg struct {
	Typ            reflect.Type
	BatchEndpoints string     // Batch endpoints, "CRUD" for create, batch read, batch update, batch delete
	IDEndPoints    string     //  ID end points, "RUD" for read one, update one, and delete one
	Mapper         MapperType // Custmized mapper, default to datamapper.SharedOwnershipMapper

	// There is no batch insert yet
	// BeforeInsert func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *BatchHookCargo) error
	// AfterInsert  func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *BatchHookCargo) error

	AfterRead func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, roles []UserRole) error

	BeforeInsert func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *BatchHookCargo) error
	AfterInsert  func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *BatchHookCargo) error

	BeforeUpdate func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *BatchHookCargo) error
	AfterUpdate  func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *BatchHookCargo) error

	// There is no batch patch yet
	// BeforePatch  func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *BatchHookCargo) error
	// AfterPatch   func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *BatchHookCargo) error

	BeforeDelete func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *BatchHookCargo) error
	AfterDelete  func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *BatchHookCargo) error
}

/*
 * Registration
 */

// RegisterOwnershipModel register the ownership table so the library can init base on it
// func RegisterOwnershipModel(ownership reflect.Type) {
// 	OwnershipTyp = ownership
// }

// AddOwnerToModelRegistry adds a New function for an owner
func AddOwnerToModelRegistry(typeString string, typ reflect.Type) {
	AddModelRegistry(typeString, typ)
	OwnerTyp = typ
}

// AddUserToModelRegistry adds a New function for a user
func AddUserToModelRegistry(typeString string, typ reflect.Type) {
	AddModelRegistry(typeString, typ)
	UserTyp = typ
}

// AddModelReg adds a New function for an IModel
// func AddModelReg(typeString string, Reg) {
// 	AddModelRegistryWithOptions(typeString, typ, "CRUD", "RUPD")
// }

// AddModelRegistry adds a New function for an IModel
func AddModelRegistry(typeString string, typ reflect.Type) {
	options := ModelRegistryOptions{BatchEndpoints: "CRUD", IDEndPoints: "RUPD", Mapper: MapperTypeViaOwnership}
	AddModelRegistryWithOptions(typeString, typ, options)
}

// AddModelRegistryWithOptions adds a New function for an IModel
func AddModelRegistryWithOptions(typeString string, typ reflect.Type, options ModelRegistryOptions) {
	// func AddModelRegistryWithOptions(typeString string, typ reflect.Type, batchEndpoints string, idEndPoints string) {
	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].Typ = typ
	if options.BatchEndpoints == "" {
		ModelRegistry[typeString].BatchEndpoints = "CRUD"
	} else {
		ModelRegistry[typeString].BatchEndpoints = options.BatchEndpoints
	}

	if options.IDEndPoints == "" {
		ModelRegistry[typeString].IDEndPoints = "RUPD"
	} else {
		ModelRegistry[typeString].IDEndPoints = options.IDEndPoints
	}

	// Default 0 is ownershipmapper
	ModelRegistry[typeString].Mapper = options.Mapper
}

// AddBatchInsertBeforeAndAfterHookPoints adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func AddBatchInsertBeforeAndAfterHookPoints(typeString string,
	before func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *BatchHookCargo) error,
	after func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *BatchHookCargo) error) {

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].BeforeInsert = before
	ModelRegistry[typeString].AfterInsert = after
}

// AddBatchReadAfterHookPoint adds hookpoints which are called after
// and read, can be left as nil
func AddBatchReadAfterHookPoint(typeString string,
	after func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, roles []UserRole) error) {

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].AfterRead = after
}

// AddBatchUpdateBeforeAndAfterHookPoints adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func AddBatchUpdateBeforeAndAfterHookPoints(typeString string,
	before func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *BatchHookCargo) error,
	after func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *BatchHookCargo) error) {

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].BeforeUpdate = before
	ModelRegistry[typeString].AfterUpdate = after
}

// AddBatchDeleteBeforeAndAfterHookPoints adds hookpoints which are called before
// and after batch delete. Either one can be left as nil
func AddBatchDeleteBeforeAndAfterHookPoints(typeString string,
	before func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *BatchHookCargo) error,
	after func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, cargo *BatchHookCargo) error) {

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
