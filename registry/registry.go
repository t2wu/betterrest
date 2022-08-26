package registry

import (
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/model/mappertype"
	"github.com/t2wu/betterrest/registry/handlermap"
	"github.com/t2wu/qry/mdl"
)

// ModelRegistry is model registry
var ModelRegistry = make(map[string]*Reg)

// OwnershipTyp is the model of ownership table, the table that has many to many links users with other mdl
// var OwnershipTyp reflect.Type

// OwnerTyp is the model of the Owner table
// var OwnerTyp reflect.Type

// UserTyp is the model of the User table
var UserTyp reflect.Type

// RegOptions is options when you want to add a model to registry
type RegOptions struct {
	BatchMethods string // Batch endpoints, "CRUD" for create, batch read, batch update, batch delete
	IdvMethods   string //  ID end points, "RUD" for read one, update one, and delete one
	Mapper       mappertype.MapperType
}

// Reg is a registry item
type Reg struct {
	Typ        reflect.Type
	TypVersion string // TypVersion is the Version of this model
	// CreateObj is by default the one passed in when calling RegModel*
	// It could be overriden with RegCustomCreate()
	CreateObj mdl.IModel

	// If type is link to user type, store type of ownership table (the one
	// that links to user)
	OwnershipType      reflect.Type
	OwnershipTableName *string
	// If custom ownership table is registered, store here
	OwnershipModelObjPtr mdl.IModel

	// OrgTypeString reflect.Type // If type has link to organization type, store organization type

	OrgTypeString string // If type has link to organization type, store organization typestring

	// CreateMethod can be defined with RegCustomCreate()
	CreateMethod func(db *gorm.DB) (*gorm.DB, error)

	GuardMethods []func(ep *hook.EndPoint) *webrender.RetError

	BatchMethods string                // Batch endpoints, "CRUD" for create, batch read, batch update, batch delete
	IdvMethods   string                //  ID end points, "RUD" for read one, update one, and delete one
	Mapper       mappertype.MapperType // Custmized mapper, default to datamapper.SharedOwnershipMapper

	// // Begin deprecated
	// BeforeCUPD func(bhpData mdlutil.BatchHookPointData, op mdlutil.CRUPDOp) error // no R since model doens't exist yet
	// AfterCRUPD func(bhpData mdlutil.BatchHookPointData, op mdlutil.CRUPDOp) error

	// AfterTransact func(bhpData mdlutil.BatchHookPointData, op mdlutil.CRUPDOp)

	// AfterRead func(bhpData mdlutil.BatchHookPointData) error

	// BeforeCreate func(bhpData mdlutil.BatchHookPointData) error
	// AfterCreate  func(bhpData mdlutil.BatchHookPointData) error

	// BeforeUpdate func(bhpData mdlutil.BatchHookPointData) error
	// AfterUpdate  func(bhpData mdlutil.BatchHookPointData) error

	// BeforePatchApply func(bhpData mdlutil.BatchHookPointData) error // This comes before patch is applied. Before "BeforePatch"
	// BeforePatch      func(bhpData mdlutil.BatchHookPointData) error
	// AfterPatch       func(bhpData mdlutil.BatchHookPointData) error

	// BeforeDelete func(bhpData mdlutil.BatchHookPointData) error
	// AfterDelete  func(bhpData mdlutil.BatchHookPointData) error

	// BatchRenderer func(c *gin.Context, ms []mdl.IModel, bhpdata *mdlutil.BatchHookPointData, op mdlutil.CRUPDOp) bool
	// // End deprecated

	// HandlerMap is the new method where we keep handlers
	// You can register any number of hook to handle the rest process.
	// When each conncection is intantiated, the hook remain in memory until the REST op is returned
	// If there are two hook which handles the same method and the same hook, they will both be called.
	// The calling order is not guaranteed.
	HandlerMap *handlermap.HandlerMap

	// RendererMethod func(c *gin.Context, data *hook.Data, info *hook.EndPoint, total *int) bool
}

// func (g *Gateway) AfterCreateDB(db *gorm.DB, typeString string) error {

/*
 * New*() functions
 */

// NewFromTypeString instantiate a new mdl.IModel object from type registry
func NewFromTypeString(typeString string) mdl.IModel {
	return reflect.New(ModelRegistry[typeString].Typ).Interface().(mdl.IModel)
}

// GetTableNameFromTypeString get table name from typeString
func GetTableNameFromTypeString(typeString string) string {
	model := NewFromTypeString(typeString)
	return mdl.GetTableNameFromIModel(model)
}

// NewSliceStructFromTypeString :
// return something originally like this
// obj := make(map[string][]Room)
// obj["content"] = make([]Room, 0, 0)
// https://stackoverflow.com/questions/50233285/create-a-map-in-go-using-reflection
// func NewSliceStructFromTypeString(typeString string) map[string][]mdl.IModel {
func NewSliceStructFromTypeString(typeString string) []mdl.IModel {
	modelType := ModelRegistry[typeString].Typ
	mapType := reflect.MapOf(reflect.TypeOf(""), reflect.SliceOf(modelType)) // string -> model
	obj := reflect.MakeMap(mapType)
	obj.SetMapIndex(reflect.ValueOf("content"), reflect.New(reflect.SliceOf(modelType)).Elem())

	// this is reflect.Value, and I cannot map it to map[string]interface{}, no Obj.Map()
	// panic: interface conversion: interface {} is map[string][]Device, not map[string][]mdl.IModel
	// return obj.Interface().(map[string][]mdl.IModel)

	// v.SetMapIndex(reflect.ValueOf(mKey), elemV)
	modelObjs := make([]mdl.IModel, obj.MapIndex(reflect.ValueOf("content")).Len(),
		obj.MapIndex(reflect.ValueOf("content")).Len())

	arr := obj.MapIndex(reflect.ValueOf("content"))
	for i := 0; i < arr.Len(); i++ {
		modelObjs[i] = arr.Index(i).Interface().(mdl.IModel)
	}

	// But...cannot unmarshal once returned
	// json: cannot unmarshal object into Go value of type []mdl.IModel
	return modelObjs
}

// NewSliceFromDBByTypeString queries the database for an array of mdl based on typeString
// func(dest interface{}) *gorm.DB
func NewSliceFromDBByTypeString(typeString string, f func(interface{}, ...interface{}) *gorm.DB) ([]mdl.IModel, error) {

	// func NewSliceFromDB(typeString string, f func(dest interface{}) *gorm.DB) ([]mdl.IModel, []mdl.Role, error) {
	modelType := ModelRegistry[typeString].Typ
	return NewSliceFromDBByType(modelType, f)
}

// NewSliceFromDBByType queries the database for an array of mdl based on modelType
// func(dest interface{}) *gorm.DB
func NewSliceFromDBByType(modelType reflect.Type, f func(interface{}, ...interface{}) *gorm.DB) ([]mdl.IModel, error) {
	// func NewSliceFromDB(typeString string, f func(dest interface{}) *gorm.DB) ([]mdl.IModel, []mdl.Role, error) {
	modelObjs := reflect.New(reflect.SliceOf(modelType))

	if err := f(modelObjs.Interface()).Error; err != nil {
		return nil, err
	}

	modelObjs = modelObjs.Elem()

	y := make([]mdl.IModel, modelObjs.Len())
	for i := 0; i < modelObjs.Len(); i++ {
		ptr2 := reflect.New(modelType)
		ptr2.Elem().Set(modelObjs.Index(i))
		y[i] = ptr2.Interface().(mdl.IModel)
	}

	return y, nil
}

// -------------------
