package datamapper

import (
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/models"
)

// IDataMapper has all the crud interfaces
type IDataMapper interface {
	CreateOne(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel, options *map[urlparam.Param]interface{}) (models.IModel, error)

	CreateMany(db *gorm.DB, who models.Who, typeString string, modelObj []models.IModel, options *map[urlparam.Param]interface{}) ([]models.IModel, error)

	GetOneWithID(db *gorm.DB, who models.Who,
		typeString string, id *datatypes.UUID, options *map[urlparam.Param]interface{}) (models.IModel, models.UserRole, error)

	GetAll(db *gorm.DB, who models.Who,
		typeString string, options *map[urlparam.Param]interface{}) ([]models.IModel, []models.UserRole, *int, error)

	UpdateOneWithID(db *gorm.DB, who models.Who,
		typeString string, modelobj models.IModel, id *datatypes.UUID, options *map[urlparam.Param]interface{}) (models.IModel, error)

	UpdateMany(db *gorm.DB, who models.Who,
		typeString string, modelObjs []models.IModel, options *map[urlparam.Param]interface{}) ([]models.IModel, error)

	PatchOneWithID(db *gorm.DB, who models.Who,
		typeString string, jsonPatch []byte, id *datatypes.UUID, options *map[urlparam.Param]interface{}) (models.IModel, error)

	PatchMany(db *gorm.DB, who models.Who,
		typeString string, jsonIDPatches []models.JSONIDPatch, options *map[urlparam.Param]interface{}) ([]models.IModel, error)

	DeleteOneWithID(db *gorm.DB, who models.Who,
		typeString string, id *datatypes.UUID, options *map[urlparam.Param]interface{}) (models.IModel, error)

	DeleteMany(db *gorm.DB, who models.Who,
		typeString string, modelObjs []models.IModel, options *map[urlparam.Param]interface{}) ([]models.IModel, error)
}
