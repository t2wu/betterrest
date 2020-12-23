package security

import (
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/betterrest/models"
)

// VerifyUserResult type with enum
type VerifyUserResult int

const (
	// VerifyUserResultOK found username and password
	VerifyUserResultOK VerifyUserResult = iota
	// VerifyUserResultPasswordNotMatch password does not match
	VerifyUserResultPasswordNotMatch
	// VerifyUserResultEmailNotFound email is not found
	VerifyUserResultEmailNotFound
	// VerifyUserResultOtherError other error
	VerifyUserResultOtherError
)

// GetVerifiedAuthUser authenticates the user
func GetVerifiedAuthUser(userModel models.IModel) (models.IModel, VerifyUserResult) {
	userModel2 := reflect.New(models.UserTyp).Interface().(models.IModel)

	// TODO: maybe email is not the login, make it more flexible?
	email := reflect.ValueOf(userModel).Elem().FieldByName(("Email")).Interface().(string)
	password := reflect.ValueOf(userModel).Elem().FieldByName(("Password")).Interface().(string)

	err := db.Shared().Where("email = ?", email).First(userModel2).Error
	if gorm.IsRecordNotFoundError(err) {
		return nil, VerifyUserResultEmailNotFound // User doesn't exists with this email
	} else if err != nil {
		// Some other unknown error
		return nil, VerifyUserResultOtherError
	}

	passwordHash := reflect.ValueOf(userModel2).Elem().FieldByName("PasswordHash").Interface().(string)
	if !IsSamePassword(password, passwordHash) {
		// Password doesn't match
		return userModel2, VerifyUserResultPasswordNotMatch
	}

	return userModel2, VerifyUserResultOK
}
