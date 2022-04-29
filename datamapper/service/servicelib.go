package service

import (
	"reflect"

	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry"
)

func modelNeedsRealDelete(modelObj models.IModel) bool {
	// real delete by default
	realDelete := true
	if modelObj2, ok := modelObj.(models.IDoRealDelete); ok {
		realDelete = modelObj2.DoRealDelete()
	}
	return realDelete
}

func getModelTableNameAndJoinTableNameFromTypeString(typeString string) (string, string, error) {
	joinTableName := registry.OwnershipTableNameFromOwnershipResourceTypeString(typeString)

	// This is the go to class for join. So if they use this it's a different
	// join table name from main resource name (org table)
	if joinTableName == "ownership_model_with_id_base" {
		resourceModel := reflect.New(registry.ModelRegistry[typeString].Typ).Interface().(models.IModel)
		resourceTableName := registry.GetTableNameFromIModel(resourceModel)
		joinTableName = "user_owns_" + resourceTableName
	}

	// joinTableName := models.GetJoinTableNameFromTypeString(typeString)
	modelTableName := registry.GetTableNameFromTypeString(typeString)
	return modelTableName, joinTableName, nil
}
