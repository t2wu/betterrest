package datamapper

import (
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/hfetcher"
	"github.com/t2wu/betterrest/hookhandler"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
)

type MapperRet struct {
	Ms      []models.IModel // if for cardinality 1, only contains one element
	Fetcher *hfetcher.HandlerFetcher
}

// IDataMapper has all the crud interfaces
type IDataMapper interface {
	CreateMany(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj []models.IModel, info *hookhandler.EndPointInfo,
		options map[urlparam.Param]interface{}, cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	CreateOne(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel,
		info *hookhandler.EndPointInfo, options map[urlparam.Param]interface{}, cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	ReadMany(db *gorm.DB, who models.UserIDFetchable,
		typeString string, info *hookhandler.EndPointInfo, options map[urlparam.Param]interface{},
		cargo *hookhandler.Cargo) (*MapperRet, []models.UserRole, *int, *webrender.RetError)

	ReadOne(db *gorm.DB, who models.UserIDFetchable,
		typeString string, id *datatypes.UUID, info *hookhandler.EndPointInfo, options map[urlparam.Param]interface{},
		cargo *hookhandler.Cargo) (*MapperRet, models.UserRole, *webrender.RetError)

	UpdateMany(db *gorm.DB, who models.UserIDFetchable,
		typeString string, modelObjs []models.IModel, info *hookhandler.EndPointInfo,
		options map[urlparam.Param]interface{},
		cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	UpdateOne(db *gorm.DB, who models.UserIDFetchable,
		typeString string, modelobj models.IModel, id *datatypes.UUID, info *hookhandler.EndPointInfo,
		options map[urlparam.Param]interface{},
		cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	PatchMany(db *gorm.DB, who models.UserIDFetchable,
		typeString string, jsonIDPatches []models.JSONIDPatch, info *hookhandler.EndPointInfo,
		options map[urlparam.Param]interface{},
		cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	PatchOne(db *gorm.DB, who models.UserIDFetchable,
		typeString string, jsonPatch []byte, id *datatypes.UUID, info *hookhandler.EndPointInfo,
		options map[urlparam.Param]interface{},
		cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	DeleteMany(db *gorm.DB, who models.UserIDFetchable,
		typeString string, modelObjs []models.IModel, info *hookhandler.EndPointInfo,
		options map[urlparam.Param]interface{},
		cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	DeleteOne(db *gorm.DB, who models.UserIDFetchable,
		typeString string, id *datatypes.UUID, info *hookhandler.EndPointInfo, options map[urlparam.Param]interface{},
		cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)
}
