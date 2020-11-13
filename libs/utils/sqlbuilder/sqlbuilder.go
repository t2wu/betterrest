package sqlbuilder

import (
	"fmt"
	"log"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/utils/letters"
	"github.com/t2wu/betterrest/models"
)

// Something like this.
// Search by dense_rank

// FilterCriteria is the criteria to query for first-level field
type FilterCriteria struct {
	// TableName   string
	FieldName   string   // Field name to match
	FieldValues []string // Criteria to match
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
	transformedFieldValues, err := getTransformedValueFromValidField(typeString,
		letters.CamelCaseToPascalCase(filter.FieldName), filter.FieldValues)
	if err != nil {
		return nil, err
	}

	// Gorm will actually use one WHERE clause with AND statements if Where is called repeatedly
	whereStmt := inOpWithFields(tableName, strcase.SnakeCase(filter.FieldName),
		len(filter.FieldValues))
	log.Println("whereStmt: ", whereStmt)
	db = db.Where(whereStmt, transformedFieldValues...)
	return db, nil
}

// AddNestedQueryJoinStmt adds a join statement into db
func AddNestedQueryJoinStmt(db *gorm.DB, typeString string, criteria TwoLevelFilterCriteria) (*gorm.DB, error) {
	// join inner table and outer table based on outer table id
	joinStmt := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".%s = \"%s\".id ",
		criteria.InnerTableName, criteria.InnerTableName, criteria.OuterTableName+"_id", criteria.OuterTableName)

	queryValues := make([]interface{}, 0)

	for _, filter := range criteria.Filters {
		innerFieldName := filter.FieldName
		fieldValues := filter.FieldValues
		transformedValues, err := getTransformedValueFromValidField(typeString,
			letters.CamelCaseToPascalCase(criteria.OuterFieldName), fieldValues)
		if err != nil {
			return nil, err
		}

		// It's possible to have multiple values by using ?xx=yy&xx=zz
		// Get the inner table's type
		inStmt := inOpWithFields(criteria.InnerTableName, strcase.SnakeCase(innerFieldName), len(transformedValues))
		joinStmt += "AND " + inStmt

		queryValues = append(queryValues, transformedValues...)
	}

	db = db.Joins(joinStmt, queryValues...)

	return db, nil
}

// AddLatestJoinWithOneLevelFilter generates latest join with one-level filter
// TODO? Can tablename be part of the "?"
func AddLatestJoinWithOneLevelFilter(db *gorm.DB, typeString string, tableName string, latestN int, filters []FilterCriteria) (*gorm.DB, error) {
	partitionByArr := make([]string, 0)
	whereArr := make([]string, 0)

	transformedValues := make([]interface{}, 0)

	for _, filter := range filters {
		transformedFieldValues, err := getTransformedValueFromValidField(typeString,
			letters.CamelCaseToPascalCase(filter.FieldName), filter.FieldValues)
		if err != nil {
			return nil, err
		}

		// If passed, the field is part of the data structure

		fieldName := strcase.SnakeCase(filter.FieldName)
		partitionByArr = append(partitionByArr, fieldName)

		whereArr = append(whereArr, inOpWithFields(tableName, fieldName, len(filter.FieldValues))) // "%s.%s IN (%s)
		transformedValues = append(transformedValues, transformedFieldValues...)
	}
	partitionBy := strings.Join(partitionByArr, ", ")
	whereStmt := strings.Join(whereArr, " AND ")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("INNER JOIN (SELECT id, DENSE_RANK() OVER (PARTITION by %s ORDER BY created_at DESC) FROM %s WHERE %s) AS latestn ",
		partitionBy, tableName, whereStmt)) // WHERE fieldName = fieldValue
	sb.WriteString(fmt.Sprintf("ON %s.id = latestn.id AND latestn.dense_rank <= ?", tableName))
	stmt := sb.String()

	transformedValues = append(transformedValues, latestN)

	db = db.Joins(stmt, transformedValues...)
	return db, nil
}

// AddLatestJoinWithTwoLevelFilter generates latest join with two-level filter
// TODO? Can tablename be part of the "?"
// func AddLatestJoinWithTwoLevelFilter(db *gorm.DB, typeString string, tableName string, latestN int, filter FilterCriteria) {
// 	var sb strings.Builder
// 	sb.WriteString(fmt.Sprintf("INNER JOIN (SELECT %s, DENSE_RANK() OVER (PARTITION by %s ORDER BY created_at DESC) FROM %s) AS latestn ",
// 		tableName, filter.FieldName, tableName))
// 	sb.WriteString(fmt.Sprintf("ON %s.%s = latestn.%s AND %s.created_at = latestn.created_at AND ",
// 		tableName, filter.FieldName, filter.FieldName, tableName))
// 	sb.WriteString(fmt.Sprintf("latestn.dense_rank <= ?"))
// 	stmt := sb.String()
// }

// ===========================================
// Private
// ===========================================

// inOpWithFields generates statements
// xxx IN (?, ?, ?)
// and
// (x1, x2, x3)
// from better_other_queries
func inOpWithFields(tableName string, fieldName string, numfieldValues int) string {
	tableName = "\"" + tableName + "\""
	fieldName = "\"" + fieldName + "\""
	blanks := strings.Repeat("?,", numfieldValues)
	blanks = blanks[:len(blanks)-1]
	return fmt.Sprintf("%s.%s IN (%s) ", tableName, fieldName, blanks)
}

// getTransformedValueFromValidField make sure the field does exist in struct
// and output the field value in correct types
func getTransformedValueFromValidField(typeString, structFieldName string, urlFieldValues []string) ([]interface{}, error) {
	modelObj := models.NewFromTypeString(typeString)

	// Important!! Check if fieldName is actually part of the schema, otherwise risk of sequal injection
	fieldType, err := datatypes.GetModelFieldTypeElmIfValid(modelObj, letters.CamelCaseToPascalCase(structFieldName))
	if err != nil {
		return nil, err
	}

	transURLFieldValues, err := datatypes.TransformFieldValue(fieldType.String(), urlFieldValues)
	if err != nil {
		return nil, err
	}

	return transURLFieldValues, nil
}
