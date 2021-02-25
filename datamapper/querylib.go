package datamapper

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/utils/letters"
	"github.com/t2wu/betterrest/libs/utils/sqlbuilder"
	"github.com/t2wu/betterrest/models"
)

func constructDbFromURLFieldQuery(db *gorm.DB, typeString string, urlParams map[string][]string, latestn *int) (*gorm.DB, error) {
	var err error

	// If there is NO latestn, a simple WHERE clause will suffice
	// IF thee IS latestn, then we use INNER JOIN with a dense_rank
	if latestn == nil {
		for fieldName, fieldValues := range urlParams {
			// If querying nested field, skip
			if strings.Contains(fieldName, ".") {
				continue
			}

			predicates := make([]sqlbuilder.FilterPredicate, len(fieldValues))

			// Check if there is any predicate
			hasFilterPredicateEQ := false
			hasFilterPredicateNotEQ := false
			for i, fieldValue := range fieldValues {
				if strings.HasPrefix(fieldValue, "<") {
					predicates[i] = sqlbuilder.FilterPredicateLT
					fieldValues[i] = fieldValue[1:]
					hasFilterPredicateNotEQ = true
				} else if strings.HasPrefix(fieldValue, "<=") {
					predicates[i] = sqlbuilder.FilterPredicateLTEQ
					fieldValues[i] = fieldValue[2:]
					hasFilterPredicateNotEQ = true
				} else if strings.HasPrefix(fieldValue, ">") {
					predicates[i] = sqlbuilder.FilterPredicateGT
					fieldValues[i] = fieldValue[1:]
					hasFilterPredicateNotEQ = true
				} else if strings.HasPrefix(fieldValue, ">=") {
					predicates[i] = sqlbuilder.FilterPredicateGTEQ
					fieldValues[i] = fieldValue[2:]
					hasFilterPredicateNotEQ = true
				} else {
					predicates[i] = sqlbuilder.FilterPredicateEQ
					hasFilterPredicateEQ = true
				}
			}

			log.Printf("predicates: %+v\n", predicates)
			// Cannot both be true
			if hasFilterPredicateEQ && hasFilterPredicateNotEQ {
				return db, fmt.Errorf("cannot use both equality and other comparisons")
			}

			criteria := sqlbuilder.FilterCriteria{
				FieldName:   fieldName,
				FieldValues: fieldValues,
				Predicates:  predicates,
			}

			// We used the fact that repeatedly call AddWhereStmt genereates only ONE WHERE with multiple filters
			db, err = sqlbuilder.AddWhereStmt(db, typeString, models.GetTableNameFromTypeString(typeString), criteria)
			if err != nil {
				return db, err
			}
		}
	} else {
		filters := make([]sqlbuilder.FilterCriteria, 0)
		for fieldName, fieldValues := range urlParams {
			// If querying nested field, skip
			if strings.Contains(fieldName, ".") {
				continue
			}

			criteria := sqlbuilder.FilterCriteria{
				FieldName:   fieldName,
				FieldValues: fieldValues,
			}
			filters = append(filters, criteria)
		}

		db, err = sqlbuilder.AddLatestJoinWithOneLevelFilter(db, typeString, models.GetTableNameFromTypeString(typeString), *latestn, filters)
		if err != nil {
			return db, err
		}
	}

	return db, nil
}

func constructDbFromURLInnerFieldQuery(db *gorm.DB, typeString string, urlParams map[string][]string, latestn *int) (*gorm.DB, error) {
	urlParamDic := urlLevel2ParametersToMapOfMap(urlParams)

	if len(urlParamDic) != 0 {
		if latestn != nil {
			return db, errors.New("latestn with two-level query is currently not supported")
		}
	}

	obj := models.NewFromTypeString(typeString)

	for outerFieldName, filters := range urlParamDic {
		// Important!! Check if fieldName is actually part of the schema, otherwise risk of sequal injection
		innerType, err := datatypes.GetModelFieldTypeElmIfValid(obj, letters.CamelCaseToPascalCase(outerFieldName))
		if err != nil {
			return nil, err
		}

		rtable := models.GetTableNameFromTypeString(typeString)
		innerTable := strcase.SnakeCase(strings.Split(innerType.String(), ".")[1])
		twoLevelFilter := sqlbuilder.TwoLevelFilterCriteria{
			OuterTableName: rtable,
			InnerTableName: innerTable,
			OuterFieldName: outerFieldName,
			Filters:        filters,
		}
		db, err = sqlbuilder.AddNestedQueryJoinStmt(db, typeString, twoLevelFilter)
		if err != nil {
			return nil, err
		}
	}

	return db, nil
}

// ***************************************
// Private within this file
// ***************************************

func urlLevel2ParametersToMapOfMap(urlParameters map[string][]string) map[string][]sqlbuilder.FilterCriteria {
	dic := make(map[string][]sqlbuilder.FilterCriteria, 0)
	for fieldName, fieldValues := range urlParameters { // map, fieldName, fieldValues
		toks := strings.Split(fieldName, ".")
		if len(toks) != 2 { // Currently only allow one level of nesting
			continue
		}
		outerFieldName, innerFieldName := toks[0], toks[1]
		_, ok := dic[outerFieldName]
		if !ok {
			dic[outerFieldName] = make([]sqlbuilder.FilterCriteria, 0)
		}

		// criteriaArray := dic[outerFieldName]
		filterc := sqlbuilder.FilterCriteria{FieldName: innerFieldName, FieldValues: fieldValues}
		dic[outerFieldName] = append(dic[outerFieldName], filterc)
	}
	return dic
}
