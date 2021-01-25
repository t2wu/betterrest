package service

import (
	"fmt"
	"log"
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// check out
// https://stackoverflow.com/questions/52124137/cant-set-field-of-a-struct-that-is-typed-as-an-interface
/*
	a := reflect.ValueOf(modelObj).Elem()
	b := reflect.Indirect(a).FieldByName("ID")
	b.Set(reflect.ValueOf(uint(id)))
*/

// OwnershipService handles all the ownership specific db calls
type OwnershipService struct {
}

func (serv *OwnershipService) HookBeforeCreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	// db.Model(&u).Association("Ownerships")

	// Get ownership type and creates it
	// field, _ := reflect.TypeOf(modelObj).Elem().FieldByName("Ownerships")
	// ownershipType := field.Type

	// has to be true otherwise shouldn't use this mapper
	modelObjOwnership, ok := modelObj.(models.IHasOwnershipLink)
	if !ok {
		return nil, ErrNoOwnership
	}

	ownershipType := modelObjOwnership.OwnershipType()

	modelID := modelObj.GetID()
	if modelID == nil {
		modelID = datatypes.NewUUID()
		modelObj.SetID(modelID)
	}

	// reflect.SliceOf
	g := reflect.New(ownershipType).Interface().(models.IOwnership)

	g.SetUserID(oid)
	g.SetModelID(modelID)
	g.SetRole(models.Admin)

	// ownerships := reflect.New(reflect.SliceOf(ownershipType))
	// o.Set(reflect.Append(ownerships, reflect.ValueOf(g)))

	// Associate a ownership group with this model
	// This is not strictly really necessary as actual SQL table has no such field. I could have
	// just save the "g", But it's for hooks
	o := reflect.ValueOf(modelObj).Elem().FieldByName("Ownerships")
	o.Set(reflect.Append(o, reflect.ValueOf(g).Elem()))
	return modelObj, nil
}

func (serv *OwnershipService) HookBeforeCreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	// db.Model(&u).Association("Ownerships")

	// Get ownership type and creates it
	// field, _ := reflect.TypeOf(modelObj).Elem().FieldByName("Ownerships")
	// ownershipType := field.Type
	modelObjOwnership, ok := modelObjs[0].(models.IHasOwnershipLink)
	if !ok {
		return nil, ErrNoOwnership
	}

	ownershipType := modelObjOwnership.OwnershipType()
	for _, modelObj := range modelObjs {
		// reflect.SliceOf
		g := reflect.New(ownershipType).Interface().(models.IOwnership)

		modelID := modelObj.GetID()
		if modelID == nil {
			modelID = datatypes.NewUUID()
			modelObj.SetID(modelID)
		}

		g.SetUserID(oid)
		g.SetModelID(modelID)
		g.SetRole(models.Admin)

		// ownerships := reflect.New(reflect.SliceOf(ownershipType))
		// o.Set(reflect.Append(ownerships, reflect.ValueOf(g)))

		// Associate a ownership group with this model
		// This is not strictly really necessary as actual SQL table has no such field. I could have
		// just save the "g", But it's for hooks
		o := reflect.ValueOf(modelObj).Elem().FieldByName("Ownerships")
		o.Set(reflect.Append(o, reflect.ValueOf(g).Elem()))
	}
	return modelObjs, nil
}

func (serv *OwnershipService) HookBeforeDeleteOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	// I'm removing stuffs from this link table, I cannot just remove myself from this. I have to remove
	// everyone who is linked to this table!
	modelObjOwnership, ok := modelObj.(models.IHasOwnershipLink)
	if !ok {
		return nil, ErrNoOwnership
	}

	// stmt := fmt.Sprintf("DELETE FROM %s WHERE user_id = ? AND model_id = ? AND role = ?", models.GetJoinTableName(modelObjOwnership))
	stmt := fmt.Sprintf("DELETE FROM %s WHERE model_id = ?", models.GetJoinTableName(modelObjOwnership))

	// Can't do db.Raw and db.Delete at the same time?!
	db2 := db.Exec(stmt, modelObj.GetID().String())
	if err := db2.Error; err != nil {
		return nil, err
	}

	return modelObj, nil
}

func (serv *OwnershipService) HookBeforeDeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	// Delete link table because GORM isn't automatic here when we customize it with UUID or when we have role
	for _, modelObj := range modelObjs {
		// Also remove entries from ownership table
		modelObjOwnership, ok := modelObj.(models.IHasOwnershipLink)
		if !ok {
			return nil, ErrNoOwnership
		}
		stmt := fmt.Sprintf("DELETE FROM %s WHERE model_id = ?", models.GetJoinTableName(modelObjOwnership))
		db2 := db.Exec(stmt, modelObj.GetID().String())
		if err := db2.Error; err != nil {
			return nil, err
		}
	}
	return modelObjs, nil
}

// getOneWithIDCore get one model object based on its type and its id string
func (serv *OwnershipService) GetOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	modelObj := models.NewFromTypeString(typeString)

	db = db.Set("gorm:auto_preload", true)

	rtable := models.GetTableNameFromIModel(modelObj)

	/*
		SELECT * from some_model
		INNER JOIN user_owns_somemodel ON somemodel.id = user_owns_somemodel.model_id AND somemodel.id = UUID_TO_BIN(id)
		INNER JOIN user ON user.id = user_owns_somemodel.user_id AND user.id = UUID_TO_BIN(oid)
	*/

	modelObjOwnership, ok := modelObj.(models.IHasOwnershipLink)
	if !ok {
		return nil, 0, ErrNoOwnership
	}

	joinTableName := models.GetJoinTableName(modelObjOwnership)

	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id AND \"%s\".id = ?", joinTableName, rtable, joinTableName, rtable)
	secondJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)
	// db2 := db

	err := db.Table(rtable).Joins(firstJoin, id.String()).Joins(secondJoin, oid.String()).Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	joinTable := reflect.New(modelObjOwnership.OwnershipType()).Interface()
	role := models.Invalid // just some default
	stmt := fmt.Sprintf("SELECT * FROM %s WHERE user_id = ? AND model_id = ?", joinTableName)
	if err2 := db.Raw(stmt, oid.String(), id.String()).Scan(joinTable).Error; err2 != nil {
		return nil, 0, err2
	}

	if m, ok := joinTable.(models.IOwnership); ok {
		role = m.GetRole()
	}

	err = gormfixes.LoadManyToManyBecauseGormFailsWithID(db, modelObj)
	if err != nil {
		return nil, 0, err
	}

	return modelObj, role, err
}

func (serv *OwnershipService) GetManyWithIDsCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
	// If I can load it, I have permission to edit it. So no need to call loadAndCheckErrorBeforeModify
	// like when I do for update. Just get the role and check if it's admin
	rtable, joinTableName, err := getModelTableNameAndJoinTableNameFromTypeString(typeString)
	if err != nil {
		return nil, nil, err
	}

	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id AND \"%s\".id IN (?)", joinTableName, rtable, joinTableName, rtable)
	secondJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)

	db2 := db.Table(rtable).Joins(firstJoin, ids).Joins(secondJoin, oid)
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

	// Check error
	// Load the roles and check if they're admin
	roles := make([]models.UserRole, 0)
	if err := db2.Select(fmt.Sprintf("\"%s\".\"role\"", joinTableName)).Scan(&roles).Error; err != nil {
		log.Printf("err getting roles")
		return nil, nil, err
	}

	for _, modelObj := range modelObjs {
		err = gormfixes.LoadManyToManyBecauseGormFailsWithID(db, modelObj)
		if err != nil {
			return nil, nil, err
		}
	}

	return modelObjs, roles, nil
}

func (serv *OwnershipService) GetAllCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string) ([]models.IModel, []models.UserRole, error) {
	rtable, joinTableName, err := getModelTableNameAndJoinTableNameFromTypeString(typeString)
	if err != nil {
		return nil, nil, err
	}

	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id", joinTableName, rtable, joinTableName)
	secondJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)
	db = db.Table(rtable).Joins(firstJoin).Joins(secondJoin, oid.String())

	// db3 := db
	roles := make([]models.UserRole, 0)
	outmodels, err := models.NewSliceFromDBByTypeString(typeString, db.Find) // error from db is returned from here
	if err != nil {
		return nil, nil, err
	}

	// ---------------------------
	// ownershipModelTyp := getOwnershipModelTypeFromTypeString(typeString)

	// role := models.Admin // just some default
	// The difference between this method and the find is that it's missing the
	// WHERE "model"."deleted_at" IS NULL, so we need to add it
	if err = db.Where(fmt.Sprintf("\"%s\".\"deleted_at\" IS NULL", rtable)).
		Select(fmt.Sprintf("\"%s\".\"role\"", joinTableName)).Scan(&roles).Error; err != nil {
		return nil, nil, err
	}

	return outmodels, roles, nil
}
