package service

import (
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

type GlobalService struct {
}

func (serv *GlobalService) HookBeforeCreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	modelID := modelObj.GetID()
	if modelID == nil {
		modelID = datatypes.NewUUID()
		modelObj.SetID(modelID)
	}
	return modelObj, nil
}

func (serv *GlobalService) HookBeforeCreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	// This probably is not necessary
	for i, modelObj := range modelObjs {
		modelID := modelObj.GetID()
		if modelID == nil {
			modelID = datatypes.NewUUID()
			modelObj.SetID(modelID)
		}
		modelObjs[i] = modelObj
	}
	return modelObjs, nil
}

func (serv *GlobalService) HookBeforeDeleteOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	return modelObj, nil
}

func (serv *GlobalService) HookBeforeDeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return modelObjs, nil
}

// GetOneWithIDCore get one model object based on its type and its id string
func (service *GlobalService) GetOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	modelObj := models.NewFromTypeString(typeString)
	modelObj.SetID(id)

	db = db.Set("gorm:auto_preload", true)

	// rtable := models.GetTableNameFromIModel(modelObj)

	// Global object, everyone can find it, simply find it
	err := db.Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	role := models.Public // just some default

	err = gormfixes.LoadManyToManyBecauseGormFailsWithID(db, modelObj)
	if err != nil {
		return nil, 0, err
	}

	return modelObj, role, err
}

func (service *GlobalService) GetManyWithIDsCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
	rtable := models.GetTableNameFromTypeString(typeString)
	db = db.Table(rtable).Where(fmt.Sprintf("\"%s\".\"id\" IN (?)", rtable), ids).Set("gorm:auto_preload", true)

	modelObjs, err := models.NewSliceFromDBByTypeString(typeString, db.Set("gorm:auto_preload", true).Find)
	if err != nil {
		log.Println("calling NewSliceFromDBByTypeString err:", err)
		return nil, nil, err
	}

	// Just in case err didn't work (as in the case with IN clause NOT in the ID field, maybe Gorm bug)
	if len(modelObjs) == 0 {
		return nil, nil, fmt.Errorf("not found")
	}

	if len(modelObjs) != len(ids) {
		return nil, nil, ErrBatchUpdateOrPatchOneNotFound
	}

	for _, modelObj := range modelObjs {
		err = gormfixes.LoadManyToManyBecauseGormFailsWithID(db, modelObj)
		if err != nil {
			return nil, nil, err
		}
	}

	return modelObjs, nil, nil
}

func (service *GlobalService) GetAllCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string) ([]models.IModel, []models.UserRole, error) {
	outmodels, err := models.NewSliceFromDBByTypeString(typeString, db.Find) // error from db is returned from here
	if err != nil {
		return nil, nil, err
	}

	// Don't know why this doesn't work
	roles := make([]models.UserRole, len(outmodels), len(outmodels))

	for i := range roles {
		roles[i] = models.Public
	}

	return outmodels, roles, nil
}
