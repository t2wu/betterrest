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
	return q.Q(args...)
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
	Err    error
	order  *string // custom order to Gorm instead of "created_at DESC"
	limit  *int    // custom limit
	offset *int    // custom offset

	mainMB *ModelAndBuilder  // the builder on the main model (including the nested one)
	mbs    []ModelAndBuilder // the builder for non-nested models, each one is a separate non-nested model
}

func (q *Query) Q(args ...interface{}) IQuery {
	q.Reset() // always reset with Q()

	mb := ModelAndBuilder{}
	for _, arg := range args {
		b, ok := arg.(*PredicateRelationBuilder)
		if !ok {
			q.Err = fmt.Errorf("incorrect arguments for Q()")
			return q
		}

		// Leave model empty because it is not going to be filled until
		// Find() or First()
		binfo := BuilderInfo{
			builder:   b,
			processed: false,
		}
		mb.builderInfos = append(mb.builderInfos, binfo)
	}

	q.mainMB = &mb

	return q
}

func (q *Query) Order(order string) IQuery {
	if q.order != nil {
		log.Println("warning: query order already set")
	}

	if strings.Contains(order, ".") {
		q.Err = fmt.Errorf("dot notation in order")
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

// args can be multiple C(), each C() works on one-level of modelObj
// The args are to select the query of modelObj designated, it could work
// on nested level inside the modelObj
// assuming first is top-level, if given.
func (q *Query) InnerJoin(modelObj models.IModel, foreignObj models.IModel, args ...interface{}) IQuery {
	if q.Err != nil {
		return q
	}

	// Need to build the "On" clause
	// modelObj.foreignObjID = foreignObj.ID plus addition condition if any
	var ok bool
	var b *PredicateRelationBuilder

	typeName := models.GetModelTypeNameFromIModel(foreignObj)
	tbl := models.GetTableNameFromIModel(foreignObj)
	esc := &Escape{Value: fmt.Sprintf("\"%s\".id", tbl)}

	// Prepare for PredicateRelationBuilder which will be use to generate inner join statement
	// between the modelobj at hand and foreignObj (when joining the immediate table, the forignObj is
	// the modelObj within Find() and First())
	if len(args) > 0 {
		b, ok = args[0].(*PredicateRelationBuilder)
		if !ok {
			q.Err = fmt.Errorf("incorrect arguments for Q()")
			return q
		}

		// Check if the designator is about inner field or the outer-most level field
		rel, err := b.GetPredicateRelation()
		if err != nil {
			q.Err = err
			return q
		}
		field2Struct, _ := FindFieldNameToStructAndStructFieldNameIfAny(rel) // hacky
		if field2Struct == nil {                                             // outer-level field
			args[0] = b.And(typeName+"ID = ", esc)
		} else {
			// No other criteria, it is just a join by itself
			args = append(args, C(typeName+"ID = ", esc))
			// mb := ModelAndBuilder{ModelObj: modelObj, Builder: b}
			// q.mbs = append(q.mbs, mb)
		}
	} else { // No PredicateRelationBuilder given, build one from scratch
		args = append(args, C(typeName+"ID = ", esc))
		// mb := ModelAndBuilder{ModelObj: modelObj, Builder: b}
		// q.mbs = append(q.mbs, mb)
	}

	mb := ModelAndBuilder{}
	mb.modelObj = modelObj

	for i := 0; i < len(args); i++ {
		b, ok := args[i].(*PredicateRelationBuilder)
		if !ok {
			q.Err = fmt.Errorf("incorrect arguments for Q()")
			return q
		}
		binfo := BuilderInfo{
			builder:   b,
			processed: false,
		}
		mb.builderInfos = append(mb.builderInfos, binfo)
	}

	q.mbs = append(q.mbs, mb)

	return q
}

func (q *Query) First(modelObj models.IModel) IQuery {
	if q.Err != nil {
		return q
	}

	if q.mainMB != nil {
		q.mainMB.modelObj = modelObj
	}

	var err error
	q.db, err = q.buildQueryCore(q.db, modelObj)
	if err != nil {
		q.Err = err
		return q
	}

	q.db = q.buildQueryOrderOffSetAndLimit(q.db, modelObj)

	f := getQueryFunc(q.db, QueryTypeFirst)
	if f == nil {
		q.Err = fmt.Errorf("wrong QueryType")
		return q
	}

	if f != nil {
		q.Err = f(modelObj).Error
	}

	return q
}

func (q *Query) Find(modelObjs interface{}) IQuery {
	if q.Err != nil {
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

	if q.mainMB != nil {
		q.mainMB.modelObj = modelObj
	}

	var err error
	q.db, err = q.buildQueryCore(q.db, modelObj)
	if err != nil {
		q.Err = err
		return q
	}

	q.db = q.buildQueryOrderOffSetAndLimit(q.db, modelObj)

	f := getQueryFunc(q.db, QueryTypeFind)
	if f == nil {
		q.Err = fmt.Errorf("wrong QueryType")
		return q
	}

	if f != nil {
		q.Err = f(modelObjs).Error
	}

	return q
}

func (q *Query) buildQueryCore(db *gorm.DB, modelObj models.IModel) (*gorm.DB, error) {
	var err error
	db = buildPreload(db).Model(modelObj)

	// handles main modelObj
	q.mainMB.SortBuilderInfosByLevel() // now sorted, so our join statement can join in correct order

	// First-level queries that have no explicit join table
	for _, buildInfo := range q.mainMB.builderInfos {
		rel, err := buildInfo.builder.GetPredicateRelation()
		if err != nil {
			return db, err
		}

		if !DesignatorContainsDot(rel) { // where clause
			s, vals, err := rel.BuildQueryStringAndValues(q.mainMB.modelObj)
			if err != nil {
				return db, err
			}
			db = db.Where(s, vals...)
		}
	}

	db, err = q.buildQueryCoreInnerJoin(db, q.mainMB)
	if err != nil {
		return db, err
	}

	// Other non-nested tables
	// where we need table joins for sure and no where clause
	// But join statements foreign keys ha salready been made
	for _, mb := range q.mbs { // Now we work on mb.modelObj
		mb.SortBuilderInfosByLevel()

		for _, buildInfo := range mb.builderInfos { // each of this is on one-level (outer or nested)
			rel, err := buildInfo.builder.GetPredicateRelation()
			if err != nil {
				return db, err
			}

			if !DesignatorContainsDot(rel) {
				// first level, but since this is the other non-nested table
				// we use a join, and the foriegn key join is already set up
				// when we call query.Join
				s, vals, err := rel.BuildQueryStringAndValues(mb.modelObj)
				if err != nil {
					return db, err
				}

				tblName := models.GetTableNameFromIModel(mb.modelObj)
				db = db.Joins(fmt.Sprintf("INNER JOIN \"%s\" ON %s", tblName, s), vals...)
			}
		}

		db, err = q.buildQueryCoreInnerJoin(db, &mb)
		if err != nil {
			return db, err
		}
	}

	return db, nil
}

func (q *Query) buildQueryCoreInnerJoin(db *gorm.DB, mb *ModelAndBuilder) (*gorm.DB, error) {
	// There may not be any builder for the level of join
	// for example, when querying for 3rd level field, 2nd level also
	// needs to join with the first level
	designators, err := mb.GetAllPotentialJoinStructDesignators()
	if err != nil {
		return db, err
	}

	for _, designator := range designators { // this only loops tables which has joins
		found := false
		for _, buildInfo := range mb.builderInfos {
			rel, err := buildInfo.builder.GetPredicateRelation()
			if err != nil {
				return db, err
			}

			designatedField := rel.GetDesignatedField(mb.modelObj)
			if designator == designatedField { // OK, with this level we have search criteria to go along with it
				found = true
				s, vals, err := rel.BuildQueryStringAndValues(mb.modelObj)
				if err != nil {
					return db, err
				}

				// If it's one-level nested, we can join, but
				innerModel, err := rel.GetDesignatedModel(mb.modelObj)
				if err != nil {
					return db, err
				}
				tblName := models.GetTableNameFromIModel(innerModel)
				// get the outer table name
				outerTableName, err := GetOuterTableName(mb.modelObj, designatedField)
				if err != nil {
					return db, err
				}

				db = db.Joins(fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".%s_id = \"%s\".id AND (%s)", tblName, tblName,
					outerTableName, outerTableName, s), vals...)
			}
		}
		if !found { // no search critiria, just pure join statement
			toks := strings.Split(designator, ".") // A.B.C then we're concerened about joinnig B & C, A has been done
			// field := toks[len(toks)-1]

			upperTableName := ""
			if len(toks) == 1 {
				upperTableName = models.GetTableNameFromIModel(mb.modelObj)
			} else {
				designatorForUpperModel := strings.Join(toks[:len(toks)-1], ".")
				upperTableName, err = GetModelTableNameInModelIfValid(mb.modelObj, designatorForUpperModel)
				if err != nil {
					return db, err
				}
			}

			currTableName, err := GetModelTableNameInModelIfValid(mb.modelObj, designator)
			if err != nil {
				return db, err
			}

			db = db.Joins(fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".%s_id = \"%s\".id",
				currTableName, currTableName,
				upperTableName, upperTableName))
		}
	}

	// There are still first-level queries that have no explicit join table
	for _, buildInfo := range mb.builderInfos {
		rel, err := buildInfo.builder.GetPredicateRelation()
		if err != nil {
			return db, err
		}

		if !DesignatorContainsDot(rel) { // where clause
			s, vals, err := rel.BuildQueryStringAndValues(mb.modelObj)
			if err != nil {
				return db, err
			}
			db = db.Model(mb.modelObj).Where(s, vals...)
		}
	}

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
			q.Err = err
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
	q.Reset()

	q.Err = q.db.Create(modelObj).Error
	return q
}

func (q *Query) Delete(modelObj models.IModel) IQuery {
	if q.Err != nil {
		return q
	}

	if q.mainMB != nil {
		q.mainMB.modelObj = modelObj
	}

	// Won't work, builtqueryCore has "ORDER BY Clause"
	var err error
	q.db = q.db.Unscoped()
	q.db, err = q.buildQueryCore(q.db, modelObj)
	if err != nil {
		q.Err = err
		return q
	}

	q.Err = q.db.Delete(modelObj).Error

	return q
}

func (q *Query) Save(modelObj models.IModel) IQuery {
	if q.Err != nil {
		return q
	}

	q.Err = q.db.Save(modelObj).Error
	return q
}

// Update only allow one level of builder
func (q *Query) Update(modelObj models.IModel, p *PredicateRelationBuilder) IQuery {
	if q.Err != nil {
		return q
	}

	if q.mainMB != nil {
		q.mainMB.modelObj = modelObj
	}

	// Won't work, builtqueryCore has "ORDER BY Clause"
	var err error
	q.db, err = q.buildQueryCore(q.db, modelObj)
	if err != nil {
		q.Err = err
		return q
	}

	updateMap := make(map[string]interface{})
	rel, err := p.GetPredicateRelation()
	if err != nil {
		q.Err = err
		return q
	}

	field2Struct, _ := FindFieldNameToStructAndStructFieldNameIfAny(rel) // hacky
	if field2Struct != nil {
		q.Err = fmt.Errorf("dot notation in update")
		return q
	}

	qstr, values, err := rel.BuildQueryStringAndValues(modelObj)
	if err != nil {
		q.Err = err
		return q
	}

	toks := strings.Split(qstr, " = ?")

	for i, tok := range toks[:len(toks)-1] { // last tok is anempty str
		s := strings.Split(tok, ".")[1] // strip away the table name
		updateMap[s] = values[i]
	}

	q.Err = q.db.Update(updateMap).Error

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
	q.Err = nil
	q.order = nil
	q.limit = nil
	q.offset = nil

	q.mbs = make([]ModelAndBuilder, 0)
	return q
}

func (q *Query) Error() error {
	return q.Err
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

func DesignatorContainsDot(rel *PredicateRelation) bool {
	_, structFieldName := FindFieldNameToStructAndStructFieldNameIfAny(rel)
	return structFieldName != nil
}

func GetOuterTableName(modelObj models.IModel, fieldNameDesignator string) (string, error) {
	outerTableName := ""
	if strings.Contains(fieldNameDesignator, ".") {
		toks := strings.Split(fieldNameDesignator, ".")
		outerFieldNameToStruct := strings.Join(toks[:len(toks)-1], ".")
		typ2, err := GetModelFieldTypeInModelIfValid(modelObj, outerFieldNameToStruct)
		if err != nil {
			return "", err
		}
		outerTableName = models.GetTableNameFromType(typ2)
	} else {
		outerTableName = models.GetTableNameFromIModel(modelObj)
	}
	return outerTableName, nil
}
