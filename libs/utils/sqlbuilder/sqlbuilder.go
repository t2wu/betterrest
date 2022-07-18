package sqlbuilder

import (
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/libs/utils/letters"
	"github.com/t2wu/betterrest/registry"
	"github.com/t2wu/qry"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"
)

// Something like this.
// Search by dense_rank

// Predicate :-
type Predicate struct {
	PredicateLogic qry.PredicateCond
	FieldValue     string
}

// FilterCriteria is the criteria to query for first-level field
type FilterCriteria struct {
	FieldName     string        // Field name to match
	PredicatesArr [][]Predicate // greater than less than, etc., multiple for AND relationship
	// Is it? age > 30 OR age < 20 AND ?
	// actually if there is predicate that's not qry.PredicateCondEQ, you can't do qry.PredicateCondEQ
}

// TwoLevelFilterCriteria is the criteria to query for inner level field
type TwoLevelFilterCriteria struct { //看到I看不到 lower left bracket
	OuterTableName string
	InnerTableName string
	OuterFieldName string
	Filters        []FilterCriteria // Key: inner table name,
}

// AddWhereStmt adds where statement into db
func AddWhereStmt(db *gorm.DB, typeString string, tableName string, filter FilterCriteria) (*gorm.DB, error) {
	// I won't have both equal test AND >, <, <=, >= tests in these case
	modelObj := registry.NewFromTypeString(typeString)

	urlFieldValues := make([]string, 0)
	for _, predicates := range filter.PredicatesArr {
		for _, predicate := range predicates {
			urlFieldValues = append(urlFieldValues, predicate.FieldValue)
		}
	}

	transformedFieldValues, err := getTransformedValueFromValidField(modelObj,
		letters.CamelCaseToPascalCase(filter.FieldName), urlFieldValues)

	if err != nil {
		return db, err
	}

	filterdFieldValues, anyNull := filterNullValue(transformedFieldValues)

	// If there is any equality comparison other than equal
	// there shouldn't be any IN then
	hasEquality := false
	for _, predicates := range filter.PredicatesArr {
		for _, predicate := range predicates {
			if predicate.PredicateLogic == qry.PredicateCondEQ {
				hasEquality = true
			}
		}
	}

	var whereStmt string
	if hasEquality {
		// Gorm will actually use one WHERE clause with AND statements if Where is called repeatedly
		whereStmt = inOpStmt(tableName, strcase.SnakeCase(filter.FieldName),
			len(filterdFieldValues), anyNull)
		db = db.Where(whereStmt, filterdFieldValues...)
	} else {
		whereStmt = comparisonOpStmt(tableName, strcase.SnakeCase(filter.FieldName), filter.PredicatesArr)

		db = db.Where(whereStmt, filterdFieldValues...)
	}

	return db, nil
}

// AddNestedQueryJoinStmt adds a join statement into db
func AddNestedQueryJoinStmt(db *gorm.DB, typeString string, criteria TwoLevelFilterCriteria) (*gorm.DB, error) {
	// join inner table and outer table based on outer table id
	joinStmt := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".%s = \"%s\".id ",
		criteria.InnerTableName, criteria.InnerTableName, criteria.OuterTableName+"_id", criteria.OuterTableName)

	queryValues := make([]interface{}, 0)
	// var err error

	for _, filter := range criteria.Filters {
		innerFieldName := filter.FieldName

		fieldValues := make([]string, 0)
		for _, predicates := range filter.PredicatesArr {
			for _, predicate := range predicates {
				fieldValues = append(fieldValues, predicate.FieldValue)
			}
		}

		// fieldValues := filter.FieldValues

		// Get inner field type
		m := registry.NewFromTypeString(typeString) // THIS IS TO BE FIXED
		fieldType, err := datatype.GetModelFieldTypeElmIfValid(m, letters.CamelCaseToPascalCase(criteria.OuterFieldName))
		if err != nil {
			return nil, err
		}

		m2 := reflect.New(fieldType).Interface()
		fieldType2, err := datatype.GetModelFieldTypeElmIfValid(m2, letters.CamelCaseToPascalCase(innerFieldName))
		if err != nil {
			return nil, err
		}

		transformedValues, err := datatype.TransformFieldValues(fieldType2.String(), fieldValues)
		if err != nil {
			return nil, err
		}

		fiterdFieldValues, anyNull := filterNullValue(transformedValues)

		// If there is any equality comparison other than equal
		// there shouldn't be any IN then
		hasEquality := false
		for _, predicates := range filter.PredicatesArr {
			for _, predicate := range predicates {
				if predicate.PredicateLogic == qry.PredicateCondEQ {
					hasEquality = true
				}
			}
		}

		var inOrComparisonStmt string
		if hasEquality {
			// It's possible to have multiple values by using ?xx=yy&xx=zz
			// Get the inner table's type
			inOrComparisonStmt = inOpStmt(criteria.InnerTableName, strcase.SnakeCase(innerFieldName), len(fiterdFieldValues), anyNull)
		} else {
			inOrComparisonStmt = comparisonOpStmt(criteria.InnerTableName, strcase.SnakeCase(innerFieldName), filter.PredicatesArr)
		}
		joinStmt += "AND (" + inOrComparisonStmt + ")"

		queryValues = append(queryValues, fiterdFieldValues...)
	}

	db = db.Joins(joinStmt, queryValues...)

	return db, nil
}

func filterHasLateston(filters []FilterCriteria) bool {
	// if having latestn but no latestnon, old behavior
	for _, filter := range filters {
		if filter.FieldName == "latestnon" {
			return true
		}
	}
	return false
}

// filters is for latestnons
func AddLatestNCTEJoin(db *gorm.DB, typeString string, tableName string, latestn int, latestnons []string, filterslatestnons []FilterCriteria) (*gorm.DB, error) {
	partitionByArr := make([]string, 0)
	latestnonWhereArr := make([]string, 0)
	transformedValues := make([]interface{}, 0)
	m := registry.NewFromTypeString(typeString)

	for _, filter := range filterslatestnons {
		// If there is any equality comparison other than equal
		// there shouldn't be any IN then
		hasEquality := false
		for _, predicates := range filter.PredicatesArr {
			for _, predicate := range predicates {
				if predicate.PredicateLogic == qry.PredicateCondEQ {
					hasEquality = true
				}
			}
		}

		urlFieldValues := make([]string, 0)
		for _, predicates := range filter.PredicatesArr {
			for _, predicate := range predicates {
				urlFieldValues = append(urlFieldValues, predicate.FieldValue)
			}
		}

		transformedFieldValues, err := getTransformedValueFromValidField(m,
			letters.CamelCaseToPascalCase(filter.FieldName), urlFieldValues)

		if _, ok := err.(*datatype.FieldNotInModelError); ok {
			continue
		}
		if err != nil {
			return db, err
		}

		filterdFieldValues, anyNull := filterNullValue(transformedFieldValues)
		// If passed, the field is part of the data structure

		// fieldName := strcase.SnakeCase(filter.FieldName)

		actualFieldName, err := mdl.JSONKeysToFieldName(m, filter.FieldName)
		if err != nil {
			return db, err
		}
		fieldName, err := mdl.FieldNameToColumn(m, actualFieldName)
		if err != nil {
			return db, err
		}

		// One field is either >= or =, so we split by equality here
		if hasEquality {
			latestnonWhereArr = append(latestnonWhereArr, inOpStmt(tableName, fieldName, len(filterdFieldValues), anyNull)) // "%s.%s IN (?) OR %s.%s IS NULL)
		} else {
			latestnonWhereArr = append(latestnonWhereArr, comparisonOpStmt(tableName, strcase.SnakeCase(filter.FieldName), filter.PredicatesArr))
		}

		transformedValues = append(transformedValues, filterdFieldValues...)

	}

	// if len(transformedValues) == 0 {
	// 	return db, fmt.Errorf("latestn cannot be used without querying field value")
	// }

	// it's possible that there isn't any WHERE clause inside the CTE but we still have to paritition by

	for _, latestnon := range latestnons {
		actualFieldName, err := mdl.JSONKeysToFieldName(m, latestnon)
		if err != nil {
			return db, err
		}
		fieldName, err := mdl.FieldNameToColumn(m, actualFieldName)
		if err != nil {
			return db, err
		}

		partitionByArr = append(partitionByArr, fieldName)
	}

	partitionBy := strings.Join(partitionByArr, ", ")

	var sb strings.Builder

	if len(latestnonWhereArr) > 0 {
		latestnonWhereStmt := strings.Join(latestnonWhereArr, " AND ")
		// The latestnon can be here in the WHERE clause
		sb.WriteString(fmt.Sprintf("INNER JOIN (SELECT id, DENSE_RANK() OVER (PARTITION by %s ORDER BY created_at DESC) FROM %s WHERE %s) AS latestn ",
			partitionBy, tableName, latestnonWhereStmt)) // WHERE fieldName = fieldValue
		sb.WriteString(fmt.Sprintf("ON %s AND %s.id = latestn.id AND latestn.dense_rank <= ?", latestnonWhereStmt, tableName))
	} else { // having partition by (latestnon), but no where clause on latestnon
		// The latestnon can be here in the WHERE clause
		sb.WriteString(fmt.Sprintf("INNER JOIN (SELECT id, DENSE_RANK() OVER (PARTITION by %s ORDER BY created_at DESC) FROM %s) AS latestn ",
			partitionBy, tableName)) // WHERE fieldName = fieldValue
		sb.WriteString(fmt.Sprintf("ON %s.id = latestn.id AND latestn.dense_rank <= ?", tableName))
	}

	stmt := sb.String()

	transformedValues = append(transformedValues, transformedValues...)
	transformedValues = append(transformedValues, latestn)

	db = db.Joins(stmt, transformedValues...)
	return db, nil
}

// AddLatestJoinWithOneLevelFilter generates latest join with one-level filter
// TODO? Can tablename be part of the "?"
func AddLatestJoinWithOneLevelFilter(db *gorm.DB, typeString string, tableName string, latestn int, filters []FilterCriteria) (*gorm.DB, error) {
	hasLatestOn := filterHasLateston(filters)
	var partitionBy, latestnonWhereStmt, whereOnStmt string
	var transformedValues []interface{}
	var err error
	if hasLatestOn {
		partitionBy, latestnonWhereStmt, whereOnStmt, transformedValues, err = latestnGetPartitionWhereAndTransformedValues2(typeString, tableName, filters)
		if err != nil {
			return db, err
		}
	} else {
		partitionBy, latestnonWhereStmt, transformedValues, err = latestnGetPartitionWhereAndTransformedValues(typeString, tableName, filters)
		if err != nil {
			return db, err
		}
	}

	var sb strings.Builder
	// The latestnon can be here in the WHERE clause
	sb.WriteString(fmt.Sprintf("INNER JOIN (SELECT id, DENSE_RANK() OVER (PARTITION by %s ORDER BY created_at DESC) FROM %s WHERE %s) AS latestn ",
		partitionBy, tableName, latestnonWhereStmt)) // WHERE fieldName = fieldValue

	// All "where" clause stuff can go on as "ON" clause here
	if whereOnStmt != "" {
		sb.WriteString(fmt.Sprintf("ON %s AND %s.id = latestn.id AND latestn.dense_rank <= ?", whereOnStmt, tableName))
	} else {
		sb.WriteString(fmt.Sprintf("ON %s.id = latestn.id AND latestn.dense_rank <= ?", tableName))
	}

	stmt := sb.String()

	transformedValues = append(transformedValues, latestn) // tofix

	db = db.Joins(stmt, transformedValues...)
	return db, nil
}

func latestnGetPartitionWhereAndTransformedValues(typeString, tableName string, filters []FilterCriteria) (string, string, []interface{}, error) {
	partitionByArr := make([]string, 0)
	whereArr := make([]string, 0)
	transformedValues := make([]interface{}, 0)

	for _, filter := range filters {
		// If there is any equality comparison other than equal
		// there shouldn't be any IN then
		hasEquality := false
		for _, predicates := range filter.PredicatesArr {
			for _, predicate := range predicates {
				if predicate.PredicateLogic == qry.PredicateCondEQ {
					hasEquality = true
				}
			}
		}

		m := registry.NewFromTypeString(typeString)

		urlFieldValues := make([]string, 0)
		for _, predicates := range filter.PredicatesArr {
			for _, predicate := range predicates {
				urlFieldValues = append(urlFieldValues, predicate.FieldValue)
			}
		}

		transformedFieldValues, err := getTransformedValueFromValidField(m,
			letters.CamelCaseToPascalCase(filter.FieldName), urlFieldValues)

		if _, ok := err.(*datatype.FieldNotInModelError); ok {
			continue
		}
		if err != nil {
			return "", "", nil, err
		}

		fiterdFieldValues, anyNull := filterNullValue(transformedFieldValues)
		// If passed, the field is part of the data structure

		fieldName := strcase.SnakeCase(filter.FieldName)
		partitionByArr = append(partitionByArr, fieldName)

		// One field is either >= or =, so we split by equality here
		if hasEquality {
			whereArr = append(whereArr, inOpStmt(tableName, fieldName, len(fiterdFieldValues), anyNull)) // "%s.%s IN (%s)
		} else {
			whereArr = append(whereArr, comparisonOpStmt(tableName, strcase.SnakeCase(filter.FieldName), filter.PredicatesArr))
		}

		transformedValues = append(transformedValues, fiterdFieldValues...)
	}

	if len(transformedValues) == 0 {
		return "", "", nil, fmt.Errorf("latestn cannot be used without querying field value")
	}

	partitionBy := strings.Join(partitionByArr, ", ")
	whereOnStmt := strings.Join(whereArr, " AND ")

	return partitionBy, whereOnStmt, transformedValues, nil
}

func getLatestnonsAndOthers(filters2 []FilterCriteria) (map[string]bool, []FilterCriteria) {
	// filter out latestnon, which are not really fields
	latestnons := make(map[string]bool, 0)
	filters := make([]FilterCriteria, 0)
	for _, filter := range filters2 {
		if filter.FieldName == "latestnon" {
			for _, predicates := range filter.PredicatesArr {
				// vals := make([]string, len(predicates))
				for _, predicate := range predicates {
					latestnons[predicate.FieldValue] = true
				}
			}
		} else {
			filters = append(filters, filter)
		}
	}
	return latestnons, filters
}

func latestnGetPartitionWhereAndTransformedValues2(typeString, tableName string, filters2 []FilterCriteria) (string, string, string, []interface{}, error) {
	latestnons, filters := getLatestnonsAndOthers(filters2)

	partitionByArr := make([]string, 0)
	latestnonWhereArr := make([]string, 0)
	transformedValues := make([]interface{}, 0)

	// First, get latestn filters
	for _, filter := range filters {
		if _, ok := latestnons[filter.FieldName]; !ok {
			// not part of the "latestnon"
			continue
		}

		// If there is any equality comparison other than equal
		// there shouldn't be any IN then
		hasEquality := false
		for _, predicates := range filter.PredicatesArr {
			for _, predicate := range predicates {
				if predicate.PredicateLogic == qry.PredicateCondEQ {
					hasEquality = true
				}
			}
		}

		m := registry.NewFromTypeString(typeString)

		urlFieldValues := make([]string, 0)
		for _, predicates := range filter.PredicatesArr {
			for _, predicate := range predicates {
				urlFieldValues = append(urlFieldValues, predicate.FieldValue)
			}
		}

		transformedFieldValues, err := getTransformedValueFromValidField(m,
			letters.CamelCaseToPascalCase(filter.FieldName), urlFieldValues)

		if _, ok := err.(*datatype.FieldNotInModelError); ok {
			continue
		}
		if err != nil {
			return "", "", "", nil, err
		}

		fiterdFieldValues, anyNull := filterNullValue(transformedFieldValues)
		// If passed, the field is part of the data structure

		fieldName := strcase.SnakeCase(filter.FieldName)
		partitionByArr = append(partitionByArr, fieldName)

		// One field is either >= or =, so we split by equality here
		if hasEquality {
			latestnonWhereArr = append(latestnonWhereArr, inOpStmt(tableName, fieldName, len(fiterdFieldValues), anyNull)) // "%s.%s IN (%s)
		} else {
			latestnonWhereArr = append(latestnonWhereArr, comparisonOpStmt(tableName, strcase.SnakeCase(filter.FieldName), filter.PredicatesArr))
		}

		transformedValues = append(transformedValues, fiterdFieldValues...)
	}

	// After this point, there is partitionByArr, latestnonWhereArr, and transformedValues,

	if len(transformedValues) == 0 {
		return "", "", "", nil, fmt.Errorf("latestn cannot be used without querying field value")
	}

	partitionBy := strings.Join(partitionByArr, ", ")
	latestnWhereStmt := strings.Join(latestnonWhereArr, " AND ")
	whereArr := make([]string, 0)

	// Then, get those filtered criteria that's NOT part of the latestnon
	for _, filter := range filters {
		if _, ok := latestnons[filter.FieldName]; ok {
			// if found, skip
			continue
		}

		// If there is any equality comparison other than equal
		// there shouldn't be any IN then
		hasEquality := false
		for _, predicates := range filter.PredicatesArr {
			for _, predicate := range predicates {
				if predicate.PredicateLogic == qry.PredicateCondEQ {
					hasEquality = true
				}
			}
		}

		m := registry.NewFromTypeString(typeString)

		urlFieldValues := make([]string, 0)
		for _, predicates := range filter.PredicatesArr {
			for _, predicate := range predicates {
				urlFieldValues = append(urlFieldValues, predicate.FieldValue)
			}
		}

		transformedFieldValues, err := getTransformedValueFromValidField(m,
			letters.CamelCaseToPascalCase(filter.FieldName), urlFieldValues)

		if _, ok := err.(*datatype.FieldNotInModelError); ok {
			continue
		}
		if err != nil {
			return "", "", "", nil, err
		}

		fiterdFieldValues, anyNull := filterNullValue(transformedFieldValues)
		// If passed, the field is part of the data structure

		fieldName := strcase.SnakeCase(filter.FieldName)
		// partitionByArr = append(partitionByArr, fieldName)

		// One field is either >= or =, so we split by equality here
		if hasEquality {
			whereArr = append(whereArr, inOpStmt(tableName, fieldName, len(fiterdFieldValues), anyNull)) // "%s.%s IN (%s)
		} else {
			whereArr = append(whereArr, comparisonOpStmt(tableName, strcase.SnakeCase(filter.FieldName), filter.PredicatesArr))
		}

		transformedValues = append(transformedValues, fiterdFieldValues...)
	}

	// if len(transformedValues) == 0 {
	// 	return "", "", "", nil, fmt.Errorf("latestn cannot be used without querying field value")
	// }

	// partitionBy := strings.Join(partitionByArr, ", ")
	whereOnStmt := strings.Join(whereArr, " AND ")

	return partitionBy, latestnWhereStmt, whereOnStmt, transformedValues, nil
	// return partitionBy, latestnWhereStmt, globalWhereStmt, transformedValues
}

// ===========================================
// Private
// ===========================================

// inOpWithFields generates statements
// xxx IN (?, ?, ?)
// and
// (x1, x2, x3)
// from better_other_queries
// This doesn't fill in the values
func inOpStmt(tableName string, fieldName string, numfieldValues int, checkNull bool) string {
	tableAndField := fmt.Sprintf(`"%s"."%s"`, tableName, fieldName)

	// A simple IN clause is OK except when I need to check if the field is an null value
	// then the IN clause won't work, need to do
	// (fieldName IN ('fieldValue1', 'fieldValue2') OR fieldName IS NULL)

	var stmt strings.Builder
	if numfieldValues >= 1 {
		questionMarks := strings.Repeat("?,", numfieldValues)
		questionMarks = questionMarks[:len(questionMarks)-1]

		// Need to build individual ? because the IN clause and other ? are filled together at once
		// If we don't spell them all out it's going to get misplaced
		stmt.WriteString(fmt.Sprintf("%s IN (%s)", tableAndField, questionMarks))
	}

	if numfieldValues >= 1 && checkNull {
		stmt.WriteString(" OR ")
	}

	if checkNull {
		stmt.WriteString(fmt.Sprintf("%s IS NULL", tableAndField))
	}

	return stmt.String()
}

func comparisonOpStmt(tableName string, fieldName string, predicatesArr [][]Predicate) string {
	tableAndField := fmt.Sprintf(`"%s"."%s"`, tableName, fieldName)

	// predicatesArr[] is OR relationships, inside is AND relationships
	var stmt strings.Builder
	predicates := predicatesArr[0]
	stmt.WriteString(fmt.Sprintf(" (%s %s ?", tableAndField, string(predicates[0].PredicateLogic)))
	for _, predicate := range predicates[1:] {
		stmt.WriteString(fmt.Sprintf(" AND %s %s ?", tableAndField, string(predicate.PredicateLogic)))
	}
	stmt.WriteString(") ")

	for _, predicates := range predicatesArr[1:] {
		// OR for the predicatesArr (outer)
		// AND for inside
		stmt.WriteString(fmt.Sprintf(" OR (%s %s ?", tableAndField, string(predicates[0].PredicateLogic)))
		for _, predicate := range predicates[1:] {
			stmt.WriteString(fmt.Sprintf(" AND %s %s ?", tableAndField, string(predicate.PredicateLogic)))
		}
		stmt.WriteString(") ")
	}

	return stmt.String()
}

// getTransformedValueFromValidField make sure the field does exist in struct
// and output the field value in correct types
func getTransformedValueFromValidField(modelObj interface{}, structFieldName string, urlFieldValues []string) ([]interface{}, error) {
	// Important!! Check if fieldName is actually part of the schema, otherwise risk of sequal injection

	fieldType, err := datatype.GetModelFieldTypeElmIfValid(modelObj, letters.CamelCaseToPascalCase(structFieldName))
	if err != nil {
		log.Println("getTransformedValueFromValidField err:", err)
		return nil, err
	}

	transURLFieldValues, err := datatype.TransformFieldValues(fieldType.String(), urlFieldValues)
	if err != nil {
		return nil, err
	}

	return transURLFieldValues, nil
}

func filterNullValue(transformedFieldValues []interface{}) (filtered []interface{}, anyNull bool) {
	// Filter out the "null" ones
	anyNull = false
	filtered = make([]interface{}, 0)
	for _, value := range transformedFieldValues {
		if isNil(value) {
			anyNull = true
		} else { // when isNil panic and recovered it goes here..I'm not sure how it works but this is what I need
			filtered = append(filtered, value)
		}
	}
	return filtered, anyNull
}

// https://mangatmodi.medium.com/go-check-nil-interface-the-right-way-d142776edef1
func isNil(i interface{}) bool {
	// Will panic for value type such as string and int
	defer func() {
		if r := recover(); r != nil {
			// fmt.Println("Recovered in f", r)
			return // for string type and stuff..
		}
	}()
	return i == nil || reflect.ValueOf(i).IsNil()
}
