package datamapper

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/security"
	"github.com/t2wu/betterrest/libs/utils"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/copier"
	"github.com/jinzhu/gorm"
)

// ---------------------------------------

var onceUser sync.Once
var usercrud *UserMapper

// UserMapper is a User CRUD manager
type UserMapper struct {
	Service service.IService
}

// SharedUserMapper creats a singleton of Crud object
func SharedUserMapper() *UserMapper {
	onceUser.Do(func() {
		usercrud = &UserMapper{Service: &service.UserService{BaseService: service.BaseService{}}}
	})

	return usercrud
}

// ISendVerificationEmailHandler for sending verification email
// (Should be impelemented in model)
// This is for user model
type ISendVerificationEmail interface {
	SendVerificationEmail(db *gorm.DB, who models.Who,
		typeString string, modelobj models.IModel, actionType datatypes.VerificationActionType) error
}

// ISendVerificationEmailHandler for sending verification email
// (Should be impelemented in model)
// This is for user model
type ISendPasswordResetEmail interface {
	SendPasswordResetEmail(db *gorm.DB, who models.Who,
		typeString string, modelobj models.IModel) error
}

//------------------------
// User specific CRUD
// Cuz user is spcial, need to create ownership and no need to check for owner
// ------------------------------------

// CreateOne creates an user model based on json and store it in db
// Also creates a ownership with admin access
func (mapper *UserMapper) CreateOne(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel) (models.IModel, error) {
	modelObj, err := mapper.Service.HookBeforeCreateOne(db, who, typeString, modelObj)
	if err != nil {
		return nil, err
	}

	verificationURL, veriOK := reflect.ValueOf(modelObj).Elem().FieldByName("VerificationURL").Interface().(*string)
	if veriOK && verificationURL != nil {
		// Do verfication code
		modelObj = mapper.setCodeAndExpiryDate(modelObj)
		modelObj = mapper.setVerficationActionType(modelObj, datatypes.VerificationActionTypeVerifyEmail)
	} else {
		// No verification needed, go right ahead
		reflect.ValueOf(modelObj).Elem().FieldByName("Status").Set(reflect.ValueOf(models.UserStatusActive))
	}

	var before, after *string
	if _, ok := modelObj.(models.IBeforeCreate); ok {
		*before = "BeforeCreateDB"
	}
	if _, ok := modelObj.(models.IAfterCreate); ok {
		*after = "AfterCreateDB"
	}
	j := opJob{
		serv:       mapper.Service,
		db:         db,
		who:        who,
		typeString: typeString,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,
		crupdOp:  models.CRUPDOpCreate,
	}

	modelObj2, err := opCore(before, after, j, mapper.Service.CreateOneCore)
	if err != nil {
		return modelObj2, err
	}

	if veriOK && verificationURL != nil {
		if v, ok := modelObj2.(ISendVerificationEmail); ok {
			if err := v.SendVerificationEmail(db, who, typeString, modelObj2, datatypes.VerificationActionTypeVerifyEmail); err != nil {
				return modelObj2, err
			}
		} else {
			return modelObj2, errors.New("user model should satisfy ISendVerificationEmail interface")
		}
	}
	return modelObj2, nil
}

// // CreateMany is currently a dummy
// func (mapper *UserMapper) CreateMany(db *gorm.DB, who models.Who, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
// 	// not really implemented
// 	return nil, errors.New("not implemented")
// }

// GetOneWithID get one model object based on its type and its id string
func (mapper *UserMapper) GetOneWithID(db *gorm.DB, who models.Who, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	log.Println("GetOneWithID...................")
	modelObj, role, err := mapper.Service.GetOneWithIDCore(db, who, typeString, id)
	if err != nil {
		return nil, 0, err
	}

	if m, ok := modelObj.(models.IAfterRead); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Role: &role}
		if err := m.AfterReadDB(hpdata); err != nil {
			return nil, 0, err
		}
	}

	return modelObj, role, nil
}

// UpdateOneWithID updates model based on this json
// Update DOESN'T change password. It'll load up the password hash and save the same.
// Update password require special endpoint
func (mapper *UserMapper) UpdateOneWithID(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel, id *datatypes.UUID) (models.IModel, error) {
	log.Println("userMapper's UpdateOneWithID called")
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, modelObj, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	modelObj, err = preserveEmailPassword(db, who.Oid, modelObj)
	if err != nil {
		return nil, err
	}

	reflect.ValueOf(modelObj).Elem().FieldByName("Status").Set(reflect.ValueOf(models.UserStatusActive))

	cargo := models.ModelCargo{}

	// Before hook
	if v, ok := modelObj.(models.IBeforeUpdate); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: &cargo}
		if err := v.BeforeUpdateDB(hpdata); err != nil {
			return nil, err
		}
	}

	modelObj2, err := mapper.Service.UpdateOneCore(db, who, typeString, modelObj, id, oldModelObj)
	if err != nil {
		return nil, err
	}

	// After hook
	if v, ok := modelObj2.(models.IAfterUpdate); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: &cargo}
		if err = v.AfterUpdateDB(hpdata); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}

// DeleteOneWithID deletes the user with the ID
func (mapper *UserMapper) DeleteOneWithID(db *gorm.DB, who models.Who, typeString string, id *datatypes.UUID) (models.IModel, error) {
	modelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, nil, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
	}

	modelObj, err = mapper.Service.HookBeforeDeleteOne(db, who, typeString, modelObj)
	if err != nil {
		return nil, err
	}

	var before, after *string
	if _, ok := modelObj.(models.IBeforeDelete); ok {
		b := "BeforeDeleteDB"
		before = &b
	}
	if _, ok := modelObj.(models.IAfterDelete); ok {
		a := "AfterDeleteDB"
		after = &a
	}

	j := opJob{
		serv:       mapper.Service,
		db:         db,
		who:        who,
		typeString: typeString,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,
		crupdOp:  models.CRUPDOpDelete,
	}

	return opCore(before, after, j, mapper.Service.DeleteOneCore)
}

// SendEmailResetPassword :-
// Calling this allow a way to change password by the verification code
// without logging in. But doens't mean the password is already invalidated
func (mapper *UserMapper) SendEmailResetPassword(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel) error {
	// TODO: shouldn't just do a fieldby name
	email := reflect.ValueOf(modelObj).Elem().FieldByName("Email").Interface().(string)
	if err := db.Model(modelObj).Where("email = ?", email).First(modelObj).Error; err != nil {
		return err
	}

	modelObj = mapper.setCodeAndExpiryDate(modelObj)
	modelObj = mapper.setVerficationActionType(modelObj, datatypes.VerificationActionTypeResetPassword)
	if err := db.Save(modelObj).Error; err != nil {
		return err
	}

	if v, ok := modelObj.(ISendVerificationEmail); ok {
		return v.SendVerificationEmail(db, who, typeString, modelObj, datatypes.VerificationActionTypeResetPassword)
	}

	return errors.New("user model does not conform to ISendVerificationEmail")
}

func (mapper *UserMapper) ResetPassword(db *gorm.DB, typeString string, modelObj models.IModel, id *datatypes.UUID, code string) error {
	// modelObj is user (typeString is "users")
	modelObj2 := models.NewFromTypeString(typeString)
	if err := db.Model(modelObj2).Where("id = ? AND verification_code = ? AND verification_action = ?", id, code, datatypes.VerificationActionTypeResetPassword).Find(modelObj2).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("verification code incorrect")
		}
		return err
	}

	if actionType := reflect.ValueOf(modelObj2).Elem().FieldByName("VerificationAction").Interface().(datatypes.VerificationActionType); actionType != datatypes.VerificationActionTypeResetPassword {
		return fmt.Errorf("no code for verification")
	}

	expiry := reflect.ValueOf(modelObj2).Elem().FieldByName("VerificationExpiredAt").Interface().(*time.Time)
	if time.Now().Sub(*expiry) > 0 { // expired
		return fmt.Errorf("expired")
	}

	password := reflect.ValueOf(modelObj).Elem().FieldByName("Password").Interface().(string)
	if password == "" {
		return fmt.Errorf("missing password")
	}

	hash, err := security.HashAndSalt(password)
	if err != nil {
		return err
	}

	reflect.ValueOf(modelObj2).Elem().FieldByName("PasswordHash").Set(reflect.ValueOf(hash))
	reflect.ValueOf(modelObj2).Elem().FieldByName("VerificationAction").Set(reflect.ValueOf(datatypes.VerificationActionTypeNone))
	// How to set VerificationExpiredAt nil?
	if err := db.Save(modelObj2).Error; err != nil {
		return err
	}

	return nil
}

func (mapper *UserMapper) SendEmailVerification(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel) error {
	// TODO: shouldn't just do a fieldby name
	email := reflect.ValueOf(modelObj).Elem().FieldByName("Email").Interface().(string)
	if err := db.Model(modelObj).Where("email = ?", email).First(modelObj).Error; err != nil {
		return err
	}

	modelObj = mapper.setCodeAndExpiryDate(modelObj)
	modelObj = mapper.setVerficationActionType(modelObj, datatypes.VerificationActionTypeVerifyEmail)
	if err := db.Save(modelObj).Error; err != nil {
		return err
	}

	if v, ok := modelObj.(ISendVerificationEmail); ok {
		return v.SendVerificationEmail(db, who, typeString, modelObj, datatypes.VerificationActionTypeVerifyEmail)
	}

	return errors.New("user model does not conform to ISendVerificationEmail")
}

func (mapper *UserMapper) VerifyEmail(db *gorm.DB, typeString string, id *datatypes.UUID, code string) error {
	// modelObj is user (typeString is "users")
	modelObj := models.NewFromTypeString(typeString)
	if err := db.Model(modelObj).Where("id = ? AND verification_code = ? AND verification_action = ?", id, code, datatypes.VerificationActionTypeVerifyEmail).Find(modelObj).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("verification code incorrect")
		}
		return err
	}

	if actionType := reflect.ValueOf(modelObj).Elem().FieldByName("VerificationAction").Interface().(datatypes.VerificationActionType); actionType != datatypes.VerificationActionTypeVerifyEmail {
		return fmt.Errorf("no code for verification")
	}

	expiry := reflect.ValueOf(modelObj).Elem().FieldByName("VerificationExpiredAt").Interface().(*time.Time)
	if time.Now().Sub(*expiry) > 0 { // expired
		return fmt.Errorf("verification code expired")
	}

	reflect.ValueOf(modelObj).Elem().FieldByName("Status").Set(reflect.ValueOf(models.UserStatusActive))
	if err := db.Save(modelObj).Error; err != nil {
		return err
	}
	return nil
}

// ChangeEmailPasswordWithID changes email and/or password
func (mapper *UserMapper) ChangeEmailPasswordWithID(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel, id *datatypes.UUID) (models.IModel, error) {
	// This will require that it has an ID, but in changing email there isn't
	// oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, modelObj, id, []models.UserRole{models.UserRoleAdmin})
	// if err != nil {
	// 	return nil, err
	// }

	// Verify the password in the current modelObj
	oldModelObj, err := security.GetVerifiedAuthUser(db, modelObj)
	if err != nil {
		// unable to login user. maybe doesn't exist?
		// or username, password wrong
		return nil, err
	}

	newModel := models.NewFromTypeString(typeString)
	copier.Copy(newModel, oldModelObj)

	modelObj.SetID(oldModelObj.GetID())

	// Hash the new password (assume already the correct format and length)
	if newPassword := reflect.ValueOf(modelObj).Elem().FieldByName("NewPassword").Interface().(string); newPassword != "" {
		hash, err := security.HashAndSalt(newPassword)
		if err != nil {
			return nil, err
		}
		reflect.ValueOf(newModel).Elem().FieldByName("PasswordHash").Set(reflect.ValueOf(hash))
		// reflect.ValueOf(modelObj).Elem().FieldByName("Password").Set(reflect.ValueOf("")) // just in case user mess up
		// reflect.ValueOf(modelObj).Elem().FieldByName("NewPassword").Set(reflect.ValueOf("")) // just in case
	} else {
		log.Println("no new password")
	}

	if newEmail := reflect.ValueOf(modelObj).Elem().FieldByName("NewEmail").Interface().(string); newEmail != "" {
		email := reflect.ValueOf(oldModelObj).Elem().FieldByName("Email").Interface().(string)

		// Override email with newemail
		reflect.ValueOf(newModel).Elem().FieldByName("Email").Set(reflect.ValueOf(newEmail))
		if newEmail != email {
			reflect.ValueOf(newModel).Elem().FieldByName("Status").Set(reflect.ValueOf(models.UserStatusUnverified))
		}
	} else {
		log.Println("no new email")
	}

	cargo := models.ModelCargo{}

	// Before hook
	if v, ok := newModel.(models.IBeforePasswordChange); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: &cargo}
		if err := v.BeforePasswordChange(hpdata); err != nil {
			return nil, err
		}
	}

	modelObj2, err := mapper.Service.UpdateOneCore(db, who, typeString, newModel, id, oldModelObj)
	if err != nil {
		return nil, err
	}

	// After hook
	if v, ok := modelObj2.(models.IAfterPasswordChange); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: &cargo}
		if err = v.AfterPasswordChange(hpdata); err != nil {
			log.Println("AfterPasswordUpdateDB error returns:", err)
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

// CreateMany :-
func (mapper *UserMapper) CreateMany(db *gorm.DB, who models.Who, typeString string, modelObj []models.IModel) ([]models.IModel, error) {
	return nil, fmt.Errorf("Not implemented")
}

// GetAll :-
func (mapper *UserMapper) GetAll(db *gorm.DB, who models.Who, typeString string, options map[URLParam]interface{}) ([]models.IModel, []models.UserRole, *int, error) {
	return nil, nil, nil, fmt.Errorf("Not implemented")
}

// UpdateMany :-
func (mapper *UserMapper) UpdateMany(db *gorm.DB, who models.Who, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return nil, fmt.Errorf("Not implemented")
}

// PatchMany :-
func (mapper *UserMapper) PatchMany(db *gorm.DB, who models.Who,
	typeString string, jsonIDPatches []models.JSONIDPatch) ([]models.IModel, error) {
	return nil, fmt.Errorf("Not implemented")
}

// DeleteMany :-
func (mapper *UserMapper) DeleteMany(db *gorm.DB, who models.Who,
	typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return nil, fmt.Errorf("Not implemented")
}

// PatchOneWithID :-
func (mapper *UserMapper) PatchOneWithID(db *gorm.DB, who models.Who,
	typeString string, jsonPatch []byte, id *datatypes.UUID) (models.IModel, error) {
	return nil, fmt.Errorf("Not implemented, todo")
}

// Private -------------------------------------------------------------------------
func (mapper *UserMapper) setCodeAndExpiryDate(modelObj models.IModel) models.IModel {
	code := utils.RandStringBytesMaskImprSrcUnsafe(12)
	reflect.ValueOf(modelObj).Elem().FieldByName("VerificationCode").Set(reflect.ValueOf(code))

	expiry := time.Now().Add(time.Duration(time.Hour * 24 * 3))
	reflect.ValueOf(modelObj).Elem().FieldByName("VerificationExpiredAt").Set(reflect.ValueOf(&expiry))
	return modelObj
}

func (mapper *UserMapper) setVerficationActionType(modelObj models.IModel, t datatypes.VerificationActionType) models.IModel {
	reflect.ValueOf(modelObj).Elem().FieldByName("VerificationAction").Set(reflect.ValueOf(t))
	return modelObj
}
