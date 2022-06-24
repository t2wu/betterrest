package registry

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/hookhandler"
	"github.com/t2wu/betterrest/libs/utils/jsontrans"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry/handlermap"
)

/*
 * Registration
 */

// For set the current registering typeString
func For(typeString string) *Registrar {
	r := NewRegistrar(typeString)
	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{
			HandlerMap: handlermap.NewHandlerMap(),
		}
	}
	return r
}

func NewRegistrar(typeString string) *Registrar {
	return &Registrar{currentTypeString: typeString}
}

// Registrar has registration methods for models
type Registrar struct {
	currentTypeString string
}

// Model adds a New function for an models.IModel (convenient function of RegModelWithOption)
func (r *Registrar) Model(modelObj models.IModel) *Registrar {
	options := RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: MapperTypeViaOwnership}
	return r.ModelWithOption(modelObj, options)
}

// ModelWithOption adds a New function for an models.IModel
func (r *Registrar) ModelWithOption(modelObj models.IModel, options RegOptions) *Registrar {
	typeString := r.currentTypeString

	// For JSON href
	jsontrans.ModelNameToTypeStringMapping[reflect.TypeOf(modelObj).String()] = typeString
	jsontrans.ModelNameToTypeStringMapping[reflect.TypeOf(modelObj).Elem().String()] = typeString

	// structNameToTypeStringMapping[reflect.TypeOf(modelObj).Name()] =

	reg := ModelRegistry[typeString] // pointer type
	reg.Typ = reflect.TypeOf(modelObj).Elem()
	// reg.TypVersion = version
	reg.CreateObj = modelObj

	if options.BatchMethods == "" {
		reg.BatchMethods = "CRUPD"
	} else {
		reg.BatchMethods = options.BatchMethods
	}

	if options.IdvMethods == "" {
		reg.IdvMethods = "RUPD"
	} else {
		reg.IdvMethods = options.IdvMethods
	}

	reg.Mapper = options.Mapper

	switch options.Mapper {
	case MapperTypeViaOrgPartition:
		fallthrough
	case MapperTypeViaOrganization:
		// We want the model type. So we get that by getting name first
		// since the foreign key field name is always nameID
		v := models.GetTagValueFromModelByTagKeyBetterRestAndValueKey(modelObj, "org")
		if v == nil {
			panic(fmt.Sprintf("%s missing betterrest:\"org:typeString\" tag", typeString))
		}
		val := *v
		if !strings.Contains(val, "org:") {
			panic(fmt.Sprintf("%s missing tag value for betterrest:\"org:typeString\"", typeString))
		}

		toks := strings.Split(val, "org:")
		reg.OrgTypeString = toks[1]
	case MapperTypeGlobal:
		// do nothing
	case MapperTypeLinkTable:
		// do nothing
	case MapperTypeUser:
		// do nothing
	case MapperTypeViaOwnership:
		fallthrough
	default:
		recursiveIntoEmbedded := true
		typ := models.GetFieldTypeFromModelByTagKeyBetterRestAndValueKey(modelObj, "ownership", recursiveIntoEmbedded)
		if typ == nil {
			panic(fmt.Sprintf("%s missing betterrest:\"ownership\" tag", typeString))
		}
		m := reflect.New(typ).Interface().(models.IModel)
		s := models.GetTableNameFromIModel(m)
		reg.OwnershipTableName = &s
		reg.OwnershipType = typ
	}

	// Check if there is any struct or element of models.IModel which has no betterrest:"peg" or "pegassoc"
	// field. There should be a designation for every struct unless it's ownership or org table
	// Traverse through the tree

	checked := make(map[string]bool)
	checkFieldsThatAreStructsForBetterTags(modelObj, checked)

	return r
}

// Hook adds the handler (contains one or more hooks) to be instantiate when a REST op occurs.
// If any hookhandler exists, old model-based hookpoints and batch hookpoints are not called
func (r *Registrar) Hook(hdlr hookhandler.IHookhandler, method string, args ...interface{}) *Registrar {
	if ModelRegistry[r.currentTypeString].HandlerMap == nil {
		ModelRegistry[r.currentTypeString].HandlerMap = handlermap.NewHandlerMap()
	}

	ModelRegistry[r.currentTypeString].HandlerMap.RegisterHandler(hdlr, method, args...)
	return r
}

// Guard register guard function
func (r *Registrar) Guard(guard func(ep *hookhandler.EndPointInfo) *webrender.RetError) *Registrar {
	ModelRegistry[r.currentTypeString].GuardMethods = append(ModelRegistry[r.currentTypeString].GuardMethods, guard)
	return r
}

// CustomCreate register custom create table funtion
func (r *Registrar) CustomCreate(modelObj models.IModel, f func(db *gorm.DB) (*gorm.DB, error)) *Registrar {
	reg := ModelRegistry[r.currentTypeString] // pointer type
	reg.CreateObj = modelObj
	reg.CreateMethod = f
	return r
}

// -------------------------
// OrgModelTypeFromOrgResourceTypeString given org resource typeString
// returns the reflect type of the organization
func OrgModelTypeFromOrgResourceTypeString(typeString string) reflect.Type {
	if ModelRegistry[typeString].Mapper != MapperTypeViaOrganization && ModelRegistry[typeString].Mapper != MapperTypeViaOrgPartition {
		// Programming error
		panic(fmt.Sprintf("TypeString %s does not represents a resource under organization", typeString))
	}

	orgTypeString := ModelRegistry[typeString].OrgTypeString
	return ModelRegistry[orgTypeString].Typ
}

// ----------------------------
// The new models for all the link tables

// NewOrgModelFromOrgResourceTypeString gets Organization object
// If you're a resource under hooked up by Organization
func NewOrgModelFromOrgResourceTypeString(typeString string) models.IModel {
	if ModelRegistry[typeString].Mapper != MapperTypeViaOrganization && ModelRegistry[typeString].Mapper != MapperTypeViaOrgPartition {
		// Programming error
		panic(fmt.Sprintf("TypeString %s does not represents a resource under organization", typeString))
	}

	orgTypeString := ModelRegistry[typeString].OrgTypeString
	return reflect.New(ModelRegistry[orgTypeString].Typ).Interface().(models.IModel)
}

// NewOrgOwnershipModelFromOrgResourceTypeString gets the joining table from the resource's
// organization model to the user
func NewOrgOwnershipModelFromOrgResourceTypeString(typeString string) models.IModel {
	if ModelRegistry[typeString].Mapper != MapperTypeViaOrganization && ModelRegistry[typeString].Mapper != MapperTypeViaOrgPartition {
		// Programming error
		panic(fmt.Sprintf("TypeString %s does not represents a resource under organization", typeString))
	}

	orgTypeString := ModelRegistry[typeString].OrgTypeString // org is an ownership resource
	return NewOwnershipModelFromOwnershipResourceTypeString(orgTypeString)
}

// NewOwnershipModelFromOwnershipResourceTypeString returns the model object
// of the ownership table (the table that links from this resource represented by the type string
// to the user)
func NewOwnershipModelFromOwnershipResourceTypeString(typeString string) models.IModel {
	if ModelRegistry[typeString].Mapper != MapperTypeViaOwnership {
		// Programming error
		panic(fmt.Sprintf("TypeString %s does not represents a resource under organization", typeString))
	}

	// Either custom one or the default one
	typ := ModelRegistry[typeString].OwnershipType

	return reflect.New(typ).Interface().(models.IModel)
}

// ----------------------------
// The new linking table names

// OrgModelNameFromOrgResourceTypeString given org resource typeString,
// returns organization table name
func OrgModelNameFromOrgResourceTypeString(typeString string) string {
	m := NewOrgModelFromOrgResourceTypeString(typeString)
	return models.GetTableNameFromIModel(m)
}

// OrgOwnershipModelNameFromOrgResourceTypeString given org resource typeString,
// returns name of organization table's linking table (ownership table) to user
func OrgOwnershipModelNameFromOrgResourceTypeString(typeString string) string {
	m := NewOrgOwnershipModelFromOrgResourceTypeString(typeString)
	return models.GetTableNameFromIModel(m)
}

// OwnershipTableNameFromOwnershipResourceTypeString given ownership resource typeStirng
// returns name of ownership table to the user
func OwnershipTableNameFromOwnershipResourceTypeString(typeString string) string {
	// m := NewOwnershipModelFromOwnershipResourceTypeString(typeString)

	// Either custom one or the default one

	tableName := *ModelRegistry[typeString].OwnershipTableName

	if tableName == "ownership_model_with_id_base" {
		m := reflect.New(ModelRegistry[typeString].Typ).Interface().(models.IModel)
		tableName = "user_owns_" + models.GetTableNameFromIModel(m)
	}

	return tableName
}

// ----------------------------------
// Batch hooks
// ----------------------------------

// BatchCRUPDHooks adds hookpoints which are called before
// CUPD (no read) and after batch CRUPD. Either one can be left as nil
func (r *Registrar) BatchCRUPDHooks(
	before func(bhpData models.BatchHookPointData, op models.CRUPDOp) error,
	after func(bhpData models.BatchHookPointData, op models.CRUPDOp) error) *Registrar {
	typeString := r.currentTypeString

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].BeforeCUPD = before
	ModelRegistry[typeString].AfterCRUPD = after

	return r
}

// BatchCRUPDHooks adds hookpoints which are called before
// CUPD (no read) and after batch CRUPD. Either one can be left as nil
func (r *Registrar) BatchAfterTransactHook(
	hook func(bhpData models.BatchHookPointData, op models.CRUPDOp)) *Registrar {
	typeString := r.currentTypeString

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].AfterTransact = hook

	return r
}

// BatchCreateHooks adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func (r *Registrar) BatchCreateHooks(
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error) *Registrar {
	typeString := r.currentTypeString

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].BeforeCreate = before
	ModelRegistry[typeString].AfterCreate = after
	return r
}

// BatchReadHooks adds hookpoints which are called after
// and read, can be left as nil
func (r *Registrar) BatchReadHooks(after func(bhpData models.BatchHookPointData) error) *Registrar {
	typeString := r.currentTypeString

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].AfterRead = after
	return r
}

// BatchUpdateHooks adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func (r *Registrar) BatchUpdateHooks(
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error) *Registrar {
	typeString := r.currentTypeString

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].BeforeUpdate = before
	ModelRegistry[typeString].AfterUpdate = after
	return r
}

// BatchPatchHooks adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func (r *Registrar) BatchPatchHooks(
	beforeApply func(bhpData models.BatchHookPointData) error,
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error) *Registrar {

	typeString := r.currentTypeString

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].BeforePatchApply = beforeApply
	ModelRegistry[typeString].BeforePatch = before
	ModelRegistry[typeString].AfterPatch = after
	return r
}

// BatchDeleteHooks adds hookpoints which are called before
// and after batch delete. Either one can be left as nil
func (r *Registrar) BatchDeleteHooks(
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error) *Registrar {

	typeString := r.currentTypeString

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].BeforeDelete = before
	ModelRegistry[typeString].AfterDelete = after
	return r
}

// Deprecated
// BatchRenderer register custom batch renderer (do your own output, not necessarily JSON)
func (r *Registrar) BatchRenderer(renderer func(c *gin.Context, ms []models.IModel, bhpdata *models.BatchHookPointData, op models.CRUPDOp) bool) *Registrar {
	typeString := r.currentTypeString

	if _, ok := ModelRegistry[typeString]; !ok {
		ModelRegistry[typeString] = &Reg{}
	}

	ModelRegistry[typeString].BatchRenderer = renderer
	return r
}

// End deprecated

// // Renderer is called for both cardinalities when registered
// func (r *Registrar) Renderer(renderer func(c *gin.Context, data *hookhandler.Data, info *hookhandler.EndPointInfo, total *int) bool) *Registrar {
// 	typeString := r.currentTypeString

// 	if _, ok := ModelRegistry[typeString]; !ok {
// 		ModelRegistry[typeString] = &Reg{}
// 	}

// 	ModelRegistry[typeString].RendererMethod = renderer
// 	return r
// }

// If within the model there is a struct that doesn't implement marshaler (which is considered "atomic"),
// it needs to be labeled in one of the ownership models
// checked slice is needed because it can be recursive in a pegassoc-manytomany
func checkFieldsThatAreStructsForBetterTags(modelObj models.IModel, checked map[string]bool) {
	modelName := reflect.TypeOf(modelObj).Elem().Name()
	if _, ok := checked[modelName]; ok { // if already checked
		return
	}
	checked[modelName] = true

	v := reflect.Indirect(reflect.ValueOf(modelObj))
	for i := 0; i < v.NumField(); i++ {
		fieldName := v.Type().Field(i).Name

		var nextType reflect.Type
		switch v.Field(i).Kind() {
		case reflect.Ptr:
			// if it's datatypes.UUID or any other which comforms to json.Marshaler
			// you don't dig further
			if _, ok := v.Field(i).Interface().(json.Marshaler); ok {
				continue
			}
			nextType = v.Type().Field(i).Type.Elem()

			// Then only check if it's a struct
			if nextType.Kind() == reflect.Struct {
				tagVal := v.Type().Field(i).Tag.Get("betterrest")
				checkBetterTagValueIsValid(tagVal, fieldName, modelName)
			}
		case reflect.Struct:
			// if it's datatypes.UUID or any other which comforms to json.Marshaler
			// you don't dig further
			if _, ok := v.Field(i).Addr().Interface().(json.Marshaler); ok {
				continue
			}
			if !v.Type().Field(i).Anonymous { // ignore fields that are anonymous
				tagVal := v.Type().Field(i).Tag.Get("betterrest")
				checkBetterTagValueIsValid(tagVal, fieldName, modelName)
			}

			nextType = v.Type().Field(i).Type
		case reflect.Slice:
			tagVal := v.Type().Field(i).Tag.Get("betterrest")
			checkBetterTagValueIsValid(tagVal, fieldName, modelName)

			nextType = v.Type().Field(i).Type.Elem()
		}

		if nextType != nil {
			// only array []*model will work, what now? if it's not array?
			if nextModel, ok := reflect.New(nextType).Interface().(models.IModel); ok {
				checkFieldsThatAreStructsForBetterTags(nextModel, checked) // how to get the name of struct
			}
		}
	}
	// return nil
}

func checkBetterTagValueIsValid(tagVal, fieldName, modelName string) {
	pairs := strings.Split(tagVal, ";")
	for _, pair := range pairs {
		if pair != "peg" && !strings.HasPrefix(pair, "pegassoc") &&
			!strings.HasPrefix(pair, "ownership") && !strings.HasPrefix(pair, "org") &&
			!strings.HasPrefix(pair, "peg-ignore") && !strings.HasPrefix(pair, "pegassoc-manytomany") {
			panic(fmt.Sprintf("%s in %s struct or array with the exception of UUID should have one of the following tag: peg, pegassoc, pegassoc-manytomany, ownership, org, or peg-ignore", fieldName, modelName))
		}
	}
}

// AutoMigrate all dbs
// Commented out because I haven't figure out about how to handle
// primary key dependencies yet. Or does Gorm do it in a newer version
// Order matters. A table has to exist first.
// func AutoMigrate() {
// 	for typeString, reg := range ModelRegistry {
// 		log.Println("=============creating db:", typeString)
// 		d := db.Shared()

// 		// CreateObject is defined when register the model
// 		// But it could be overridden by RegCustomCreate
// 		// and can be overrridden to be nil
// 		if reg.CreateObj != nil {
// 			d.AutoMigrate(reg.CreateObj)
// 		}

// 		if reg.Mapper == MapperTypeViaOwnership {
// 			log.Println("=============creating default ownership table:", typeString)
// 			// Search for custom ownership, otherwise the automatic one
// 			// reflect.New(OwnershipType)
// 			d.Table(*reg.OwnershipTableName).AutoMigrate(reflect.New(reg.OwnershipType))
// 		}

// 		// In addition, RegCustomCreate cna define a CreateMethod
// 		// which handles any additional create procedure or the actual create procedure
// 		// This is run in addition to CreateObj unless CreateObj is set null
// 		if reg.CreateMethod != nil {
// 			log.Println("has custom create")
// 			reg.CreateMethod(d)
// 		}
// 	}
// }
