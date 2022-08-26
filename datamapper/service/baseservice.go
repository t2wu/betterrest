package service

import (
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/qry"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"
)

// IService provice basic data fetch for various type of table to user relationships
type IService interface {
	// ContructPermissionQuery adds permission query for query one
	// ContructPermissionClauseForOne(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel) (*gorm.DB, error)

	// ContructPermissionQuery adds permission query for query many
	// ContructPermissionClauseForMany(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObjs []mdl.IModel) (*gorm.DB, error)

	// HookBeforeCreateOne(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel) (mdl.IModel, error)
	// HookBeforeCreateMany(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObjs []mdl.IModel) ([]mdl.IModel, error)
	HookBeforeDeleteOne(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel) (mdl.IModel, error)
	HookBeforeDeleteMany(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObjs []mdl.IModel) ([]mdl.IModel, error)

	CreateOneCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, oldModelObj mdl.IModel) (mdl.IModel, error)
	ReadOneCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, id *datatype.UUID, options map[urlparam.Param]interface{}) (mdl.IModel, userrole.UserRole, error)
	UpdateOneCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, oldModelObj mdl.IModel) (modelObj2 mdl.IModel, err error)
	DeleteOneCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, oldModelObjs mdl.IModel) (mdl.IModel, error)

	GetManyCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, ids []*datatype.UUID, options map[urlparam.Param]interface{}) ([]mdl.IModel, []userrole.UserRole, error)

	GetAllQueryContructCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string) (*gorm.DB, error)
	GetAllRolesCore(dbChained *gorm.DB, dbClean *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObjs []mdl.IModel) ([]userrole.UserRole, error)

	// Permitted is asking if this user has permission to add to it
	// 1. Ownership service: ask hook for permission, if the endpoint is allowed, then the role for the created item is admin
	// 2. Link service: If you're an admin or someone with permission to edit it (if hook allows it), then the role you can set is also restricted by the hook.
	// 3. Org/Org partition service(things under org): if you're admin or some permission which allows to write, determined by the hook, then the role
	// then no new role is needed
	// 4. User/Global: Either you have permission to edit or not, hook can determine that. No role needed.
	// Some permission stuff or role linking stuff such as ownership's model has extra stuffs such as associated link table's info).
	// So data.Ms itself can be modified, so a new set is returned
	PermissionAndRole(data *hook.Data, ep *hook.EndPoint) (*hook.Data, *webrender.RetError)
}

// BaseService is the superclass of all services
type BaseService struct {
}

// CreateOneCore create the model
func (serv *BaseService) CreateOneCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, oldModelObj mdl.IModel) (mdl.IModel, error) {
	// No need to check if primary key is blank.
	// If it is it'll be created by Gorm's BeforeCreate hook
	// (defined in base model)

	// Calling query model fix pegged struct's ID check (cannot be pre-existing)
	// And also create update pegged associated fields
	if err := qry.DB(db).Create(modelObj).Error(); err != nil {
		return nil, err
	}

	// For pegassociated, the since we expect association_autoupdate:false
	// need to manually create it
	// if err := gormfixes.CreatePeggedAssocFields(db, modelObj); err != nil {
	// 	return nil, err
	// }

	// For table with trigger which update before insert, we need to load it again
	if err := db.Take(modelObj).Error; err != nil {
		// That's weird. we just inserted it.
		return nil, err
	}

	return modelObj, nil
}

// UpdateOneCore one, permissin should already be checked
// called for patch operation as well (after patch has already applied)
// Fuck, repeat the following code for now (you can't call the overriding method from the non-overriding one)
func (serv *BaseService) UpdateOneCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, oldModelObj mdl.IModel) (modelObj2 mdl.IModel, err error) {
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

	// Kind of hack, but update don't have any parameter anyway
	// This was for parittioned table where read has to have some date
	options := make(map[urlparam.Param]interface{}, 0)

	// This loads the IDs
	// This so we have the preloading.
	modelObj2, _, err = serv.ReadOneCore(db, who, typeString, id, options)
	if err != nil { // Error is "record not found" when not found
		return nil, err
	}

	// ouch! for many to many we need to remove it again!!
	// because it's in a transaction so it will load up again
	gormfixes.FixManyToMany(modelObj, modelObj2)

	return modelObj2, nil
}

func (serv *BaseService) DeleteOneCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, oldModelObjs mdl.IModel) (mdl.IModel, error) {
	// Many field is not used, it's just used to conform the interface
	if err := db.Delete(modelObj).Error; err != nil {
		return nil, err
	}

	if err := gormfixes.DeleteModelFixManyToManyAndPeg(db, modelObj); err != nil {
		return nil, err
	}

	return modelObj, nil
}

func (serv *BaseService) ReadOneCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, id *datatype.UUID, options map[urlparam.Param]interface{}) (mdl.IModel, userrole.UserRole, error) {
	return nil, userrole.UserRoleInvalid, fmt.Errorf("ReadOneCore to be overrridden shouldn't be called")
}
