package sqlbuilder

import (
	"fmt"
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

// If search is dock_id and others, I need more than one qery
// If there is NO other search field, then I simply rank

// LatestNRecords returns the latest N records, or if there are criteria to match, partition by the
// criteria, then use the latest N records per partition
func LatestNRecords(tableName string, n int, criteria []FilterCriteria) string {
	stmt := ""
	tableName = "\"" + tableName + "\""

	criteriaChain := ""
	if len(criteria) != 0 {
		criteriaChain = "PARTITION BY " + criteria[0].FieldName
		for _, c := range criteria[1:] {
			criteriaChain += (", " + c.FieldName)
		}
	}

	stmt = "INNER JOIN (" +
		fmt.Sprintf("SELECT id, DENSE_RANK() OVER (%s created_at desc) AS dense_rank FROM %s", criteriaChain, tableName) +
		") as latestn " +
		fmt.Sprintf("ON %s.id = latestn.id \n", tableName) +
		fmt.Sprintf("AND latest.dense_rank <= %d", n)

	return stmt
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
