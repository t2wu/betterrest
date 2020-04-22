package models

import (
	"encoding/json"
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
var OwnershipTyp reflect.Type

// UserTyp is the model of the User table
var UserTyp reflect.Type

// Reg is a registry item
type Reg struct {
	Typ          reflect.Type
	BeforeUpdate func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *BatchHookCargo) error
	AfterUpdate  func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *BatchHookCargo) error
	BeforeDelete func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *BatchHookCargo) error
	AfterDelete  func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *BatchHookCargo) error
}

/*
 * Registration
 */

// RegisterOwnershipModel register the ownership table so the library can init base on it
func RegisterOwnershipModel(ownership reflect.Type) {
	OwnershipTyp = ownership
}

// AddUserToModelRegistry adds a New function for an IModel
func AddUserToModelRegistry(typeString string, typ reflect.Type) {
	AddModelRegistry(typeString, typ)
	UserTyp = typ
}

// AddModelRegistry adds a New function for an IModel
func AddModelRegistry(typeString string, typ reflect.Type) {
	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].Typ = typ
}

// AddBatchUpdateBeforeAndAfterHookPoints adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func AddBatchUpdateBeforeAndAfterHookPoints(typeString string,
	before func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *BatchHookCargo) error,
	after func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *BatchHookCargo) error) {

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].BeforeUpdate = before
	ModelRegistry[typeString].AfterUpdate = after
}

// AddBatchDeleteBeforeAndAfterHookPoints adds hookpoints which are called before
// and after batch delete. Either one can be left as nil
func AddBatchDeleteBeforeAndAfterHookPoints(typeString string,
	before func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *BatchHookCargo) error,
	after func(ms []IModel, db *gorm.DB, oid *datatypes.UUID, typeString string, cargo *BatchHookCargo) error) {

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

// NewSliceStructFromTypeStringAndJSON unmarshals a JSON collection
func NewSliceStructFromTypeStringAndJSON(typeString string, jsn []byte) ([]IModel, error) {
	modelType := ModelRegistry[typeString].Typ
	mapType := reflect.MapOf(reflect.TypeOf(""), reflect.SliceOf(modelType)) // string -> model

	obj := reflect.MakeMap(mapType)
	obj.SetMapIndex(reflect.ValueOf("content"), reflect.New(reflect.SliceOf(modelType)).Elem())

	ptr := reflect.New(mapType)
	ptr.Elem().Set(obj)

	// obj.(map[string]interface{})
	err := json.Unmarshal(jsn, ptr.Interface())
	if err != nil {
		return nil, err
	}

	// this is reflect.Value, and I cannot map it to map[string]interface{}, no Obj.Map()
	// panic: interface conversion: interface {} is map[string][]Device, not map[string][]IModel
	// return obj.Interface().(map[string][]IModel)

	// v.SetMapIndex(reflect.ValueOf(mKey), elemV)
	modelObjs := make([]IModel, obj.MapIndex(reflect.ValueOf("content")).Len(),
		obj.MapIndex(reflect.ValueOf("content")).Len())

	arr := obj.MapIndex(reflect.ValueOf("content"))
	for i := 0; i < arr.Len(); i++ {

		ptr2 := reflect.New(modelType)
		ptr2.Elem().Set(arr.Index(i))
		modelObjs[i] = ptr2.Interface().(IModel)
	}

	return modelObjs, nil
}

// NewSliceFromDB queries the database for an array of models
func NewSliceFromDB(typeString string, f func(interface{}, ...interface{}) *gorm.DB) ([]IModel, error) {
	modelType := ModelRegistry[typeString].Typ
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
