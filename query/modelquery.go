package query

import (
	"fmt"
	"log"
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

// It would be Q(db, C(...), C(...)...).First() or Q(db).First() with empty PredicateRelationBuilder
// Use multiple C() when working on inner fields (one C() per struct field)
func Q(db *gorm.DB, args ...interface{}) IQuery {
	q := &Query{db: db, dbori: db}

	for _, arg := range args {
		b, ok := arg.(*PredicateRelationBuilder)
		if !ok {
			q.err = fmt.Errorf("incorrect arguments for Q()")
			return q
		}

		// Leave model empty because it is not going to be filled until
		// Find() or First()
		mb := ModelAndBuilder{Builder: b}
		q.mbs = append(q.mbs, mb)
	}

	return q
}

// Instead of Q() directly, we can use DB().Q()
// This is so it's easier to stubb out when testing
func DB(db *gorm.DB) IQuery {
	return &Query{db: db, dbori: db}
}

// Q is the query struct
// Q(db).By("Name IN", []strings{name1, name2}, "Age >=", 18).Find(&model).Error
// This is a wrapper over Gorm's.
// Query by field name, and prevent SQL injection by making sure that fields are part of the
// model
type Query struct {
	db    *gorm.DB // Gorm db object can be a transaction
	dbori *gorm.DB // So we can reset it for another query if needed.
	// args  []interface{}
	err    error
	order  *string // custom order to Gorm instead of "created_at DESC"
	limit  *int    // custom limit
	offset *int    // custom offset

	mbs []ModelAndBuilder
}

type ModelAndBuilder struct {
	ModelObj models.IModel // THe model this predicate relation applies to
	Builder  *PredicateRelationBuilder
}

func (q *Query) Q(args ...interface{}) IQuery {
	q.Reset() // always reset with Q()

	for _, arg := range args {
		b, ok := arg.(*PredicateRelationBuilder)
		if !ok {
			q.err = fmt.Errorf("incorrect arguments for Q()")
			return q
		}

		// Leave model empty because it is not going to be filled until
		// Find() or First()
		mb := ModelAndBuilder{Builder: b}
		q.mbs = append(q.mbs, mb)
	}

	return q
}

func (q *Query) Order(order string) IQuery {
	if q.order != nil {
		log.Println("warning: query order already set")
	}

	if strings.Contains(order, ".") {
		q.err = fmt.Errorf("dot notation in order")
		return q
	}

	q.order = &order
	return q
}

func (q *Query) Limit(limit int) IQuery {
	if q.limit != nil {
		log.Println("warning: query limit already set")
	}
	q.limit = &limit
	return q
}

func (q *Query) Offset(offset int) IQuery {
	if q.offset != nil {
		log.Println("warning: query offset already set")
	}
	q.offset = &offset
	return q
}

// args can be multiple C(), but each C() works on one-level of modelObj
// assuming first is top-level, if given
func (q *Query) InnerJoin(modelObj models.IModel, foreignObj models.IModel, args ...interface{}) IQuery {
	if q.err != nil {
		return q
	}

	// Need to build the "On" clause
	// modelObj.foreignObjID = foreignObj.ID plus addition condition if any
	var ok bool
	var b *PredicateRelationBuilder

	typeName := models.GetModelTypeNameFromIModel(foreignObj)
	tbl := models.GetTableNameFromIModel(foreignObj)
	esc := &Escape{Value: fmt.Sprintf("\"%s\".id", tbl)}

	if len(args) > 0 {
		b, ok = args[0].(*PredicateRelationBuilder)
		if !ok {
			q.err = fmt.Errorf("incorrect arguments for Q()")
			return q
		}

		b = b.And(typeName+"ID = ", esc)
	} else { // No PredicateRelationBuilder given, build one from scratch
		b = C(typeName+"ID = ", esc)
	}

	mb := ModelAndBuilder{ModelObj: modelObj, Builder: b}
	q.mbs = append(q.mbs, mb)

	for i := 1; i < len(args); i++ {
		b, ok := args[i].(*PredicateRelationBuilder)
		if !ok {
			q.err = fmt.Errorf("incorrect arguments for Q()")
			return q
		}
		mb := ModelAndBuilder{ModelObj: modelObj, Builder: b}
		q.mbs = append(q.mbs, mb)
	}

	return q
}

func (q *Query) First(modelObj models.IModel) IQuery {
	if q.err != nil {
		return q
	}
	var err error
	q.db, err = q.buildQueryCore(q.db, modelObj)
	if err != nil {
		q.err = err
		return q
	}

	q.db = q.buildQueryOrderOffSetAndLimit(q.db, modelObj)

	f := getQueryFunc(q.db, QueryTypeFirst)
	if f == nil {
		q.err = fmt.Errorf("wrong QueryType")
		return q
	}

	if f != nil {
		q.err = f(modelObj).Error
	}

	return q
}

func (q *Query) Find(modelObjs interface{}) IQuery {
	if q.err != nil {
		return q
	}

	typ := reflect.TypeOf(modelObjs)
loop:
	for {
		switch typ.Kind() {
		case reflect.Slice:
			typ = typ.Elem()
		case reflect.Ptr:
			typ = typ.Elem()
		default:
			break loop
		}
	}

	modelObj := reflect.New(typ).Interface().(models.IModel)

	var err error
	q.db, err = q.buildQueryCore(q.db, modelObj)
	if err != nil {
		q.err = err
		return q
	}

	q.db = q.buildQueryOrderOffSetAndLimit(q.db, modelObj)

	f := getQueryFunc(q.db, QueryTypeFind)
	if f == nil {
		q.err = fmt.Errorf("wrong QueryType")
		return q
	}

	if f != nil {
		q.err = f(modelObjs).Error
	}

	return q
}

func (q *Query) buildQueryCore(db *gorm.DB, modelObj models.IModel) (*gorm.DB, error) {
	db = buildPreload(db)
	modelTypeName := models.GetModelTypeNameFromIModel(modelObj)
	firstOneProcessed := false

	if len(q.mbs) > 0 {
		// If no dot notation, and no table designator, then it's modelObj
		// Else if dot notation, and no table designator, then it's a nested modelObj of this modelObj

		// But if the first one is nested table, that means there is no where clause
		// I need to get the name of the table directly.
		if q.mbs[0].ModelObj == nil {
			firstOneProcessed = true
			q.mbs[0].ModelObj = modelObj // It's definitely the main modelObj we want to search for

			// Now if there is a dot notation, it means we'd be using the join clause to find the nested
			// one, and that means we need the table name.
			// Any dot notation? Need to find just one Predicator
			rel, err := q.mbs[0].Builder.GetPredicateRelation()
			if err != nil {
				return db, err
			}
			field2Struct, _ := FindFieldNameToStructAndStructFieldNameIfAny(rel) // hacky
			if field2Struct != nil {                                             // has dot notation and so has an inner field
				nestedModel, err := GetInnerModelIfValid(modelObj, *field2Struct)
				if err != nil {
					return db, err
				}
				nestedTableName := models.GetTableNameFromIModel(nestedModel)
				mainTableName := models.GetTableNameFromIModel(modelObj)

				foreignKeyQueryStr := fmt.Sprintf("%s.%sID =", *field2Struct, modelTypeName)
				foreignKeyQueryValue := fmt.Sprintf("%s.ID", mainTableName)
				esc := &Escape{Value: foreignKeyQueryValue}

				rel, err := q.mbs[0].Builder.And(foreignKeyQueryStr, esc).GetPredicateRelation()
				if err != nil {
					return db, err
				}

				s, vals, err := rel.BuildQueryStringAndValues(modelObj)
				if err != nil {
					return db, err
				}

				db = db.Model(modelObj).Joins(fmt.Sprintf("INNER JOIN %s ON %s", nestedTableName, s), vals...)
			} else { // No dot notation, so it is the modelObj itself
				rel, err := q.mbs[0].Builder.GetPredicateRelation()
				if err != nil {
					return db, err
				}

				s, vals, err := rel.BuildQueryStringAndValues(q.mbs[0].ModelObj)

				if err != nil {
					return db, err
				}

				db = db.Model(modelObj).Where(s, vals...)
			}
		}
	}

	start := 0
	if firstOneProcessed {
		start = 1
	}

	for i := start; i < len(q.mbs); i++ {
		// modelObj := q.mbs[i].ModelObj
		rel, err := q.mbs[i].Builder.GetPredicateRelation()
		if err != nil {
			return db, err
		}

		s, vals, err := rel.BuildQueryStringAndValues(q.mbs[i].ModelObj)
		if err != nil {
			return db, err
		}

		tblName := models.GetTableNameFromIModel(q.mbs[i].ModelObj)
		db = db.Joins(fmt.Sprintf("INNER JOIN %s ON %s", tblName, s), vals...)
	}

	// order := ""
	// if q.order != nil {
	// 	order = *q.order
	// } else {
	// 	order = fmt.Sprintf("\"%s\".created_at DESC", models.GetTableNameFromIModel(modelObj))
	// }

	// db = db.Order(order)

	// if q.offset != nil {
	// 	db = db.Offset(*q.offset)
	// }

	// if q.limit != nil {
	// 	db = db.Limit(*q.limit)
	// }

	return db, nil
}

func (q *Query) buildQueryOrderOffSetAndLimit(db *gorm.DB, modelObj models.IModel) *gorm.DB {
	order := ""
	if q.order != nil {
		toks := strings.Split(*q.order, " ")
		fieldName := toks[0]
		rest := toks[1] // DESC or ASC
		col, err := FieldNameToColumn(modelObj, fieldName)
		if err != nil {
			q.err = err
		}

		tableName := models.GetTableNameFromIModel(modelObj)
		order = fmt.Sprintf("\"%s\".%s %s", tableName, col, rest)
	} else {
		order = fmt.Sprintf("\"%s\".created_at DESC", models.GetTableNameFromIModel(modelObj))
	}

	db = db.Order(order)

	if q.offset != nil {
		db = db.Offset(*q.offset)
	}

	if q.limit != nil {
		db = db.Limit(*q.limit)
	}
	return db
}

func (q *Query) Create(modelObj models.IModel) IQuery {
	if q.err != nil {
		return q
	}

	q.err = q.dbori.Create(modelObj).Error
	return q
}

func (q *Query) Delete(modelObj models.IModel) IQuery {
	if q.err != nil {
		return q
	}

	// Won't work, builtqueryCore has "ORDER BY Clause"
	var err error
	q.db = q.db.Unscoped()
	q.db, err = q.buildQueryCore(q.db, modelObj)
	if err != nil {
		q.err = err
		return q
	}

	// updateMap := make(map[string]interface{})
	// rel, err := p.GetPredicateRelation()
	// if err != nil {
	// 	q.err = err
	// 	return q
	// }

	// field2Struct, _ := FindFieldNameToStructAndStructFieldNameIfAny(rel) // hacky
	// if field2Struct != nil {
	// 	q.err = fmt.Errorf("dot notation in update")
	// 	return q
	// }

	// qstr, values, err := rel.BuildQueryStringAndValues(modelObj)
	// if err != nil {
	// 	q.err = err
	// 	return q
	// }

	// toks := strings.Split(qstr, " = ?")

	// for i, tok := range toks[:len(toks)-1] { // last tok is anempty str
	// 	s := strings.Split(tok, ".")[1] // strip away the table name
	// 	updateMap[s] = values[i]
	// }

	q.err = q.db.Delete(modelObj).Error

	return q

	// if q.err != nil {
	// 	return q
	// }

	// q.err = q.dbori.Delete(modelObj).Error
	// return q
}

func (q *Query) Save(modelObj models.IModel) IQuery {
	if q.err != nil {
		return q
	}

	q.err = q.db.Save(modelObj).Error
	return q
}

// Update only allow one level of builder
func (q *Query) Update(modelObj models.IModel, p *PredicateRelationBuilder) IQuery {
	if q.err != nil {
		return q
	}

	// Won't work, builtqueryCore has "ORDER BY Clause"
	var err error
	q.db, err = q.buildQueryCore(q.db, modelObj)
	if err != nil {
		q.err = err
		return q
	}

	updateMap := make(map[string]interface{})
	rel, err := p.GetPredicateRelation()
	if err != nil {
		q.err = err
		return q
	}

	field2Struct, _ := FindFieldNameToStructAndStructFieldNameIfAny(rel) // hacky
	if field2Struct != nil {
		q.err = fmt.Errorf("dot notation in update")
		return q
	}

	qstr, values, err := rel.BuildQueryStringAndValues(modelObj)
	if err != nil {
		q.err = err
		return q
	}

	toks := strings.Split(qstr, " = ?")

	for i, tok := range toks[:len(toks)-1] { // last tok is anempty str
		s := strings.Split(tok, ".")[1] // strip away the table name
		updateMap[s] = values[i]
	}

	q.err = q.db.Update(updateMap).Error

	return q
}

func (q *Query) GetDB() *gorm.DB {
	return q.db
}

func (q *Query) GetDBOri() *gorm.DB {
	return q.dbori
}

func (q *Query) Reset() IQuery {
	q.db = q.dbori
	q.err = nil
	q.order = nil
	q.limit = nil
	q.offset = nil

	q.mbs = make([]ModelAndBuilder, 0)
	return q
}

func (q *Query) Error() error {
	return q.err
}

type TableAndArgs struct {
	TblName string // The table the predicate relation applies to, at this level (non-nested)
	Args    []interface{}
}

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

// hacky...
func FindFieldNameToStructAndStructFieldNameIfAny(rel *PredicateRelation) (*string, *string) {
	for _, pr := range rel.PredOrRels {
		if p, ok := pr.(*Predicate); ok {
			if strings.Contains(p.Field, ".") {
				toks := strings.Split(p.Field, ".")
				name := toks[len(toks)-2] // next to alst
				return &name, &toks[len(toks)-1]
			}
		}
		if rel2, ok := pr.(*PredicateRelation); ok {
			return FindFieldNameToStructAndStructFieldNameIfAny(rel2)
		}
	}
	return nil, nil
}
