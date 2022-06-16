package registry

import (
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/hookhandler"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry/handlermap"
)

// ModelRegistry is model registry
var ModelRegistry = make(map[string]*Reg)

// OwnershipTyp is the model of ownership table, the table that has many to many links users with other models
// var OwnershipTyp reflect.Type

// OwnerTyp is the model of the Owner table
// var OwnerTyp reflect.Type

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

	// MapperTypeViaOrgPartition is for type where an organization owns something and it's in partitioned table
	MapperTypeViaOrgPartition

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

// Reg is a registry item
type Reg struct {
	Typ        reflect.Type
	TypVersion string // TypVersion is the Version of this model
	// CreateObj is by default the one passed in when calling RegModel*
	// It could be overriden with RegCustomCreate()
	CreateObj models.IModel

	// If type is link to user type, store type of ownership table (the one
	// that links to user)
	OwnershipType      reflect.Type
	OwnershipTableName *string
	// If custom ownership table is registered, store here
	OwnershipModelObjPtr models.IModel

	// OrgTypeString reflect.Type // If type has link to organization type, store organization type

	OrgTypeString string // If type has link to organization type, store organization typestring

	// CreateMethod can be defined with RegCustomCreate()
	CreateMethod func(db *gorm.DB) (*gorm.DB, error)

	GuardMethod func(ep *hookhandler.EndPointInfo) *webrender.RetError

	BatchMethods string     // Batch endpoints, "CRUD" for create, batch read, batch update, batch delete
	IdvMethods   string     //  ID end points, "RUD" for read one, update one, and delete one
	Mapper       MapperType // Custmized mapper, default to datamapper.SharedOwnershipMapper

	// Begin deprecated
	BeforeCUPD func(bhpData models.BatchHookPointData, op models.CRUPDOp) error // no R since model doens't exist yet
	AfterCRUPD func(bhpData models.BatchHookPointData, op models.CRUPDOp) error

	AfterTransact func(bhpData models.BatchHookPointData, op models.CRUPDOp)

	AfterRead func(bhpData models.BatchHookPointData) error

	BeforeCreate func(bhpData models.BatchHookPointData) error
	AfterCreate  func(bhpData models.BatchHookPointData) error

	BeforeUpdate func(bhpData models.BatchHookPointData) error
	AfterUpdate  func(bhpData models.BatchHookPointData) error

	BeforePatchApply func(bhpData models.BatchHookPointData) error // This comes before patch is applied. Before "BeforePatch"
	BeforePatch      func(bhpData models.BatchHookPointData) error
	AfterPatch       func(bhpData models.BatchHookPointData) error

	BeforeDelete func(bhpData models.BatchHookPointData) error
	AfterDelete  func(bhpData models.BatchHookPointData) error

	BatchRenderer func(c *gin.Context, ms []models.IModel, bhpdata *models.BatchHookPointData, op models.CRUPDOp) bool
	// End deprecated

	// HandlerMap is the new method where we keep handlers
	// You can register any number of hookhandler to handle the rest process.
	// When each conncection is intantiated, the hookhandler remain in memory until the REST op is returned
	// If there are two hookhandler which handles the same method and the same hook, they will both be called.
	// The calling order is not guaranteed.
	HandlerMap *handlermap.HandlerMap

	RendererMethod func(c *gin.Context, data *hookhandler.Data, info *hookhandler.EndPointInfo) bool
}

// func (g *Gateway) AfterCreateDB(db *gorm.DB, typeString string) error {

/*
 * New*() functions
 */

// NewFromTypeString instantiate a new models.IModel object from type registry
func NewFromTypeString(typeString string) models.IModel {
	return reflect.New(ModelRegistry[typeString].Typ).Interface().(models.IModel)
}

// GetTableNameFromTypeString get table name from typeString
func GetTableNameFromTypeString(typeString string) string {
	model := NewFromTypeString(typeString)
	return models.GetTableNameFromIModel(model)
}

// NewSliceStructFromTypeString :
// return something originally like this
// obj := make(map[string][]Room)
// obj["content"] = make([]Room, 0, 0)
// https://stackoverflow.com/questions/50233285/create-a-map-in-go-using-reflection
// func NewSliceStructFromTypeString(typeString string) map[string][]models.IModel {
func NewSliceStructFromTypeString(typeString string) []models.IModel {
	modelType := ModelRegistry[typeString].Typ
	mapType := reflect.MapOf(reflect.TypeOf(""), reflect.SliceOf(modelType)) // string -> model
	obj := reflect.MakeMap(mapType)
	obj.SetMapIndex(reflect.ValueOf("content"), reflect.New(reflect.SliceOf(modelType)).Elem())

	// this is reflect.Value, and I cannot map it to map[string]interface{}, no Obj.Map()
	// panic: interface conversion: interface {} is map[string][]Device, not map[string][]models.IModel
	// return obj.Interface().(map[string][]models.IModel)

	// v.SetMapIndex(reflect.ValueOf(mKey), elemV)
	modelObjs := make([]models.IModel, obj.MapIndex(reflect.ValueOf("content")).Len(),
		obj.MapIndex(reflect.ValueOf("content")).Len())

	arr := obj.MapIndex(reflect.ValueOf("content"))
	for i := 0; i < arr.Len(); i++ {
		modelObjs[i] = arr.Index(i).Interface().(models.IModel)
	}

	// But...cannot unmarshal once returned
	// json: cannot unmarshal object into Go value of type []models.IModel
	return modelObjs
}

// NewSliceFromDBByTypeString queries the database for an array of models based on typeString
// func(dest interface{}) *gorm.DB
func NewSliceFromDBByTypeString(typeString string, f func(interface{}, ...interface{}) *gorm.DB) ([]models.IModel, error) {

	// func NewSliceFromDB(typeString string, f func(dest interface{}) *gorm.DB) ([]models.IModel, []models.Role, error) {
	modelType := ModelRegistry[typeString].Typ
	return NewSliceFromDBByType(modelType, f)
}

// NewSliceFromDBByType queries the database for an array of models based on modelType
// func(dest interface{}) *gorm.DB
func NewSliceFromDBByType(modelType reflect.Type, f func(interface{}, ...interface{}) *gorm.DB) ([]models.IModel, error) {
	// func NewSliceFromDB(typeString string, f func(dest interface{}) *gorm.DB) ([]models.IModel, []models.Role, error) {
	modelObjs := reflect.New(reflect.SliceOf(modelType))

	if err := f(modelObjs.Interface()).Error; err != nil {
		return nil, err
	}

	modelObjs = modelObjs.Elem()

	y := make([]models.IModel, modelObjs.Len())
	for i := 0; i < modelObjs.Len(); i++ {
		ptr2 := reflect.New(modelType)
		ptr2.Elem().Set(modelObjs.Index(i))
		y[i] = ptr2.Interface().(models.IModel)
	}

	return y, nil
}

// -------------------
