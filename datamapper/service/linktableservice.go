package service

import (
	"errors"
	"fmt"
	"log"
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry"
)

type LinkTableService struct {
	BaseService
}

func (serv *LinkTableService) HookBeforeCreateOne(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel) (models.IModel, error) {
	ownerModelObj, ok := modelObj.(models.IOwnership)
	if !ok {
		return nil, fmt.Errorf("model not an IOwnership object")
	}

	// Might not need this
	if modelObj.GetID() == nil {
		modelObj.SetID(datatypes.NewUUID())
	}

	// You gotta have admin access to the model in order to create a relation
	err := userHasAdminAccessToOriginalModel(db, who.GetUserID(), typeString, ownerModelObj.GetModelID())
	if err != nil {
		return nil, err
	}

	// userID := ownerModelObj.GetUserID()

	// // This user actually has to exists!
	// // Again the user table needs to be called "user" (limitation)
	// // Unless I provide an interface to register it specifically
	// type result struct {
	// 	ID *datatypes.UUID
	// }
	// res := result{}
	// if err := db.Table("user").Select("id").Where("id = ?", userID).Scan(&res).Error; err != nil {
	// 	if errors.Is(err, gorm.ErrRecordNotFound) {
	// 		return nil, fmt.Errorf("user does not exists")
	// 	}
	// }
	return modelObj, nil
}

func (serv *LinkTableService) HookBeforeCreateMany(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	for _, modelObj := range modelObjs {
		ownerModelObj, ok := modelObj.(models.IOwnership)
		if !ok {
			return nil, fmt.Errorf("model not an IOwnership object")
		}

		// You gotta have admin access to the model in order to create a relation
		err := userHasAdminAccessToOriginalModel(db, who.GetUserID(), typeString, ownerModelObj.GetModelID())
		if err != nil {
			return nil, err
		}
	}
	return modelObjs, nil
}

func (serv *LinkTableService) HookBeforeDeleteOne(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel) (models.IModel, error) {
	return modelObj, nil
}

func (serv *LinkTableService) HookBeforeDeleteMany(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return modelObjs, nil
}

// ReadOneCore get one model object based on its type and its id string
func (service *LinkTableService) ReadOneCore(db *gorm.DB, who models.UserIDFetchable, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	modelObj := registry.NewFromTypeString(typeString)

	// Check if link table
	if _, ok := modelObj.(models.IOwnership); !ok {
		log.Printf("%s not an IOwnership type\n", typeString)
		return nil, models.UserRoleInvalid, fmt.Errorf("%s not an IOwnership type", typeString)
	}

	rtable := registry.GetTableNameFromIModel(modelObj)

	// Subquery: find all models where user_id has ME in it, then find
	// record where model_id from subquery and id matches the one we query for

	// Specify user_id because you gotta own this or is a guest to this
	subquery := fmt.Sprintf("model_id IN (select model_id from %s where user_id = ?)", rtable)

	err := db.Table(rtable).Where(subquery, who.GetUserID()).Where("id = ?", id).Find(modelObj).Error
	// err := db.Table(rtable).Where(subquery, oid).Where("user_id = ?", &id).Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	// It doesn't have to be models.OwnershipModelWithIDBase but it has to have this field
	modelID := reflect.ValueOf(modelObj).Elem().FieldByName(("ModelID")).Interface().(*datatypes.UUID)

	// The role for this role is determined on the role of the row where the user_id is YOU
	type result struct {
		Role models.UserRole
	}
	res := result{}
	if err := db.Table(rtable).Where("user_id = ? and role = ? and model_id = ?",
		who.GetUserID(), models.UserRoleAdmin, modelID).Select("role").Scan(&res).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, models.UserRoleInvalid, ErrPermission
		}
		return nil, models.UserRoleInvalid, err // some other error
	}

	return modelObj, res.Role, err
}

func (serv *LinkTableService) GetManyCore(db *gorm.DB, who models.UserIDFetchable, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
	rtable := registry.GetTableNameFromTypeString(typeString)
	subquery := fmt.Sprintf("model_id IN (select model_id from %s where user_id = ?)", rtable)
	db2 := db.Table(rtable).Where(subquery, who.GetUserID()).Where("id IN (?)", ids)
	modelObjs, err := registry.NewSliceFromDBByTypeString(typeString, db2.Set("gorm:auto_preload", true).Find)
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
		_, roles[i], err = serv.userHasPermissionToEdit(db, who, typeString, id)
		if err != nil {
			return nil, nil, err
		}
	}

	return modelObjs, roles, nil
}

// GetAllQueryContructCore construct query core
func (serv *LinkTableService) GetAllQueryContructCore(db *gorm.DB, who models.UserIDFetchable, typeString string) (*gorm.DB, error) {
	rtable := registry.GetTableNameFromTypeString(typeString)

	// Check if link table
	testModel := registry.NewFromTypeString(typeString)
	// IOwnership means link table
	if _, ok := testModel.(models.IOwnership); !ok {
		log.Printf("%s not an IOwnership type\n", typeString)
		return nil, fmt.Errorf("%s not an IOwnership type", typeString)
	}

	// select * from rtable where model_id IN (select model_id from rtable where user_id = ?)
	// subquery := db.Where("user_id = ?", oid).Table(rtable)
	subquery := fmt.Sprintf("model_id IN (select model_id from %s where user_id = ?)", rtable)
	db = db.Table(rtable).Where(subquery, who.GetUserID())

	return db, nil
}

// GetAllRolesCore gets all roles according to the criteria
func (serv *LinkTableService) GetAllRolesCore(dbChained *gorm.DB, dbClean *gorm.DB, who models.UserIDFetchable, typeString string, modelObjs []models.IModel) ([]models.UserRole, error) {
	// No roles for this table, because this IS the linking table
	roles := make([]models.UserRole, len(modelObjs))
	for i := range roles {
		roles[i] = models.UserRoleInvalid // FIXME It shouldn't be Invaild, it should be the user's access to this
	}

	return roles, nil
}

func (serv *LinkTableService) userHasPermissionToEdit(db *gorm.DB, who models.UserIDFetchable, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	if id == nil || id.UUID.String() == "" {
		return nil, models.UserRoleInvalid, ErrIDEmpty
	}

	// Pull out entire modelObj
	modelObj, _, err := serv.ReadOneCore(db, who, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return nil, models.UserRoleInvalid, err
	}

	uuidVal := modelObj.GetID()
	if uuidVal == nil || uuidVal.String() == "" {
		// in case it's an empty string
		return nil, models.UserRoleInvalid, ErrIDEmpty
	} else if uuidVal.String() != id.UUID.String() {
		return nil, models.UserRoleInvalid, ErrIDNotMatch
	}

	ownerModelObj, ok := modelObj.(models.IOwnership)
	if !ok {
		return nil, models.UserRoleInvalid, fmt.Errorf("model not an IOwnership object")
	}

	// If you're admin to this model, you can only update/delete link data to other
	// If you're guest to this model, then you can remove yourself, but not others
	rtable := registry.GetTableNameFromTypeString(typeString)
	type result struct {
		Role models.UserRole
	}
	res := result{}
	if err := db.Table(rtable).Where("user_id = ? and model_id = ?", who.GetUserID(), ownerModelObj.GetModelID()).First(&res).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, models.UserRoleInvalid, ErrPermission
		}
		return nil, models.UserRoleInvalid, err
	}

	if res.Role == models.UserRoleAdmin && ownerModelObj.GetUserID().String() == who.GetUserID().String() {
		// You can remove other's relation, but not yours
		return nil, res.Role, ErrPermissionWrongEndPoint
	} else if res.Role != models.UserRoleAdmin && ownerModelObj.GetUserID().String() != who.GetUserID().String() {
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
	rtable := registry.GetTableNameFromTypeString(typeString)

	result := struct {
		ID *datatypes.UUID
	}{}

	if err := db.Table(rtable).Where("user_id = ? and role = ? and model_id = ?",
		oid, models.UserRoleAdmin, id).Scan(&result).Error; err != nil {
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
func (serv *LinkTableService) UpdateOneCore(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (modelObj2 models.IModel, err error) {
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
