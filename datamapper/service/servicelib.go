package service

import "github.com/t2wu/betterrest/models"

func modelNeedsRealDelete(modelObj models.IModel) bool {
	// real delete by default
	realDelete := true
	if modelObj2, ok := modelObj.(models.IDoRealDelete); ok {
		realDelete = modelObj2.DoRealDelete()
	}
	return realDelete
}

func getModelTableNameAndJoinTableNameFromTypeString(typeString string) (string, string, error) {
	joinTableName := models.OwnershipModelNameFromOwnershipResourceTypeString(typeString)
	// joinTableName := models.GetJoinTableNameFromTypeString(typeString)
	modelTableName := models.GetTableNameFromTypeString(typeString)
	return modelTableName, joinTableName, nil
}
