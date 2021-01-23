package datamapper

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"sync"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/security"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/gorm"
)

// ---------------------------------------

// createOneCoreUserMapper creates a user
func createOneCoreUserMapper(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel) (models.IModel, error) {
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
	if err := createPeggedAssocFields(db, modelObj); err != nil {
		return nil, err
	}

	// For table with trigger which update before insert, we need to load it again
	if err := db.First(modelObj).Error; err != nil {
		// That's weird. we just inserted it.
		return nil, err
	}

	return modelObj, nil
}

// ---------------------------------------

var onceUser sync.Once
var usercrud *UserMapper

// UserMapper is a User CRUD manager
type UserMapper struct {
}

// SharedUserMapper creats a singleton of Crud object
func SharedUserMapper() *UserMapper {
	onceUser.Do(func() {
		usercrud = &UserMapper{}
	})

	return usercrud
}

//------------------------
// User specific CRUD
// Cuz user is spcial, need to create ownership and no need to check for owner
// ------------------------------------

// CreateOne creates an user model based on json and store it in db
// Also creates a ownership with admin access
func (mapper *UserMapper) CreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	// Special case, there is really no oid in this case, user doesn't exist yet

	// modelObj is a a User struct, but we cannot do type assertion because library user
	// should define it. If we make an interface with user.Ownership setter and getter,
	// we need to ask library user to define a user.Ownership setter and getter, it's too
	// much hassle
	password := reflect.ValueOf(modelObj).Elem().FieldByName(("Password")).Interface().(string)

	// Additional checking because password should not be blank with create
	if password == "" {
		log.Println("password should not be blank!!!")
		return nil, fmt.Errorf("password should not be blank")
	}

	// field, _ := reflect.TypeOf(modelObj).Elem().FieldByName("Ownerships")
	// ownershipType := field.Type

	// ownership := reflect.ValueOf(modelObj).Elem().FieldByName("Ownerships")
	// ownership.Set(reflect.MakeSlice(reflect.SliceOf(ownershipType), 1, 1))
	// ownership.Index(0).Set(reflect.New(ownershipType).Elem())

	hash, err := security.HashAndSalt(password)
	if err != nil {
		return nil, err
	}

	reflect.ValueOf(modelObj).Elem().FieldByName("PasswordHash").Set(reflect.ValueOf(hash))

	// there isn't really an oid at this point
	return createOneWithHooks(createOneCoreUserMapper, db, oid, scope, typeString, modelObj)
}

// CreateMany is currently a dummy
func (mapper *UserMapper) CreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	// not really implemented
	return nil, errors.New("not implemented")
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *UserMapper) GetOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id datatypes.UUID) (models.IModel, models.UserRole, error) {
	modelObj, role, err := mapper.getOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil {
		return nil, 0, err
	}

	if m, ok := modelObj.(models.IAfterRead); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Role: &role}
		if err := m.AfterReadDB(hpdata); err != nil {
			return nil, 0, err
		}
	}

	return modelObj, role, nil
}

// getOneWithID get one model object based on its type and its id string without invoking read hookpoing
func (mapper *UserMapper) getOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id datatypes.UUID) (models.IModel, models.UserRole, error) {
	// TODO: Currently can only read ID from your own (not others in the admin group either)
	db = db.Set("gorm:auto_preload", true)

	// Todo: maybe guest shoud be able to read some fields
	if id.String() != oid.String() {
		return nil, 0, errPermission
	}

	modelObj := models.NewFromTypeString(typeString)
	modelObj.SetID(oid)

	if err := db.First(modelObj).Error; err != nil {
		return nil, 0, err
	}

	return modelObj, models.Admin, nil
}

// UpdateOneWithID updates model based on this json
// Update DOESN'T change password. It'll load up the password hash and save the same.
// Update password require special endpoint
func (mapper *UserMapper) UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id datatypes.UUID) (models.IModel, error) {
	log.Println("userMapper's UpdateOneWithID called")
	if _, err := loadAndCheckErrorBeforeModify(mapper, db, oid, scope, typeString, modelObj, id, models.Admin); err != nil {
		return nil, err
	}

	if *oid != id {
		return nil, errPermission
	}

	var err error
	modelObj, err = preserveEmailPassword(db, oid, modelObj)
	if err != nil {
		return nil, err
	}

	cargo := models.ModelCargo{}

	// Before hook
	if v, ok := modelObj.(models.IBeforeUpdate); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := v.BeforeUpdateDB(hpdata); err != nil {
			return nil, err
		}
	}

	modelObj2, err := updateOneCore(mapper, db, oid, scope, typeString, modelObj, id, models.Admin)
	if err != nil {
		return nil, err
	}

	// After hook
	if v, ok := modelObj2.(models.IAfterUpdate); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = v.AfterUpdateDB(hpdata); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}

// DeleteOneWithID deletes the user with the ID
func (mapper *UserMapper) DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id datatypes.UUID) (models.IModel, error) {
	if id.UUID.String() == "" {
		return nil, errIDEmpty
	}

	// Pull out entire modelObj
	modelObj, role, err := mapper.GetOneWithID(db, oid, scope, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return nil, err
	}
	if role != models.Admin {
		// even if found, not authorized, so return a not found
		// but how do I do that here?
		return nil, errors.New("not found")
	}

	cargo := models.ModelCargo{}

	// Before delete hookpoint
	if v, ok := modelObj.(models.IBeforeDelete); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		err = v.BeforeDeleteDB(hpdata)
		if err != nil {
			return nil, err
		}
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
	}

	// Unscoped() for REAL delete!
	// Otherwise my constraint won't work...
	// Soft delete will take more work, have to verify myself manually
	// db.Unscoped().Delete(modelObj).Errorf
	err = db.Delete(modelObj).Error
	if err != nil {
		return nil, err
	}

	// After delete hookpoint
	if v, ok := modelObj.(models.IAfterDelete); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		err = v.AfterDeleteDB(hpdata)
		if err != nil {
			return nil, err
		}
	}

	return modelObj, nil
}

// ChangeEmailPasswordWithID changes email and/or password
func (mapper *UserMapper) ChangeEmailPasswordWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id datatypes.UUID) (models.IModel, error) {
	log.Println("userMapper's ChangeEmailPasswordWithID called")
	if _, err := loadAndCheckErrorBeforeModify(mapper, db, oid, scope, typeString, modelObj, id, models.Admin); err != nil {
		return nil, err
	}

	if *oid != id {
		return nil, errPermission
	}

	// Verify the password in the current modelObj
	if _, code := security.GetVerifiedAuthUser(modelObj); code != security.VerifyUserResultOK {
		// unable to login user. maybe doesn't exist?
		// or username, password wrong
		return nil, fmt.Errorf("password incorrect")
	}

	// Hash the new password (assume already the correct format and length)
	newPassword := reflect.ValueOf(modelObj).Elem().FieldByName(("NewPassword")).Interface().(string)
	hash, err := security.HashAndSalt(newPassword)
	if err != nil {
		return nil, err
	}

	newEmail := reflect.ValueOf(modelObj).Elem().FieldByName(("NewEmail")).Interface().(string)

	// Load the original model, then override email and password hash which we want to change
	oldModel, _, err := mapper.getOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil {
		return nil, err
	}
	modelObj = oldModel

	// Override email with newemail
	reflect.ValueOf(modelObj).Elem().FieldByName("Email").Set(reflect.ValueOf(newEmail))

	reflect.ValueOf(modelObj).Elem().FieldByName("PasswordHash").Set(reflect.ValueOf(hash))
	reflect.ValueOf(modelObj).Elem().FieldByName("Password").Set(reflect.ValueOf(""))    // just in case user mess up
	reflect.ValueOf(modelObj).Elem().FieldByName("NewPassword").Set(reflect.ValueOf("")) // just in case

	cargo := models.ModelCargo{}

	// Before hook
	if v, ok := modelObj.(models.IBeforePasswordUpdate); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := v.BeforePasswordUpdateDB(hpdata); err != nil {
			return nil, err
		}
	}

	modelObj2, err := updateOneCore(mapper, db, oid, scope, typeString, modelObj, id, models.Admin)
	if err != nil {
		return nil, err
	}

	// After hook
	if v, ok := modelObj2.(models.IAfterPasswordUpdate); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = v.AfterPasswordUpdateDB(hpdata); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}

func preserveEmailPassword(db *gorm.DB, oid *datatypes.UUID, modelObj models.IModel) (models.IModel, error) {
	// Don't mess with password here, load the password hash
	// Load password hash so we don't override it with blank
	type result struct {
		PasswordHash string
		Email        string
	}
	res := result{}
	rtable := models.GetTableNameFromIModel(modelObj)
	if err := db.Table(rtable).Select([]string{"password_hash", "email"}).Where("id = ?", oid).Scan(&res).Error; err != nil {
		// Doesn't work
		// if err := db.Table(rtable).Select("password_hash", "email").Where("id = ?", oid).Scan(&res).Error; err != nil {
		log.Println("Fetching passwordhash and email problem:", err)
		return nil, err
	}

	// Override modelObj because this endpoint does not allow changing password and email
	reflect.ValueOf(modelObj).Elem().FieldByName("PasswordHash").Set(reflect.ValueOf(res.PasswordHash))
	reflect.ValueOf(modelObj).Elem().FieldByName("Email").Set(reflect.ValueOf(res.Email))

	// Erase passwords fields in case user makes a mistake and forgot gorm:"-" and save them to db
	reflect.ValueOf(modelObj).Elem().FieldByName("Password").Set(reflect.ValueOf(""))    // just in case user mess up
	reflect.ValueOf(modelObj).Elem().FieldByName("NewPassword").Set(reflect.ValueOf("")) // just in case

	return modelObj, nil
}
