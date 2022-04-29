package service

import (
	"errors"
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry"
)

type UserService struct {
	BaseService
}

func (serv *UserService) HookBeforeCreateOne(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel) (models.IModel, error) {
	// Special case, there is really no oid in this case, user doesn't exist yet

	// Do nothing

	return modelObj, nil
}

func (serv *UserService) HookBeforeCreateMany(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return nil, errors.New("not implemented")
}

func (serv *UserService) HookBeforeDeleteOne(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel) (models.IModel, error) {
	return modelObj, nil // looks like nothing to do
}

func (serv *UserService) HookBeforeDeleteMany(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return nil, errors.New("not implemented")
}

// ReadOneCore get one model object based on its type and its id string
// ReadOne get one model object based on its type and its id string without invoking read hookpoing
func (serv *UserService) ReadOneCore(db *gorm.DB, who models.UserIDFetchable, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	// TODO: Currently can only read ID from your own (not others in the admin group either)
	db = db.Set("gorm:auto_preload", true)

	// Todo: maybe guest shoud be able to read some fields
	if id.String() != who.GetUserID().String() {
		return nil, 0, ErrPermission
	}

	modelObj := registry.NewFromTypeString(typeString)
	modelObj.SetID(who.GetUserID())

	if err := db.First(modelObj).Error; err != nil {
		return nil, 0, err
	}

	return modelObj, models.UserRoleAdmin, nil
}

func (serv *UserService) GetManyCore(db *gorm.DB, who models.UserIDFetchable, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
	return nil, nil, fmt.Errorf("Not implemented")
}

// GetAllQueryContructCore :-
func (serv *UserService) GetAllQueryContructCore(db *gorm.DB, who models.UserIDFetchable, typeString string) (*gorm.DB, error) {
	return nil, fmt.Errorf("Not implemented")
}

// GetAllRolesCore :-
func (serv *UserService) GetAllRolesCore(dbChained *gorm.DB, dbClean *gorm.DB, who models.UserIDFetchable, typeString string, modelObjs []models.IModel) ([]models.UserRole, error) {
	return nil, fmt.Errorf("Not implemented")
}

// UpdateOneCore one, permissin should already be checked
// called for patch operation as well (after patch has already applied)
// Fuck, repeat the following code for now (you can't call the overriding method from the non-overriding one)
func (serv *UserService) UpdateOneCore(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (modelObj2 models.IModel, err error) {
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
	modelObj2, _, err = serv.ReadOneCore(db, who, typeString, id)
	if err != nil { // Error is "record not found" when not found
		log.Println("Error:", err)
		return nil, err
	}

	// ouch! for many to many we need to remove it again!!
	// because it's in a transaction so it will load up again
	gormfixes.FixManyToMany(modelObj, modelObj2)

	return modelObj2, nil
}
