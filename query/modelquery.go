package query

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/models"
)

// -----------------------------
type QueryType int

const (
	QueryTypeFirst QueryType = iota
	QueryTypeFind  QueryType = iota
)

func Q(db *gorm.DB) *Query {
	return &Query{db: db}
}

// Q is the query struct
// Q(db).By("Name IN", []strings{name1, name2}, "Age >=", 18).Find(&model).Error
// This is a wrapper over Gorm's.
// Query by field name, and prevent SQL injection by making sure that fields are part of the
// model
type Query struct {
	db    *gorm.DB // Gorm db object can be a transaction
	args  []interface{}
	Error error
}

func (q *Query) By(args ...interface{}) *Query {
	q.args = args
	return q
}

func (q *Query) Find(modelObjs interface{}) *Query {
	typ := reflect.TypeOf(modelObjs).Elem().Elem()
	imodel := reflect.New(typ).Interface().(models.IModel)

	query, ok := q.args[0].(string)
	if !ok {
		q.Error = fmt.Errorf("incorrect query")
		return q
	}

	query, err := normalizeQuery(imodel, query)
	if err != nil {
		q.Error = err
		return q
	}

	builder := C(query, q.args[1])
	for i := 2; i < len(q.args); i += 2 {
		query, ok := q.args[i].(string)
		if !ok {
			q.Error = fmt.Errorf("incorrect query")
			return q
		}
		query, err := normalizeQuery(imodel, query)
		if err != nil {
			q.Error = err
			return q
		}

		builder.And(query, q.args[i+1])
	}

	if builder.Error != nil {
		q.Error = builder.Error
		return q
	}

	rel, err := builder.GetPredicateRelation()
	if err != nil {
		q.Error = err
		return q
	}

	q.Error = FindByPredicateRelation(q.db, modelObjs, rel)
	return q
}

func (q *Query) First(modelObj models.IModel) *Query {
	query, ok := q.args[0].(string)
	if !ok {
		q.Error = fmt.Errorf("incorrect query")
		return q
	}

	query, err := normalizeQuery(modelObj, query)
	if err != nil {
		q.Error = err
		return q
	}

	builder := C(query, q.args[1])
	for i := 2; i < len(q.args); i += 2 {
		query, ok := q.args[i].(string)
		if !ok {
			q.Error = fmt.Errorf("incorrect query")
			return q
		}
		query, err := normalizeQuery(modelObj, query)
		if err != nil {
			q.Error = err
			return q
		}

		builder.And(query, q.args[i+1])
	}

	if builder.Error != nil {
		q.Error = builder.Error
		return q
	}

	rel, err := builder.GetPredicateRelation()
	if err != nil {
		q.Error = err
		return q
	}

	q.Error = FirstByPredicateRelation(q.db, modelObj, rel)
	return q
}

// FirstByPredicateRelation finds the model from the querying the predicate relation
// In PredicateRelation, it is expected that queries are already in column name
func FirstByPredicateRelation(tx *gorm.DB, modelObj models.IModel, pr *PredicateRelation) error {
	err := byPredicateRelationQueryType(tx, modelObj, pr, QueryTypeFirst)
	if err != nil {
		toks := strings.SplitAfterN(err.Error(), "pg:", 2) // reformat the postgres error message
		if len(toks) == 2 {
			err = fmt.Errorf(toks[1])
		}
	}
	return err
}

// FindByPredicateRelation finds the model(s) from the querying the predicate relation
// In PredicateRelation, it is expected that queries are already in column name
// modelObjs is an array of IModel
func FindByPredicateRelation(tx *gorm.DB, modelObjs interface{}, pr *PredicateRelation) error {
	err := byPredicateRelationQueryType(tx, modelObjs, pr, QueryTypeFind)
	if err != nil {
		toks := strings.SplitAfterN(err.Error(), "pg:", 2)
		return fmt.Errorf(toks[1])
	}
	return nil
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

// normalize query to column name query
func normalizeQuery(obj models.IModel, query string) (string, error) {
	toks := strings.Split(strings.TrimSpace(query), " ")
	col, err := FieldNameToColumn(obj, toks[0])
	if err != nil {
		return "", err
	}
	return col + " " + toks[1], nil
}
