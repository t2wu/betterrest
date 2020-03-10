package typeregistry

import (
	"betterrest/libs/security"
	"betterrest/models"
	"encoding/json"

	"betterrest/db"

	"github.com/jinzhu/gorm"
)

// init iniitializes db
func init() {

	// TODO: Set null on delete
	// But we have soft delete...
	// https://www.techonthenet.com/sql_server/foreign_keys/foreign_delete_null.php
	// https://stackoverflow.com/questions/506432/cascading-soft-delete

	db.Shared().AutoMigrate(&models.Client{})
	db.Shared().AutoMigrate(&models.User{})
	// ON DELTE RESTRICT ON UPDATE RESTRICT
	db.Shared().AutoMigrate(&models.Ownership{})
	db.Shared().AutoMigrate(&models.Class{})
}

// NewRegistry is a map of functions to create new models
var NewRegistry = map[string]func() models.IModel{
	"users": func() models.IModel {
		return new(models.User)
	},
	"classes": func() models.IModel {
		return new(models.Class)
	},
}

// NewFromJSONRegistry is a map of functions to create new models from JSON
var NewFromJSONRegistry = map[string]func([]byte) (models.IModel, error){
	"users": func(jsn []byte) (models.IModel, error) {
		m := new(models.User)
		err := json.Unmarshal(jsn, m)

		if err != nil {
			return nil, err
		}

		// Need to encrypt the password
		if m.Password != "" {
			hash, err := security.HashAndSalt(m.Password)
			if err != nil {
				return nil, err
			}

			// m.Password = "" // Leave the password intact, may actually be a login
			m.PasswordHash = hash
		}

		return m, err
	},
	"classes": func(jsn []byte) (models.IModel, error) {
		m := new(models.Class)
		err := json.Unmarshal(jsn, m)
		return m, err
	},
}

// NewSliceFromJSONRegistry makes slice of model object from json string
var NewSliceFromJSONRegistry = map[string]func([]byte) ([]models.IModel, error){
	"classes": func(jsn []byte) ([]models.IModel, error) {
		obj := make(map[string][]models.Class)
		obj["content"] = make([]models.Class, 0, 0)

		err := json.Unmarshal(jsn, &obj)
		if err != nil {
			return nil, err
		}

		var ret = make([]models.IModel, len(obj["content"]), len(obj["content"]))
		for i := 0; i < len(obj["content"]); i++ {
			ret[i] = &obj["content"][i]
		}

		return ret, err
	},
}

// NewSliceFromDBRegistry get slice of data from db
// Reflection stuffs
// https://stackoverflow.com/questions/18604345/how-to-make-array-with-given-namestring-in-golang-with-reflect
// https://stackoverflow.com/questions/25384640/why-golang-reflect-makeslice-returns-un-addressable-value
// https://stackoverflow.com/questions/44319906/why-golang-struct-array-cannot-be-assigned-to-an-interface-array
var NewSliceFromDBRegistry = map[string]func(f func(interface{}, ...interface{}) *gorm.DB) ([]models.IModel, error){
	"classes": func(f func(interface{}, ...interface{}) *gorm.DB) ([]models.IModel, error) {
		var modelObjs []models.Class
		if err := f(&modelObjs).Error; err != nil {
			return nil, err
		}

		y := make([]models.IModel, len(modelObjs))
		for i := 0; i < len(modelObjs); i++ {
			y[i] = &modelObjs[i]
		}

		return y, nil
	},
}
