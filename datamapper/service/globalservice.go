package service

import (
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

type GlobalService struct {
	BaseService
}

func (serv *GlobalService) HookBeforeCreateOne(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel) (models.IModel, error) {
	modelID := modelObj.GetID()
	if modelID == nil {
		modelID = datatypes.NewUUID()
		modelObj.SetID(modelID)
	}
	return modelObj, nil
}

func (serv *GlobalService) HookBeforeCreateMany(db *gorm.DB, who models.Who, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return modelObjs, nil
}

func (serv *GlobalService) HookBeforeDeleteOne(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel) (models.IModel, error) {
	return modelObj, nil
}

func (serv *GlobalService) HookBeforeDeleteMany(db *gorm.DB, who models.Who, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return modelObjs, nil
}

// GetOneWithIDCore get one model object based on its type and its id string
func (service *GlobalService) GetOneWithIDCore(db *gorm.DB, who models.Who, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	modelObj := models.NewFromTypeString(typeString)
	modelObj.SetID(id)

	db = db.Set("gorm:auto_preload", true)

	// rtable := models.GetTableNameFromIModel(modelObj)

	// Global object, everyone can find it, simply find it
	err := db.Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	role := models.UserRolePublic // just some default

	err = gormfixes.LoadManyToManyBecauseGormFailsWithID(db, modelObj)
	if err != nil {
		return nil, 0, err
	}

	return modelObj, role, err
}

func (service *GlobalService) GetManyWithIDsCore(db *gorm.DB, who models.Who, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
	rtable := models.GetTableNameFromTypeString(typeString)
	db = db.Table(rtable).Where(fmt.Sprintf("\"%s\".\"id\" IN (?)", rtable), ids).Set("gorm:auto_preload", true)

	modelObjs, err := models.NewSliceFromDBByTypeString(typeString, db.Set("gorm:auto_preload", true).Find)
	if err != nil {
		log.Println("calling NewSliceFromDBByTypeString err:", err)
		return nil, nil, err
	}

	// Just in case err didn't work (as in the case with IN clause NOT in the ID field, maybe Gorm bug)
	if len(modelObjs) == 0 {
		return nil, nil, fmt.Errorf("not found")
	}

	if len(modelObjs) != len(ids) {
		return nil, nil, ErrBatchUpdateOrPatchOneNotFound
	}

	for _, modelObj := range modelObjs {
		err = gormfixes.LoadManyToManyBecauseGormFailsWithID(db, modelObj)
		if err != nil {
			return nil, nil, err
		}
	}

	return modelObjs, nil, nil
}

// GetAllQueryContructCore construct the meat of the query
func (serv *GlobalService) GetAllQueryContructCore(db *gorm.DB, who models.Who, typeString string) (*gorm.DB, error) {
	rtable := models.GetTableNameFromTypeString(typeString)
	return db.Table(rtable), nil // that's it
}

// GetAllRolesCore gets all roles according to the criteria
func (serv *GlobalService) GetAllRolesCore(dbChained *gorm.DB, dbClean *gorm.DB, who models.Who, typeString string, modelObjs []models.IModel) ([]models.UserRole, error) {
	// Don't know why this doesn't work
	roles := make([]models.UserRole, len(modelObjs))
	for i := range roles {
		roles[i] = models.UserRolePublic
	}

	return roles, nil
}

// UpdateOneCore one, permissin should already be checked
// called for patch operation as well (after patch has already applied)
// Fuck, repeat the following code for now (you can't call the overriding method from the non-overriding one)
func (serv *GlobalService) UpdateOneCore(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (modelObj2 models.IModel, err error) {
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
	modelObj2, _, err = serv.GetOneWithIDCore(db, who, typeString, id)
	if err != nil { // Error is "record not found" when not found
		log.Println("Error:", err)
		return nil, err
	}

	// ouch! for many to many we need to remove it again!!
	// because it's in a transaction so it will load up again
	gormfixes.FixManyToMany(modelObj, modelObj2)

	return modelObj2, nil
}
