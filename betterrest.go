package betterrest

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/utils/jsontrans"
	"github.com/t2wu/betterrest/models"
)

/*
 * Registration
 */

// Registrar has registration methods for models
type Registrar struct {
	currentTypeString string
}

// For set the current registering typeString
func For(typeString string) *Registrar {
	r := &Registrar{}
	r.currentTypeString = typeString
	if _, ok := models.ModelRegistry[typeString]; !ok {
		models.ModelRegistry[typeString] = &models.Reg{}
	}
	return r
}

// UserModel set the IModel type
func (r *Registrar) UserModel(modelObj models.IModel) *Registrar {
	options := models.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: models.MapperTypeUser}
	// Always override as there is only ONE user type
	models.UserTyp = reflect.TypeOf(modelObj).Elem()

	return r.ModelWithOption(modelObj, options)
}

// Model adds a New function for an models.IModel (convenient function of RegModelWithOption)
func (r *Registrar) Model(modelObj models.IModel) *Registrar {
	options := models.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: models.MapperTypeViaOwnership}
	return r.ModelWithOption(modelObj, options)
}

// ModelWithOption adds a New function for an models.IModel
func (r *Registrar) ModelWithOption(modelObj models.IModel, options models.RegOptions) *Registrar {
	typeString := r.currentTypeString

	models.ModelRegistry[typeString] = &models.Reg{}

	// For JSON href
	jsontrans.ModelNameToTypeStringMapping[reflect.TypeOf(modelObj).String()] = typeString
	jsontrans.ModelNameToTypeStringMapping[reflect.TypeOf(modelObj).Elem().String()] = typeString

	// structNameToTypeStringMapping[reflect.TypeOf(modelObj).Name()] =

	reg := models.ModelRegistry[typeString] // pointer type
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
	case models.MapperTypeViaOrganization:
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
	case models.MapperTypeGlobal:
		// do nothing
	case models.MapperTypeLinkTable:
		// do nothing
	case models.MapperTypeUser:
		// do nothing
	case models.MapperTypeViaOwnership:
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

	// Check if there is any struct or element of IModel which has no betterrest:"peg" or "peg-associate"
	// field. There should be a designation for every struct unless it's ownership or org table
	// Traverse through the tree

	checked := make(map[string]bool)
	checkFieldsThatAreStructsForBetterTags(modelObj, checked)

	return r
}

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

// CustomCreate register custom create table funtion
func (r *Registrar) CustomCreate(modelObj models.IModel, f func(db *gorm.DB) (*gorm.DB, error)) *Registrar {
	reg := models.ModelRegistry[r.currentTypeString] // pointer type
	reg.CreateObj = modelObj
	reg.CreateMethod = f
	return r
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

	if _, ok := models.ModelRegistry[typeString]; !ok {
		models.ModelRegistry[typeString] = &models.Reg{}
	}

	models.ModelRegistry[typeString].BeforeCUPD = before
	models.ModelRegistry[typeString].AfterCRUPD = after

	return r
}

// BatchCreateHooks adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func (r *Registrar) BatchCreateHooks(
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error) *Registrar {
	typeString := r.currentTypeString

	if _, ok := models.ModelRegistry[typeString]; !ok {
		models.ModelRegistry[typeString] = &models.Reg{}
	}

	models.ModelRegistry[typeString].BeforeCreate = before
	models.ModelRegistry[typeString].AfterCreate = after
	return r
}

// BatchReadHooks adds hookpoints which are called after
// and read, can be left as nil
func (r *Registrar) BatchReadHooks(after func(bhpData models.BatchHookPointData) error) *Registrar {
	typeString := r.currentTypeString

	if _, ok := models.ModelRegistry[typeString]; !ok {
		models.ModelRegistry[typeString] = &models.Reg{}
	}

	models.ModelRegistry[typeString].AfterRead = after
	return r
}

// BatchUpdateHooks adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func (r *Registrar) BatchUpdateHooks(
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error) *Registrar {
	typeString := r.currentTypeString

	if _, ok := models.ModelRegistry[typeString]; !ok {
		models.ModelRegistry[typeString] = &models.Reg{}
	}

	models.ModelRegistry[typeString].BeforeUpdate = before
	models.ModelRegistry[typeString].AfterUpdate = after
	return r
}

// BatchPatchHooks adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func (r *Registrar) BatchPatchHooks(
	beforeApply func(bhpData models.BatchHookPointData) error,
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error) *Registrar {

	typeString := r.currentTypeString

	if _, ok := models.ModelRegistry[typeString]; !ok {
		models.ModelRegistry[typeString] = &models.Reg{}
	}

	models.ModelRegistry[typeString].BeforePatchApply = beforeApply
	models.ModelRegistry[typeString].BeforePatch = before
	models.ModelRegistry[typeString].AfterPatch = after
	return r
}

// BatchDeleteHooks adds hookpoints which are called before
// and after batch delete. Either one can be left as nil
func (r *Registrar) BatchDeleteHooks(
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error) *Registrar {

	typeString := r.currentTypeString

	if _, ok := models.ModelRegistry[typeString]; !ok {
		models.ModelRegistry[typeString] = &models.Reg{}
	}

	models.ModelRegistry[typeString].BeforeDelete = before
	models.ModelRegistry[typeString].AfterDelete = after
	return r
}

// BatchRenderer register custom batch renderer (do your own output, not necessarily JSON)
func (r *Registrar) BatchRenderer(renderer func(roles []models.UserRole, who models.Who, modelObj []models.IModel) []byte) *Registrar {
	typeString := r.currentTypeString

	if _, ok := models.ModelRegistry[typeString]; !ok {
		models.ModelRegistry[typeString] = &models.Reg{}
	}

	models.ModelRegistry[typeString].BatchRenderer = renderer
	return r
}

// AutoMigrate all dbs
// Commented out because I haven't figure out about how to handle
// primary key dependencies yet. Or does Gorm do it in a newer version
// Order matters. A table has to exist first.
// func AutoMigrate() {
// 	for typeString, reg := range models.ModelRegistry {
// 		log.Println("=============creating db:", typeString)
// 		d := db.Shared()

// 		// CreateObject is defined when register the model
// 		// But it could be overridden by RegCustomCreate
// 		// and can be overrridden to be nil
// 		if reg.CreateObj != nil {
// 			d.AutoMigrate(reg.CreateObj)
// 		}

// 		if reg.Mapper == models.MapperTypeViaOwnership {
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
