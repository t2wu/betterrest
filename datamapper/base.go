package datamapper

import (
	"betterrest/models"

	"github.com/jinzhu/gorm"
)

// ICreateOneMapper has a create one interface
type ICreateOneMapper interface {
	CreateOne(db *gorm.DB, oid uint, typeString string, modelObj models.IModel) (models.IModel, error)
}

// IGetOneWithIDMapper gets a record with ID
type IGetOneWithIDMapper interface {
	GetOneWithID(db *gorm.DB, oid uint, typeString string, id uint64) (models.IModel, error)
}

// IGetAllMapper gets all record
type IGetAllMapper interface {
	GetAll(db *gorm.DB, oid uint, typeString string, options map[string]int) ([]models.IModel, error)
}

// IUpdateOneWithIDMapper updates a record with its ID
type IUpdateOneWithIDMapper interface {
	UpdateOneWithID(db *gorm.DB, oid uint, typeString string, modelobj models.IModel, id uint64) (models.IModel, error)
}

// IUpdateManyMapper updates a record with its ID
type IUpdateManyMapper interface {
	UpdateMany(db *gorm.DB, oid uint, typeString string, modelObjs []models.IModel) ([]models.IModel, error)
}

// IDeleteOneWithID delete a record with its ID
type IDeleteOneWithID interface {
	DeleteOneWithID(db *gorm.DB, oid uint, typeString string, id uint64) (models.IModel, error)
}

// IDeleteMany delete a record with its ID
type IDeleteMany interface {
	DeleteMany(db *gorm.DB, oid uint, typeString string, modelObjs []models.IModel) ([]models.IModel, error)
}
