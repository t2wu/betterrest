package datamapper

import (
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/models"
)

// IDataMapper has all the crud interfaces
type IDataMapper interface {
	CreateOne(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel,
		options map[urlparam.Param]interface{}, cargo *models.ModelCargo) (models.IModel, error)

	CreateMany(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj []models.IModel,
		options map[urlparam.Param]interface{}, cargo *models.BatchHookCargo) ([]models.IModel, error)

	ReadOne(db *gorm.DB, who models.UserIDFetchable,
		typeString string, id *datatypes.UUID, options map[urlparam.Param]interface{},
		cargo *models.ModelCargo) (models.IModel, models.UserRole, error)

	ReadMany(db *gorm.DB, who models.UserIDFetchable,
		typeString string, options map[urlparam.Param]interface{},
		cargo *models.BatchHookCargo) ([]models.IModel, []models.UserRole, *int, error)

	UpdateOne(db *gorm.DB, who models.UserIDFetchable,
		typeString string, modelobj models.IModel, id *datatypes.UUID,
		options map[urlparam.Param]interface{},
		cargo *models.ModelCargo) (models.IModel, error)

	UpdateMany(db *gorm.DB, who models.UserIDFetchable,
		typeString string, modelObjs []models.IModel,
		options map[urlparam.Param]interface{},
		cargo *models.BatchHookCargo) ([]models.IModel, error)

	PatchOne(db *gorm.DB, who models.UserIDFetchable,
		typeString string, jsonPatch []byte, id *datatypes.UUID,
		options map[urlparam.Param]interface{},
		cargo *models.ModelCargo) (models.IModel, error)

	PatchMany(db *gorm.DB, who models.UserIDFetchable,
		typeString string, jsonIDPatches []models.JSONIDPatch,
		options map[urlparam.Param]interface{},
		cargo *models.BatchHookCargo) ([]models.IModel, error)

	DeleteOne(db *gorm.DB, who models.UserIDFetchable,
		typeString string, id *datatypes.UUID, options map[urlparam.Param]interface{},
		cargo *models.ModelCargo) (models.IModel, error)

	DeleteMany(db *gorm.DB, who models.UserIDFetchable,
		typeString string, modelObjs []models.IModel,
		options map[urlparam.Param]interface{},
		cargo *models.BatchHookCargo) ([]models.IModel, error)
}
