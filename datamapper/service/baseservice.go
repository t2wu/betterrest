package service

import (
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// IService provice basic data fetch for various type of table to user relationships
type IService interface {
	HookBeforeCreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error)
	HookBeforeCreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error)
	HookBeforeDeleteOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error)
	HookBeforeDeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error)

	CreateOneCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (models.IModel, error)
	UpdateOneCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (modelObj2 models.IModel, err error)
	DeleteOneCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObjs models.IModel) (models.IModel, error)

	GetOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error)
	GetManyWithIDsCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error)

	GetAllQueryContructCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string) (*gorm.DB, error)
	GetAllRolesCore(dbChained *gorm.DB, dbClean *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.UserRole, error)
}

// BaseService is the superclass of all services
type BaseService struct {
}

// CreateOneCoreBasic create something
func (serv *BaseService) CreateOneCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (models.IModel, error) {
	// No need to check if primary key is blank.
	// If it is it'll be created by Gorm's BeforeCreate hook
	// (defined in base model)
	// if dbc := db.Create(modelObj); dbc.Error != nil {
	if dbc := db.Create(modelObj); dbc.Error != nil {
		// create failed: UNIQUE constraint failed: user.email
		// It looks like this error may be dependent on the type of database we use
		return nil, dbc.Error
	}

	// For pegassociated, the since we expect association_autoupdate:false
	// need to manually create it
	if err := gormfixes.CreatePeggedAssocFields(db, modelObj); err != nil {
		return nil, err
	}

	// For table with trigger which update before insert, we need to load it again
	if err := db.First(modelObj).Error; err != nil {
		// That's weird. we just inserted it.
		return nil, err
	}

	return modelObj, nil
}

// UpdateOneCore one, permissin should already be checked
// called for patch operation as well (after patch has already applied)
// Fuck, repeat the following code for now (you can't call the overriding method from the non-overriding one)
func (serv *BaseService) UpdateOneCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (modelObj2 models.IModel, err error) {
	if modelNeedsRealDelete(oldModelObj) { // parent model
		db = db.Unscoped()
	}

	err = gormfixes.UpdatePeggedFields(db, oldModelObj, modelObj)
	if err != nil {
		return nil, err
	}

	// For some unknown reason
	// insert many-to-many works cuz Gorm does and works???
	// [2020-05-22 18:50:17]  [1.63ms]  INSERT INTO \"dock_group\" (\"group_id\",\"dock_id\") SELECT '<binary>','<binary>' FROM DUAL WHERE NOT EXISTS (SELECT * FROM \"dock_group\" WHERE \"group_id\" = '<binary>' AND \"dock_id\" = '<binary>')
	// [0 rows affected or returned ]

	// (/Users/t2wu/Documents/Go/pkg/mod/github.com/t2wu/betterrest@v0.1.19/datamapper/modulelibs.go:62)
	// [2020-05-22 18:50:17]  [1.30ms]  UPDATE \"dock\" SET \"updated_at\" = '2020-05-22 18:50:17', \"deleted_at\" = NULL, \"name\" = '', \"model\" = '', \"serial_no\" = '', \"mac\" = '', \"hub_id\" = NULL, \"is_online\" = false, \"room_id\" = NULL  WHERE \"dock\".\"deleted_at\" IS NULL AND \"dock\".\"id\" = '{2920e86e-33b1-4848-a773-e68e5bde4fc0}'
	// [1 rows affected or returned ]

	// (/Users/t2wu/Documents/Go/pkg/mod/github.com/t2wu/betterrest@v0.1.19/datamapper/modulelibs.go:62)
	// [2020-05-22 18:50:17]  [2.84ms]  INSERT INTO \"dock_group\" (\"dock_id\",\"group_id\") SELECT ') �n3�HH�s�[�O�','<binary>' FROM DUAL WHERE NOT EXISTS (SELECT * FROM \"dock_group\" WHERE \"dock_id\" = ') �n3�HH�s�[�O�' AND \"group_id\" = '<binary>')
	// [1 rows affected or returned ]
	if err = db.Save(modelObj).Error; err != nil { // save updates all fields (FIXME: need to check for required)
		log.Println("Error updating:", err)
		return nil, err
	}

	// This loads the IDs
	// This so we have the preloading.
	modelObj2, _, err = serv.GetOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil { // Error is "record not found" when not found
		log.Println("Error:", err)
		return nil, err
	}

	// ouch! for many to many we need to remove it again!!
	// because it's in a transaction so it will load up again
	gormfixes.FixManyToMany(modelObj, modelObj2)

	return modelObj2, nil
}

func (serv *BaseService) DeleteOneCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObjs models.IModel) (models.IModel, error) {
	// Many field is not used, it's just used to conform the interface
	if err := db.Delete(modelObj).Error; err != nil {
		return nil, err
	}

	if err := gormfixes.RemovePeggedField(db, modelObj); err != nil {
		return nil, err
	}

	return modelObj, nil
}

func (serv *BaseService) GetOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	return nil, models.Invalid, fmt.Errorf("GetOneWithIDCore to be overrridden shouldn't be called")
}
