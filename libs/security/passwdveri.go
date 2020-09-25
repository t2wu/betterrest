package security

import (
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/betterrest/models"
)

// GetVerifiedAuthUser authenticates the user
func GetVerifiedAuthUser(userModel models.IModel) (models.IModel, bool) {
	userModel2 := reflect.New(models.UserTyp).Interface().(models.IModel)

	// TODO: maybe email is not the login, make it more flexible?
	email := reflect.ValueOf(userModel).Elem().FieldByName(("Email")).Interface().(string)
	password := reflect.ValueOf(userModel).Elem().FieldByName(("Password")).Interface().(string)

	err := db.Shared().Where("email = ?", email).First(userModel2).Error
	if gorm.IsRecordNotFoundError(err) {
		return nil, false // User doesn't exists with this email
	} else if err != nil {
		// Some other unknown error
		return nil, false
	}

	passwordHash := reflect.ValueOf(userModel2).Elem().FieldByName("PasswordHash").Interface().(string)
	if !IsSamePassword(password, passwordHash) {
		// Password doesn't match
		return nil, false
	}

	return userModel2, true
}
