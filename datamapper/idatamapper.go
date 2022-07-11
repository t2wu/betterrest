package datamapper

import (
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/hfetcher"
	"github.com/t2wu/betterrest/hookhandler"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
)

type MapperRet struct {
	Ms      []models.IModel // if for cardinality 1, only contains one element
	Fetcher *hfetcher.HandlerFetcher
}

// IDataMapper has all the crud interfaces
type IDataMapper interface {
	CreateMany(db *gorm.DB, modelObjs []models.IModel, ep *hookhandler.EndPointInfo, cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	CreateOne(db *gorm.DB, modelObj models.IModel, ep *hookhandler.EndPointInfo, cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	ReadMany(db *gorm.DB, ep *hookhandler.EndPointInfo, cargo *hookhandler.Cargo) (*MapperRet, []models.UserRole, *int, *webrender.RetError)

	ReadOne(db *gorm.DB, id *datatypes.UUID, ep *hookhandler.EndPointInfo, cargo *hookhandler.Cargo) (*MapperRet, models.UserRole, *webrender.RetError)

	UpdateMany(db *gorm.DB, modelObjs []models.IModel, ep *hookhandler.EndPointInfo, cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	UpdateOne(db *gorm.DB, modelObj models.IModel, id *datatypes.UUID, ep *hookhandler.EndPointInfo, cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	PatchMany(db *gorm.DB, jsonIDPatches []models.JSONIDPatch, ep *hookhandler.EndPointInfo, cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	PatchOne(db *gorm.DB, jsonPatch []byte, id *datatypes.UUID, ep *hookhandler.EndPointInfo, cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	DeleteMany(db *gorm.DB, modelObjs []models.IModel, ep *hookhandler.EndPointInfo, cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)

	DeleteOne(db *gorm.DB, id *datatypes.UUID, ep *hookhandler.EndPointInfo, cargo *hookhandler.Cargo) (*MapperRet, *webrender.RetError)
}
