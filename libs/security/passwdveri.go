package security

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/models"
)

var ErrPasswordIncorrect = errors.New("password incorrect")
var ErrEmailNotFound = errors.New("email not found")
var ErrNotVerified = errors.New("email not verified")
var ErrInactive = errors.New("account inactive")

// VerifyUserResult type with enum
// type VerifyUserResult int

// const (
// 	// VerifyUserResultOK found username and password
// 	VerifyUserResultOK VerifyUserResult = iota
// 	// VerifyUserResultPasswordNotMatch password does not match
// 	VerifyUserResultPasswordNotMatch
// 	// VerifyUserResultEmailNotFound email is not found
// 	VerifyUserResultEmailNotFound
// 	// VerifyUserResultAccountNotVerified is when account is not verified yet
// 	VerifyUserResultAccountNotVerified
// 	// VerifyUserResultAccountNotActive is when account is not currently active
// 	VerifyUserResultAccountNotActive
// 	// VerifyUserResultOtherError other error
// 	VerifyUserResultOtherError
// )

// GetVerifiedAuthUser authenticates the user
// userModel is from JSON in the HTTP body
func GetVerifiedAuthUser(db *gorm.DB, userModel models.IModel) (models.IModel, error) {
	// TODO: maybe email is not the login, make it more flexible?
	// field name needs to be more flexible
	email := reflect.ValueOf(userModel).Elem().FieldByName(("Email")).Interface().(string)
	password := reflect.ValueOf(userModel).Elem().FieldByName(("Password")).Interface().(string)
	// status := reflect.ValueOf(userModel).Elem().FieldByName(("Status")).Interface().(models.UserStatus)
	// code := reflect.ValueOf(userModel).Elem().FieldByName(("VerificationCode")).Interface().(string)
	// expiredAt := reflect.ValueOf(userModel).Elem().FieldByName(("VerificationExpiredAt")).Interface().(*time.Time)

	userModel2 := reflect.New(models.UserTyp).Interface().(models.IModel)
	err := db.Where("email = ?", email).First(userModel2).Error
	if gorm.IsRecordNotFoundError(err) {
		return nil, ErrEmailNotFound // User doesn't exists with this email
		// return nil, VerifyUserResultEmailNotFound // User doesn't exists with this email
	} else if err != nil {
		// Some other unknown error
		return nil, err
	}

	status := reflect.ValueOf(userModel2).Elem().FieldByName(("Status")).Interface().(models.UserStatus)

	if status == models.UserStatusActive {
		passwordHash := reflect.ValueOf(userModel2).Elem().FieldByName("PasswordHash").Interface().(string)
		if !IsSamePassword(password, passwordHash) {
			// Password doesn't match
			return userModel2, ErrPasswordIncorrect
		}

		return userModel2, nil
	} else if status == models.UserStatusUnverified {
		return nil, ErrNotVerified
	} else if status == models.UserStatusInactive {
		return nil, ErrInactive
	}

	return nil, fmt.Errorf("error unknown")
}
