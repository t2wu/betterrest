package service

import (
	"errors"
	"fmt"
	"log"
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/security"
	"github.com/t2wu/betterrest/models"
)

type UserService struct {
}

func (serv *UserService) HookBeforeCreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
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

	return modelObj, nil
}

func (serv *UserService) HookBeforeCreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return nil, errors.New("not implemented")
}

func (serv *UserService) HookBeforeDeleteOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	return nil, errors.New("not implemented") // probably can
}

func (serv *UserService) HookBeforeDeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return nil, errors.New("not implemented")
}

// GetOneWithIDCore get one model object based on its type and its id string
// getOneWithID get one model object based on its type and its id string without invoking read hookpoing
func (serv *UserService) GetOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	// TODO: Currently can only read ID from your own (not others in the admin group either)
	db = db.Set("gorm:auto_preload", true)

	// Todo: maybe guest shoud be able to read some fields
	if id.String() != oid.String() {
		return nil, 0, ErrPermission
	}

	modelObj := models.NewFromTypeString(typeString)
	modelObj.SetID(oid)

	if err := db.First(modelObj).Error; err != nil {
		return nil, 0, err
	}

	return modelObj, models.Admin, nil
}

func (serv *UserService) GetManyWithIDsCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
	return nil, nil, fmt.Errorf("Not implemented")
}

func (serv *UserService) GetAllCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string) ([]models.IModel, []models.UserRole, error) {
	return nil, nil, fmt.Errorf("Not implemented")
}
