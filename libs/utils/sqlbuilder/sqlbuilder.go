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

// FilterCriteria is the criteria to partition by (for SQL rank windows function)
type FilterCriteria struct {
	TableNameSnakeCase string
	FieldName          string   // Field name to match
	FieldValues        []string // Criteria to match
}

// type

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
func AddWhereStmt(db *gorm.DB, typeString string, criteria FilterCriteria) (*gorm.DB, error) {
	fieldName := criteria.FieldName
	tableNameSnakeCase := criteria.TableNameSnakeCase
	fieldValues := criteria.FieldValues

	transformedFieldValues, err := getTransformedValueFromValidField(typeString, letters.CamelCaseToPascalCase(fieldName), fieldValues)
	if err != nil {
		return nil, err
	}

	// Gorm will actually use one WHERE clause with AND statements if Where is called repeatedly
	whereStmt := InOpWithFields(tableNameSnakeCase, strcase.SnakeCase(fieldName), len(fieldValues))
	db = db.Where(whereStmt, transformedFieldValues...)
	return db, nil
}

// AddNestedJoinStmt adds a join statement into db
func AddNestedJoinStmt(db *gorm.DB, typeString string, criteria FilterCriteria) (*gorm.DB, error) {
	return nil, nil
}

// InOpWithFields generates statements
// xxx IN (?, ?, ?)
// and
// (x1, x2, x3)
// from better_other_queries
func InOpWithFields(tableName string, fieldName string, numfieldValues int) string {
	tableName = "\"" + tableName + "\""
	fieldName = "\"" + fieldName + "\""
	blanks := strings.Repeat("?,", numfieldValues)
	blanks = blanks[:len(blanks)-1]
	return fmt.Sprintf("%s.%s IN (%s) ", tableName, fieldName, blanks)
}

// ===========================================
// Private
// ===========================================

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
