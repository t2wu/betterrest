package service

import (
	"errors"
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/betterrest/registry"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"
)

type UserService struct {
	BaseServiceV1
}

func (serv *UserService) HookBeforeCreateOne(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel) (mdl.IModel, error) {
	// Special case, there is really no oid in this case, user doesn't exist yet

	// Do nothing

	return modelObj, nil
}

func (serv *UserService) HookBeforeCreateMany(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObjs []mdl.IModel) ([]mdl.IModel, error) {
	return nil, errors.New("not implemented")
}

func (serv *UserService) HookBeforeDeleteOne(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel) (mdl.IModel, error) {
	return modelObj, nil // looks like nothing to do
}

func (serv *UserService) HookBeforeDeleteMany(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObjs []mdl.IModel) ([]mdl.IModel, error) {
	return nil, errors.New("not implemented")
}

// ReadOneCore get one model object based on its type and its id string
// ReadOne get one model object based on its type and its id string without invoking read hookpoing
func (serv *UserService) ReadOneCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, id *datatype.UUID) (mdl.IModel, userrole.UserRole, error) {
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

	return modelObj, userrole.UserRoleAdmin, nil
}

func (serv *UserService) GetManyCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, ids []*datatype.UUID) ([]mdl.IModel, []userrole.UserRole, error) {
	return nil, nil, fmt.Errorf("Not implemented")
}

// GetAllQueryContructCore :-
func (serv *UserService) GetAllQueryContructCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string) (*gorm.DB, error) {
	return nil, fmt.Errorf("Not implemented")
}

// GetAllRolesCore :-
func (serv *UserService) GetAllRolesCore(dbChained *gorm.DB, dbClean *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObjs []mdl.IModel) ([]userrole.UserRole, error) {
	return nil, fmt.Errorf("Not implemented")
}

// UpdateOneCore one, permissin should already be checked
// called for patch operation as well (after patch has already applied)
// Fuck, repeat the following code for now (you can't call the overriding method from the non-overriding one)
func (serv *UserService) UpdateOneCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, oldModelObj mdl.IModel) (modelObj2 mdl.IModel, err error) {
	if modelNeedsRealDelete(oldModelObj) { // parent model
		db = db.Unscoped()
	}

	err = gormfixes.UpdatePeggedFields(db, oldModelObj, modelObj)
	if err != nil {
		return nil, err
	}

	if err = db.Save(modelObj).Error; err != nil { // save updates all fields (FIXME: need to check for required)
		log.Println("Error updating:", err)
		return nil, err
	}

	// This loads the IDs
	// This so we have the preloading.
	modelObj2, _, err = serv.ReadOneCore(db, who, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return nil, err
	}

	// ouch! for many to many we need to remove it again!!
	// because it's in a transaction so it will load up again
	gormfixes.FixManyToMany(modelObj, modelObj2)

	return modelObj2, nil
}
