package datamapper

import (
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/controller"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
)

// IDataMapper has all the crud interfaces
type IDataMapper interface {
	CreateMany(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj []models.IModel,
		options map[urlparam.Param]interface{}, cargo *controller.Cargo) ([]models.IModel, *webrender.RetVal)

	CreateOne(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel,
		options map[urlparam.Param]interface{}, cargo *controller.Cargo) (models.IModel, *webrender.RetVal)

	ReadMany(db *gorm.DB, who models.UserIDFetchable,
		typeString string, options map[urlparam.Param]interface{},
		cargo *controller.Cargo) ([]models.IModel, []models.UserRole, *int, *webrender.RetVal)

	ReadOne(db *gorm.DB, who models.UserIDFetchable,
		typeString string, id *datatypes.UUID, options map[urlparam.Param]interface{},
		cargo *controller.Cargo) (models.IModel, models.UserRole, *webrender.RetVal)

	UpdateMany(db *gorm.DB, who models.UserIDFetchable,
		typeString string, modelObjs []models.IModel,
		options map[urlparam.Param]interface{},
		cargo *controller.Cargo) ([]models.IModel, *webrender.RetVal)

	UpdateOne(db *gorm.DB, who models.UserIDFetchable,
		typeString string, modelobj models.IModel, id *datatypes.UUID,
		options map[urlparam.Param]interface{},
		cargo *controller.Cargo) (models.IModel, *webrender.RetVal)

	PatchMany(db *gorm.DB, who models.UserIDFetchable,
		typeString string, jsonIDPatches []models.JSONIDPatch,
		options map[urlparam.Param]interface{},
		cargo *controller.Cargo) ([]models.IModel, *webrender.RetVal)

	PatchOne(db *gorm.DB, who models.UserIDFetchable,
		typeString string, jsonPatch []byte, id *datatypes.UUID,
		options map[urlparam.Param]interface{},
		cargo *controller.Cargo) (models.IModel, *webrender.RetVal)

	DeleteMany(db *gorm.DB, who models.UserIDFetchable,
		typeString string, modelObjs []models.IModel,
		options map[urlparam.Param]interface{},
		cargo *controller.Cargo) ([]models.IModel, *webrender.RetVal)

	DeleteOne(db *gorm.DB, who models.UserIDFetchable,
		typeString string, id *datatypes.UUID, options map[urlparam.Param]interface{},
		cargo *controller.Cargo) (models.IModel, *webrender.RetVal)
}
