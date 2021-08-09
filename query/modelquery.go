package query

import (
	"fmt"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// -----------------------------

type QueryType int

const (
	QueryTypeFirst QueryType = iota
	QueryTypeFind  QueryType = iota
)

// -----------------------------

func ByID(tx *gorm.DB, modelObj models.IModel, id *datatypes.UUID) error {
	return FirstByFieldQueries(tx, modelObj, "id =", id)
}

// FirstByFieldQueries query the model by query string and values
// args are "name =", "Christy", "age >=", 30 for example
func FirstByFieldQueries(tx *gorm.DB, modelObj models.IModel, args ...interface{}) error {
	query, ok := args[0].(string)
	if !ok {
		return fmt.Errorf("incorrect query")
	}
	builder := C(query, args[1])
	for i := 2; i < len(args); i += 2 {
		query, ok := args[i].(string)
		if !ok {
			return fmt.Errorf("incorrect query")
		}
		builder.And(query, args[i+1])
	}

	if builder.Error != nil {
		return builder.Error
	}

	return FirstByPredicateRelation(tx, modelObj, builder.Rel)
}

// FirstByFieldQuery allows query for a table field
// query is like "name =" or "age >=", value is the correspondonding value to query for
// func FirstByFieldQuery(tx *gorm.DB, modelObj models.IModel, query string, value interface{}) error {
// 	rel, err := C(query, value).GetPredicateRelation()
// 	if err != nil {
// 		return err
// 	}
// 	return FirstByPredicateRelation(tx, modelObj, rel)
// }

func FirstByPredicateRelation(tx *gorm.DB, modelObj models.IModel, pr *PredicateRelation) error {
	return byPredicateRelationQueryType(tx, modelObj, pr, QueryTypeFirst)
}

func FindByPredicateRelation(tx *gorm.DB, modelObjs []models.IModel, pr *PredicateRelation) error {
	return byPredicateRelationQueryType(tx, modelObjs, pr, QueryTypeFind)
}

func byPredicateRelationQueryType(tx *gorm.DB, resultOrResults interface{}, pr *PredicateRelation, typ QueryType) error {
	tx2 := buildPreload(tx)

	qs, v := pr.BuildQueryStringAndValues()
	tx2 = tx2.Where(qs, v...).Order("created_at DESC")
	f := getQueryFunc(tx2, typ)
	if f == nil {
		return fmt.Errorf("wrong QueryType")
	}

	if err := f(resultOrResults).Error; err != nil {
		return err
	}

	return nil
}

// ==============
func buildPreload(tx *gorm.DB) *gorm.DB {
	return tx.Set("gorm:auto_preload", true)
}

// func (s *DB) Find(out interface{}, where ...interface{}) *DB {
func getQueryFunc(tx *gorm.DB, f QueryType) func(interface{}, ...interface{}) *gorm.DB {
	switch f {
	case QueryTypeFind:
		return tx.Find
	case QueryTypeFirst:
		return tx.First
	}

	return nil
}

// // verifyFieldOnModel verifies that field part of the model
// func verifyFieldOnModel(model *models.IModel, field string) bool {

// }
