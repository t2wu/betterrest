package datamapper

import (
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/hfetcher"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"
)

type MapperRet struct {
	Ms      []mdl.IModel // if for cardinality 1, only contains one element
	Roles   []userrole.UserRole
	Fetcher *hfetcher.HandlerFetcher
}

// IDataMapper has all the crud interfaces
type IDataMapper interface {
	// CreateMany(db *gorm.DB, modelObjs []mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError)

	// CreateOne(db *gorm.DB, modelObj mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError)

	ReadMany(db *gorm.DB, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, []userrole.UserRole, *int, *webrender.RetError)

	ReadOne(db *gorm.DB, id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, userrole.UserRole, *webrender.RetError)

	// UpdateMany(db *gorm.DB, modelObjs []mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError)

	// UpdateOne(db *gorm.DB, modelObj mdl.IModel, id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError)

	// PatchMany(db *gorm.DB, jsonIDPatches []mdlutil.JSONIDPatch, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError)

	// PatchOne(db *gorm.DB, jsonPatch []byte, id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError)

	DeleteMany(db *gorm.DB, modelObjs []mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError)

	DeleteOne(db *gorm.DB, id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError)

	// The new one, since endpoints other than read and delete are not necessary to differentiate between two endpoints
	Create(db *gorm.DB, modelObjs []mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError)
	// ReadMany(db *gorm.DB, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, []userrole.UserRole, *int, *webrender.RetError)
	// ReadOne(db *gorm.DB, id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, userrole.UserRole, *webrender.RetError)

	Update(db *gorm.DB, modelObjs []mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError)

	// Patch is not easy to change since that patch itself has different format,
	// need to figure it out from lifecycle first

	Patch(db *gorm.DB, jsonIDPatches []mdlutil.JSONIDPatch, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError)

	// DeleteMany(db *gorm.DB, modelObjs []mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError)
	// DeleteOne(db *gorm.DB, id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError)
}
