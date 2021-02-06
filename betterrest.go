package betterrest

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/t2wu/betterrest/models"
)

/*
 * Registration
 */

// RegUserModel register the user model
func RegUserModel(typeString string, modelObj models.IModel) {

	options := models.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: models.MapperTypeUser}
	RegModelWithOption(typeString, modelObj, options)
	models.UserTyp = reflect.TypeOf(modelObj).Elem()
}

// RegModel adds a New function for an models.IModel
func RegModel(typeString string, modelObj models.IModel) {
	options := models.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: models.MapperTypeViaOwnership}
	RegModelWithOption(typeString, modelObj, options)
}

// RegModelWithOption adds a New function for an models.IModel
func RegModelWithOption(typeString string, modelObj models.IModel, options models.RegOptions) {
	if _, ok := models.ModelRegistry[typeString]; ok {
		panic(fmt.Sprintf("%s should not register the same type string twice:", typeString))
	}

	models.ModelRegistry[typeString] = &models.Reg{}

	reg := models.ModelRegistry[typeString] // pointer type
	reg.Typ = reflect.TypeOf(modelObj).Elem()
	reg.CreateTyp = modelObj

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

	// Default 0 is ownershipmapper
	reg.Mapper = options.Mapper

	switch options.Mapper {
	case models.MapperTypeViaOwnership:
		if m, ok := modelObj.(models.IHasOwnershipLink); !ok {
			panic(fmt.Sprintf("struct for typeString %s does not comform to IOwnership", typeString))
		} else {
			reg.OwnershipType = reflect.TypeOf(m.OwnershipType())
		}
	case models.MapperTypeViaOrganization:
		// We want the model type. So we get that by getting name first
		// since the foreign key field name is always nameID
		v := models.GetValueFromModelByTagKeyBetterRestAndValueKey(modelObj, "org")
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

// RegBatchInsertHooks adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func RegBatchInsertHooks(typeString string,
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error) {

	if _, ok := models.ModelRegistry[typeString]; !ok {
		models.ModelRegistry[typeString] = &models.Reg{}
	}

	models.ModelRegistry[typeString].BeforeInsert = before
	models.ModelRegistry[typeString].AfterInsert = after
}

// RegBatchReadHooks adds hookpoints which are called after
// and read, can be left as nil
func RegBatchReadHooks(typeString string,
	after func(bhpData models.BatchHookPointData) error) {

	if _, ok := models.ModelRegistry[typeString]; !ok {
		models.ModelRegistry[typeString] = &models.Reg{}
	}

	models.ModelRegistry[typeString].AfterRead = after
}

// RegBatchUpdateHooks adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func RegBatchUpdateHooks(typeString string,
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error) {

	if _, ok := models.ModelRegistry[typeString]; !ok {
		models.ModelRegistry[typeString] = &models.Reg{}
	}

	models.ModelRegistry[typeString].BeforeUpdate = before
	models.ModelRegistry[typeString].AfterUpdate = after
}

// RegBatchPatchHooks adds hookpoints which are called before
// and after batch update. Either one can be left as nil
func RegBatchPatchHooks(typeString string,
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error) {

	if _, ok := models.ModelRegistry[typeString]; !ok {
		models.ModelRegistry[typeString] = &models.Reg{}
	}

	models.ModelRegistry[typeString].BeforePatch = before
	models.ModelRegistry[typeString].AfterPatch = after
}

// RegBatchDeleteHooks adds hookpoints which are called before
// and after batch delete. Either one can be left as nil
func RegBatchDeleteHooks(typeString string,
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error) {

	if _, ok := models.ModelRegistry[typeString]; !ok {
		models.ModelRegistry[typeString] = &models.Reg{}
	}

	models.ModelRegistry[typeString].BeforeDelete = before
	models.ModelRegistry[typeString].AfterDelete = after
}
