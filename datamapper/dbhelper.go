package datamapper

import (
	"log"
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// How about AOP?
// https://github.com/gogap/aop

// createOneWithHooks handles before and after DB hookpoints for create
// func createOneWithHooks(createOneCore func(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel) (models.IModel, error), db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
// 	var err error
// 	var cargo models.ModelCargo

// 	if v, ok := modelObj.(models.IBeforeCreate); ok {
// 		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
// 		err = v.BeforeInsertDB(hpdata)
// 		if err != nil {
// 			return nil, err
// 		}
// 	}

// 	modelObj, err = createOneCore(db, oid, typeString, modelObj)
// 	if err != nil {
// 		return nil, err
// 	}

// 	if v, ok := modelObj.(models.IAfterCreate); ok {
// 		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
// 		err = v.AfterInsertDB(hpdata)
// 		if err != nil {
// 			return nil, err
// 		}
// 	}

// 	return modelObj, nil
// }

// createOneCoreOwnership creates a model
func createOneCoreOwnership(mapper IGetOneWithIDMapper, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (models.IModel, error) {
	// It looks like I need to explicitly call create here
	o := reflect.ValueOf(modelObj).Elem().FieldByName("Ownerships")
	g, _ := o.Index(0).Addr().Interface().(models.IOwnership)

	// No need to check if primary key is blank.
	// If it is it'll be created by Gorm's BeforeCreate hook
	// (defined in base model)
	// if dbc := db.Create(modelObj); dbc.Error != nil {
	if dbc := db.Create(modelObj).Create(g); dbc.Error != nil {
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

// createOneCoreBasic creates a user
func createOneCoreBasic(mapper IGetOneWithIDMapper, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (models.IModel, error) {
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

// updateOneCore one, permissin should already be checked
// called for patch operation as well (after patch has already applied)
func updateOneCore(mapper IGetOneWithIDMapper, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (modelObj2 models.IModel, err error) {
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
	modelObj2, _, err = mapper.getOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil { // Error is "record not found" when not found
		log.Println("Error:", err)
		return nil, err
	}

	// ouch! for many to many we need to remove it again!!
	// because it's in a transaction so it will load up again
	gormfixes.FixManyToMany(modelObj, modelObj2)

	return modelObj2, nil
}

func deleteOneCore(mapper IGetOneWithIDMapper, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObjs models.IModel) (models.IModel, error) {
	// Many field is not used, it's just used to conform the interface
	if err := db.Delete(modelObj).Error; err != nil {
		return nil, err
	}

	if err := gormfixes.RemovePeggedField(db, modelObj); err != nil {
		return nil, err
	}

	return modelObj, nil
}

type batchOpJob struct {
	mapper       IGetOneWithIDMapper
	db           *gorm.DB
	oid          *datatypes.UUID
	scope        *string
	typeString   string
	oldmodelObjs []models.IModel // use for update (need to load and override for pegged fields)
	modelObjs    []models.IModel // current field value from the user if update, or from the loaded field if delete
}

func batchOpCore(job batchOpJob,
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error,
	taskFunc func(mapper IGetOneWithIDMapper, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (models.IModel, error),
) ([]models.IModel, error) {

	mapper, db, oid, scope, typeString, modelObjs, oldmodelObjs :=
		job.mapper, job.db, job.oid, job.scope, job.typeString, job.modelObjs, job.oldmodelObjs

	ms := make([]models.IModel, len(modelObjs))
	cargo := models.BatchHookCargo{}

	// Before batch update hookpoint
	// if before := models.ModelRegistry[typeString].BeforeUpdate; before != nil {
	if before != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := before(bhpData); err != nil {
			return nil, err
		}
	}

	// TODO: Could update all at once, then load all at once again
	for i, modelObj := range modelObjs {
		id := modelObj.GetID()

		// m, err := updateOneCore(mapper, db, oid, scope, typeString, modelObj, id)
		m, err := taskFunc(mapper, db, oid, scope, typeString, modelObj, id, oldmodelObjs[i])
		if err != nil { // Error is "record not found" when not found
			return nil, err
		}

		ms[i] = m
	}

	// After batch update hookpoint
	// if after := models.ModelRegistry[typeString].AfterUpdate; after != nil {
	if after != nil {
		bhpData := models.BatchHookPointData{Ms: ms, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := after(bhpData); err != nil {
			return nil, err
		}
	}

	return ms, nil
}

type opJob struct {
	mapper      IGetOneWithIDMapper
	db          *gorm.DB
	oid         *datatypes.UUID
	scope       *string
	typeString  string
	oldModelObj models.IModel // use for update (need to load and override for pegged fields)
	modelObj    models.IModel // current field value from the user if update, or from the loaded field if delete
}

func opCore(before func(hpdata models.HookPointData) error,
	after func(hpdata models.HookPointData) error,
	job opJob,
	taskFun func(mapper IGetOneWithIDMapper, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (models.IModel, error),
) (models.IModel, error) {
	mapper, db, oid, scope, typeString, oldModelObj, modelObj := job.mapper, job.db,
		job.oid, job.scope, job.typeString, job.oldModelObj, job.modelObj

	cargo := models.ModelCargo{}

	// Before hook
	// It is now expected that the hookpoint for before expect that the patch
	// gets applied to the JSON, but not before actually updating to DB.
	if before != nil {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := before(hpdata); err != nil {
			return nil, err
		}
	}

	// Now do the task
	id := modelObj.GetID()
	modelObj2, err := taskFun(mapper, db, oid, scope, typeString, modelObj, id, oldModelObj)
	if err != nil {
		return nil, err
	}

	// After hook
	if after != nil {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = after(hpdata); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}
