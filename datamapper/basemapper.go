package datamapper

import (
	"errors"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/gorm"
)

// Error
var errPermission = errors.New("permission denied")
var errIDEmpty = errors.New("cannot update when ID is empty")
var errIDNotMatch = errors.New("cannot update when ID in HTTP body and URL parameter not match")
var errPatch = errors.New("patch syntax error") // json: cannot unmarshal object into Go value of type jsonpatch.Patch

// ICreateOneMapper has a create one interface
type ICreateOneMapper interface {
	CreateOne(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel) (models.IModel, error)
}

// IGetOneWithIDMapper gets a record with ID
type IGetOneWithIDMapper interface {
	GetOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, id datatypes.UUID) (models.IModel, models.UserRole, error)
}

// IGetOneWithIDCoreMapper gets a record with ID without invoking read hookpoint (internal use only)
type IGetOneWithIDCoreMapper interface {
	getOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, typeString string, id datatypes.UUID) (models.IModel, models.UserRole, error)
}

// IGetAllMapper gets all record
type IGetAllMapper interface {
	ReadAll(db *gorm.DB, oid *datatypes.UUID, typeString string, options map[string]interface{}) ([]models.IModel, []models.UserRole, error)
}

// IUpdateOneWithIDMapper updates a record with the ID
type IUpdateOneWithIDMapper interface {
	UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, modelobj models.IModel, id datatypes.UUID) (models.IModel, error)
}

// IUpdateManyMapper updates many records
type IUpdateManyMapper interface {
	UpdateMany(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObjs []models.IModel) ([]models.IModel, error)
}

// IPatchOneWithIDMapper patch a record with the ID
type IPatchOneWithIDMapper interface {
	PatchOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, jsonPatch []byte, id datatypes.UUID) (models.IModel, error)
}

// IDeleteOneWithID delete a record with the ID
type IDeleteOneWithID interface {
	DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, id datatypes.UUID) (models.IModel, error)
}

// IDeleteMany delete many records
type IDeleteMany interface {
	DeleteMany(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObjs []models.IModel) ([]models.IModel, error)
}
