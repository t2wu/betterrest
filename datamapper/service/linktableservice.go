package service

import (
	"errors"
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

type LinkTableService struct {
	BaseService
}

func (serv *LinkTableService) HookBeforeCreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	ownerModelObj, ok := modelObj.(models.IOwnership)
	if !ok {
		return nil, fmt.Errorf("model not an IOwnership object")
	}

	// Might not need this
	if modelObj.GetID() == nil {
		modelObj.SetID(datatypes.NewUUID())
	}

	// You gotta have admin access to the model in order to create a relation
	err := userHasAdminAccessToOriginalModel(db, oid, typeString, ownerModelObj.GetModelID())
	if err != nil {
		return nil, err
	}

	userID := ownerModelObj.GetUserID()

	// This user actually has to exists!
	// Again the user table needs to be called "user" (limitation)
	// Unless I provide an interface to register it specifically
	type result struct {
		ID *datatypes.UUID
	}
	res := result{}
	if err := db.Table("user").Select("id").Where("id = ?", userID).Scan(&res).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user does not exists")
		}
	}
	return modelObj, nil
}

func (serv *LinkTableService) HookBeforeCreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	for i, modelObj := range modelObjs {
		ownerModelObj, ok := modelObj.(models.IOwnership)
		if !ok {
			return nil, fmt.Errorf("model not an IOwnership object")
		}

		// Probably not necessary
		if modelObj.GetID() == nil {
			modelObjs[i].SetID(datatypes.NewUUID())
		}

		// You gotta have admin access to the model in order to create a relation
		err := userHasAdminAccessToOriginalModel(db, oid, typeString, ownerModelObj.GetModelID())
		if err != nil {
			return nil, err
		}
	}
	return modelObjs, nil
}

func (serv *LinkTableService) HookBeforeDeleteOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	return modelObj, nil
}

func (serv *LinkTableService) HookBeforeDeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return modelObjs, nil
}

// getOneWithIDCore get one model object based on its type and its id string
// since this is organizationMapper, need to make sure it's the same organization
func (service *LinkTableService) GetOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	modelObj := models.NewFromTypeString(typeString)

	// Check if link table
	if _, ok := modelObj.(models.IOwnership); !ok {
		log.Printf("%s not an IOwnership type\n", typeString)
		return nil, models.Invalid, fmt.Errorf("%s not an IOwnership type", typeString)
	}

	rtable := models.GetTableNameFromIModel(modelObj)

	// Subquery: find all models where user_id has ME in it, then find
	// record where model_is from subquery and id matches the one we query for

	// Specify user_id because you gotta own this or is a guest to this
	subquery := fmt.Sprintf("model_id IN (select model_id from %s where user_id = ?)", rtable)

	err := db.Table(rtable).Where(subquery, oid).Where("id = ?", &id).Find(modelObj).Error
	// err := db.Table(rtable).Where(subquery, oid).Where("user_id = ?", &id).Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	// The role for this role is determined on the role of the row where the user_id is YOU
	type result struct {
		Role models.UserRole
	}
	res := result{}
	if err := db.Table(rtable).Where("user_id = ? and role = ? and model_id = ?",
		oid, models.Admin, id).Select("role").Scan(&res).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, models.Invalid, ErrPermission
		}
		return nil, models.Invalid, err // some other error
	}

	return modelObj, res.Role, err
}

func (serv *LinkTableService) GetManyWithIDsCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
	rtable := models.GetTableNameFromTypeString(typeString)
	subquery := fmt.Sprintf("model_id IN (select model_id from %s where user_id = ?)", rtable)
	db2 := db.Table(rtable).Where(subquery, oid).Where("id in (?)", ids)
	modelObjs, err := models.NewSliceFromDBByTypeString(typeString, db2.Set("gorm:auto_preload", true).Find)
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

	// Role is more complicated. I need to find for each unique model_id, when I have a corresponding
	// link to it, what role is it?
	// So currently I only know how to fetch one by one
	// I probably can do it in one go, need extra time.
	// Probably using the query in the beginnign of the function but by selecting the Role column
	// TODO
	roles := make([]models.UserRole, len(modelObjs))
	for i, modelObj := range modelObjs {
		id := modelObj.GetID()
		_, roles[i], err = serv.userHasPermissionToEdit(db, oid, scope, typeString, id)
		if err != nil {
			return nil, nil, err
		}
	}

	return modelObjs, roles, nil
}

func (serv *LinkTableService) GetAllCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string) ([]models.IModel, []models.UserRole, error) {
	rtable := models.GetTableNameFromTypeString(typeString)

	// Check if link table
	testModel := models.NewFromTypeString(typeString)
	// IOwnership means link table
	if _, ok := testModel.(models.IOwnership); !ok {
		log.Printf("%s not an IOwnership type\n", typeString)
		return nil, nil, fmt.Errorf("%s not an IOwnership type", typeString)
	}

	// select * from rtable where model_id IN (select model_id from rtable where user_id = ?)
	// subquery := db.Where("user_id = ?", oid).Table(rtable)
	subquery := fmt.Sprintf("model_id IN (select model_id from %s where user_id = ?)", rtable)
	db = db.Table(rtable).Where(subquery, oid)

	outmodels, err := models.NewSliceFromDBByTypeString(typeString, db.Find) // error from db is returned from here
	if err != nil {
		return nil, nil, err
	}

	// No roles for this table, because this IS the linking table
	roles := make([]models.UserRole, len(outmodels), len(outmodels))
	for i := range roles {
		roles[i] = models.Invalid // FIXME It shouldn't be Invaild, it should be the user's access to this
	}

	return outmodels, roles, nil
}

func (serv *LinkTableService) userHasPermissionToEdit(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	if id == nil || id.UUID.String() == "" {
		return nil, models.Invalid, ErrIDEmpty
	}

	// Pull out entire modelObj
	modelObj, _, err := serv.GetOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return nil, models.Invalid, err
	}

	uuidVal := modelObj.GetID()
	if uuidVal == nil || uuidVal.String() == "" {
		// in case it's an empty string
		return nil, models.Invalid, ErrIDEmpty
	} else if uuidVal.String() != id.UUID.String() {
		return nil, models.Invalid, ErrIDNotMatch
	}

	ownerModelObj, ok := modelObj.(models.IOwnership)
	if !ok {
		return nil, models.Invalid, fmt.Errorf("model not an IOwnership object")
	}

	// If you're admin to this model, you can only update/delete link data to other
	// If you're guest to this model, then you can remove yourself, but not others
	rtable := models.GetTableNameFromTypeString(typeString)
	type result struct {
		Role models.UserRole
	}
	res := result{}
	if err := db.Table(rtable).Where("user_id = ? and model_id = ?", oid, ownerModelObj.GetModelID()).First(&res).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, models.Invalid, ErrPermission
		}
		return nil, models.Invalid, err
	}

	if res.Role == models.Admin && ownerModelObj.GetUserID().String() == oid.String() {
		// You can remove other's relation, but not yours
		return nil, res.Role, ErrPermissionWrongEndPoint
	} else if res.Role != models.Admin && ownerModelObj.GetUserID().String() != oid.String() {
		// not admin, only remove yourself
		return nil, res.Role, ErrPermission
	}

	return modelObj, res.Role, nil
}

// ---------------------------------------

func userHasAdminAccessToOriginalModel(db *gorm.DB, oid *datatypes.UUID, typeString string, id *datatypes.UUID) error {
	// We need to find at least one role with the same model id
	// where we're admin for

	// We make sure we NOT by checking the original model table
	// but check link table which we have admin access for
	rtable := models.GetTableNameFromTypeString(typeString)
	if err := db.Table(rtable).Where("user_id = ? and role = ? and model_id = ?",
		oid, models.Admin, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPermission
		}
		return err
	}
	return nil
}

// UpdateOneCore one, permissin should already be checked
// called for patch operation as well (after patch has already applied)
// Fuck, repeat the following code for now (you can't call the overriding method from the non-overriding one)
func (serv *LinkTableService) UpdateOneCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (modelObj2 models.IModel, err error) {
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
