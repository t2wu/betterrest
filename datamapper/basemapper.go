package datamapper

import (
	"errors"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/gorm"
)

// Error
var errPermission = errors.New("permission denied")
var errPermissionWrongEndPoint = errors.New("permission denied. Change it through the resource endpoint or unable to change your own role.")
var errIDEmpty = errors.New("cannot update when ID is empty")
var errIDNotMatch = errors.New("cannot update when ID in HTTP body and URL parameter not match")
var errPatch = errors.New("patch syntax error")               // json: cannot unmarshal object into Go value of type jsonpatch.Patch
var errNoOwnership = errors.New("model has no OwnershipType") // this is a programmatic error
var errBatchUpdateOrPatchOneNotFound = errors.New("at least one not found")

// ICreateMapper has a create one interface
type ICreateMapper interface {
	CreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error)
	CreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj []models.IModel) ([]models.IModel, error)
}

// IGetOneWithIDMapper gets a record with ID
type IGetOneWithIDMapper interface {
	GetOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error)

	// getOneWithIDCore gets a record with ID without invoking read hookpoint (internal use only)
	getOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error)
}

// IGetAllMapper gets all record
type IGetAllMapper interface {
	GetAll(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, options map[URLParam]interface{}) ([]models.IModel, []models.UserRole, *int, error)
}

// IUpdateOneWithIDMapper updates a record with the ID
type IUpdateOneWithIDMapper interface {
	UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, modelobj models.IModel, id *datatypes.UUID) (models.IModel, error)
}

// IUpdateManyMapper updates many records
type IUpdateManyMapper interface {
	UpdateMany(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, modelObjs []models.IModel) ([]models.IModel, error)
}

// IPatchOneWithIDMapper patch a record with the ID
type IPatchOneWithIDMapper interface {
	PatchOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, jsonPatch []byte, id *datatypes.UUID) (models.IModel, error)
}

// IPatchManyMapper patch a record with the ID
type IPatchManyMapper interface {
	PatchMany(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, jsonIDPatches []models.JSONIDPatch) ([]models.IModel, error)
}

// IDeleteOneWithID delete a record with the ID
type IDeleteOneWithID interface {
	DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, id *datatypes.UUID) (models.IModel, error)
}

// IDeleteMany delete many records
type IDeleteMany interface {
	DeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, modelObjs []models.IModel) ([]models.IModel, error)
}

//------------------------------------
// User model only
//------------------------------------

// IChangeEmailPasswordMapper changes email and password
type IChangeEmailPasswordMapper interface {
	ChangeEmailPasswordWithID(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, modelobj models.IModel, id *datatypes.UUID) (models.IModel, error)
}
