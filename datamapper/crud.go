package datamapper

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	"betterrest/models"
	"betterrest/typeregistry"

	"github.com/jinzhu/gorm"
)

//------------------------

/*
CrudDataMapper is an interface supports the following
Single instance:
Create one
Read one
Update one
Patch one
Delete one

It maps JSON to Gorm objects and vice versa
*/
// type CrudDataMapper interface {
// 	Create(interface{}) (interface{}, error)
// 	Read(interface{}) (interface{}, error)
// 	Update(interface{}) (interface{}, error)

// 	!! Need to make sure this has non-empty ID, otherwise gorm clears everything in the database!!
// 	Delete(interface{}) (interface{}, error)
// }
// ------------------------------------

/*
If Mysql, the error from Gorm is like this: You can print the number
Error() is like this: "Error 1054: Unknown column 'class_id' in 'field list'""
type MySQLError struct {
    Number  uint16
    Message string
}
*/

var once sync.Once
var crud *BasicMapper

// BasicMapper is a basic CRUD manager
type BasicMapper struct {
}

// SharedBasicMapper creats a singleton of Crud object
func SharedBasicMapper() *BasicMapper {
	once.Do(func() {
		crud = &BasicMapper{}
	})

	return crud
}

// CreateOne creates an instance of this model based on json and store it in db
func (mapper *BasicMapper) CreateOne(db *gorm.DB, oid uint, typeString string, modelObj models.IModel) (models.IModel, error) {

	// Get owner id's admin ownership id
	// u := models.User{}
	// u.ID = oid
	// // db.Model(&u).Association("Ownerships")
	g := models.Ownership{}
	// var result gorm.Result

	firstJoin := "INNER JOIN `user_ownerships` ON `ownership`.id = `user_ownerships`.ownership_id AND user_id=? AND role=?"
	err := db.Table("ownership").Joins(firstJoin, oid, models.Admin).Find(&g).Error
	if err != nil {
		return nil, err
	}

	// rows, err := db.Raw("select * from `ownership` inner join user_ownerships on `ownership`.id = `user_ownerships`.ownership_id where user_id=? and role=?;",
	// 	oid, models.Admin).Rows()
	// defer rows.Close()
	// if err != nil {
	// 	return nil, err
	// }

	// c := 0
	// for rows.Next() {
	// 	c = c + 1
	// 	log.Println("BEFORE SCAN:", g)
	// 	db.ScanRows(rows, &g)
	// 	log.Println("BEFORE SCAN:", g)
	// }
	// if c != 1 {
	// 	// something is wrong.
	// 	err = errors.New("Cannot find exactly one resource owner for this resource")
	// 	log.Println(err)
	// 	return nil, err
	// }

	// log.Println("ownership found is", g)

	modelObj.AppendOwnership(g)

	// Oh this check if primary key is blank...
	if db.NewRecord(modelObj) { // FIXME: new record is not what I think. it will still return
		if dbc := db.Create(modelObj); dbc.Error != nil { // FIXME: Create doesn't return error, why?
			// create failed: UNIQUE constraint failed: user.email
			// It looks like this error may be dependent on the type of database we use
			log.Println("create failed:", dbc.Error)
			log.Println("error:", reflect.TypeOf(dbc.Error))
			return nil, dbc.Error
		}
	} else {
		return nil, errors.New("record exists")
	}

	// For table with trigger which update before insert, we need to load it again
	if err = db.First(modelObj).Error; err != nil {
		// That's weird. we just inserted it.
		return nil, err
	}

	return modelObj, nil
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *BasicMapper) GetOneWithID(db *gorm.DB, oid uint, typeString string, id uint64) (models.IModel, error) {
	modelObj := typeregistry.NewRegistry[typeString]()

	// err := db.Raw(qstring, id, models.Admin, oid).Scan(modelObj).Error

	db = db.Set("gorm:auto_preload", true)

	rtable := typeString[0 : len(typeString)-1] // resource table e.g. class
	firstJoin := fmt.Sprintf("INNER JOIN `%s_ownerships` ON `%s`.id = `%s_ownerships`.%s_id AND `%s`.id = ? ",
		rtable, rtable, rtable, rtable, rtable)
	secondJoin := fmt.Sprintf("INNER JOIN `ownership` ON `ownership`.id = `%s_ownerships`.ownership_id AND `ownership`.role = ? ",
		rtable)
	thirdJoin := "INNER JOIN `user_ownerships` ON `user_ownerships`.ownership_id = `ownership`.id "
	fourthJoin := "INNER JOIN user ON `user_ownerships`.user_id = user.id AND user.id = ?"
	err := db.Table(rtable).Joins(firstJoin, id).Joins(secondJoin, models.Admin).Joins(thirdJoin).Joins(fourthJoin, oid).Find(modelObj).Error

	return modelObj, err
}

// GetOneByField select by field
// func (c *Crud) GetOneByField(typeString string, fieldName string, fieldValue string) ([]byte, error) {
// 	modelPtr := typeregistry.NewRegistry[typeString]()

// 	// This is a value type
// 	if dbc := db.Where(fieldName+" = ?", fieldValue).First(modelPtr); dbc.Error != nil {
// 		return nil, dbc.Error
// 	}

// 	return tools.ToJSON(typeString, modelPtr, models.Admin)
// }

// GetAll obtains a slice of models.DomainModel
// options can be string "offset" and "limit", both of type int
// This is very Javascript-esque. I would have liked Python's optional parameter more.
// Alas, no such feature in Go. https://stackoverflow.com/questions/2032149/optional-parameters-in-go
// How does Gorm do the following? Might want to check out its source code.
// Cancel offset condition with -1
//  db.Offset(10).Find(&users1).Offset(-1).Find(&users2)
func (mapper *BasicMapper) GetAll(db *gorm.DB, oid uint, typeString string, options map[string]int) ([]models.IModel, error) {
	offset, limit := 0, 0
	if _, ok := options["offset"]; ok {
		offset = options["offset"]
	}
	if _, ok := options["limit"]; ok {
		limit = options["limit"]
	}

	cstart, cstop := 0, 0
	if _, ok := options["cstart"]; ok {
		cstart = options["cstart"]
	}
	if _, ok := options["cstop"]; ok {
		cstop = options["cstop"]
	}

	var f func(interface{}, ...interface{}) *gorm.DB

	db = db.Set("gorm:auto_preload", true)

	rtable := typeString[0 : len(typeString)-1] // resource table e.g. class
	firstJoin := fmt.Sprintf("INNER JOIN `%s_ownerships` ON `%s`.id = `%s_ownerships`.%s_id ",
		rtable, rtable, rtable, rtable)
	secondJoin := fmt.Sprintf("INNER JOIN `ownership` ON `ownership`.id = `%s_ownerships`.ownership_id AND `ownership`.role = ? ",
		rtable)
	thirdJoin := "INNER JOIN `user_ownerships` ON `user_ownerships`.ownership_id = `ownership`.id "
	fourthJoin := "INNER JOIN user ON `user_ownerships`.user_id = user.id AND user.id = ?"
	db = db.Table(rtable).Joins(firstJoin).Joins(secondJoin, models.Admin).Joins(thirdJoin).Joins(fourthJoin, oid)

	if cstart != 0 && cstop != 0 {

		db = db.Where(rtable+".created_at BETWEEN ? AND ?", time.Unix(int64(cstart), 0), time.Unix(int64(cstop), 0))
	}

	if limit != 0 {
		f = db.Offset(offset).Limit(limit).Find
	} else {
		f = db.Find
	}

	return typeregistry.NewSliceFromDBRegistry[typeString](f) // error from db is returned from here
}

// func CreateMany(modelType Type, json []byte) v DomainModel {
// 	// huh, how do I unmarshal an array of stuffs
// https://stackoverflow.com/questions/11066946/partly-json-unmarshal-into-a-map-in-go
// }

// UpdateOneWithID updates model based on this json
func (mapper *BasicMapper) UpdateOneWithID(db *gorm.DB, oid uint, typeString string, modelObj models.IModel, id uint64) (models.IModel, error) {
	if id == 0 {
		return nil, errors.New("Cannot update when ID is equal to 0")
	}

	// TODO: Is there a more efficient way?
	_, err := mapper.GetOneWithID(db, oid, typeString, id)
	if err != nil { // Error is "record not found" when not found
		log.Println("Error:", err)
		return nil, err
	}

	a := reflect.ValueOf(modelObj).Elem()
	if reflect.Indirect(a).FieldByName("ID").Interface().(uint) == 0 {
		return nil, errors.New("Cannot update when ID is equal to 0")
	} else if reflect.Indirect(a).FieldByName("ID").Interface().(uint) != uint(id) {
		return nil, errors.New("Cannot update when IDs are different")
	}

	if err := db.Save(modelObj).Error; err != nil { // save updates all fields (FIXME: need to check for required)
		log.Println("Error updating:", err)
		return nil, err
	}

	// This so we have the preloading.
	modelObj2, err := mapper.GetOneWithID(db, oid, typeString, id)
	if err != nil { // Error is "record not found" when not found
		log.Println("Error:", err)
		return nil, err
	}

	return modelObj2, nil
}

// UpdateMany updates multiple models
func (mapper *BasicMapper) UpdateMany(db *gorm.DB, oid uint, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	ms := make([]models.IModel, 0, 0)
	var err error
	var m models.IModel

	for _, modelObj := range modelObjs {
		a := reflect.ValueOf(modelObj).Elem()
		id := reflect.Indirect(a).FieldByName("ID").Interface().(uint)
		if m, err = mapper.UpdateOneWithID(db, oid, typeString, modelObj, uint64(id)); err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}

	return ms, nil
}

// DeleteOneWithID delete the model
func (mapper *BasicMapper) DeleteOneWithID(db *gorm.DB, oid uint, typeString string, id uint64) (models.IModel, error) {
	if id == 0 {
		return nil, errors.New("Cannot delete when ID is equal to 0")
	}

	// modelObj := typeregistry.NewRegistry[typeString]()

	// check out
	// https://stackoverflow.com/questions/52124137/cant-set-field-of-a-struct-that-is-typed-as-an-interface
	/*
		a := reflect.ValueOf(modelObj).Elem()
		b := reflect.Indirect(a).FieldByName("ID")
		b.Set(reflect.ValueOf(uint(id)))
	*/

	// Right now it will NOT allow you to delete another user's stuff,
	// But there would be no warning. (unless we do an extra query)

	modelObj, err := mapper.GetOneWithID(db, oid, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return nil, err
	}

	// Unscoped() REAL delete!
	// Otherwise my constraint won't work...
	// Soft delete will take more work, have to verify myself manually
	return modelObj, db.Delete(modelObj).Error
	// return modelObj, db.Unscoped().Delete(modelObj).Error
}

// DeleteMany deletes multiple models
func (mapper *BasicMapper) DeleteMany(db *gorm.DB, oid uint, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	ids := make([]uint, len(modelObjs), len(modelObjs))
	for i, v := range modelObjs {
		fmt.Println("V is?", v)
		a := reflect.ValueOf(v).Elem()
		id := reflect.Indirect(a).FieldByName("ID").Interface().(uint)
		if id == 0 {
			return nil, errors.New("Cannot delete when ID is equal to 0")
		}

		ids[i] = id
	}

	ms := make([]models.IModel, 0, 0)
	var err error
	var m models.IModel

	for _, id := range ids {
		if m, err = mapper.DeleteOneWithID(db, oid, typeString, uint64(id)); err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}

	return ms, nil
}
