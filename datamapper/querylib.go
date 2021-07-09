package datamapper

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/utils/letters"
	"github.com/t2wu/betterrest/libs/utils/sqlbuilder"
	"github.com/t2wu/betterrest/models"
)

func constructFilterCriteriaFromFieldNameAndFieldValue(fieldName string, fieldValues []string) (*sqlbuilder.FilterCriteria, error) {
	predicatesArr := make([][]sqlbuilder.Predicate, len(fieldValues))

	// Check if there is any predicate
	hasFilterPredicateEQ := false
	hasFilterPredicateNotEQ := false

	for i, fieldValue := range fieldValues {
		// fieldValue could be ">30;<20" for greater than 30 OR smaller than 20
		fieldValue = strings.TrimSpace(fieldValue)
		fieldValue = strings.TrimRight(fieldValue, ";")
		if fieldValue == "" {
			return nil, fmt.Errorf("query value shouldn't be empty")
		}

		fieldVals := strings.Split(fieldValue, ";")

		predicatesArr[i] = make([]sqlbuilder.Predicate, len(fieldVals))
		for j, fieldVal := range fieldVals {
			predicate := sqlbuilder.Predicate{}

			if strings.HasPrefix(fieldVal, "<") {
				predicate.PredicateLogic = sqlbuilder.FilterPredicateLogicLT
				predicate.FieldValue = fieldVal[1:]
				hasFilterPredicateNotEQ = true
			} else if strings.HasPrefix(fieldVal, "<=") {
				predicate.PredicateLogic = sqlbuilder.FilterPredicateLogicLTEQ
				predicate.FieldValue = fieldVal[2:]
				hasFilterPredicateNotEQ = true
			} else if strings.HasPrefix(fieldVal, ">") {
				predicate.PredicateLogic = sqlbuilder.FilterPredicateLogicGT
				predicate.FieldValue = fieldVal[1:]
				hasFilterPredicateNotEQ = true
			} else if strings.HasPrefix(fieldVal, ">=") {
				predicate.PredicateLogic = sqlbuilder.FilterPredicateLogicGTEQ
				predicate.FieldValue = fieldVal[2:]
				hasFilterPredicateNotEQ = true
			} else { // no sign
				predicate.PredicateLogic = sqlbuilder.FilterPredicateLogicEQ
				predicate.FieldValue = fieldVal
				hasFilterPredicateEQ = true
			}
			predicatesArr[i][j] = predicate
		}
	}

	// Cannot both be true
	if hasFilterPredicateEQ && hasFilterPredicateNotEQ {
		return nil, fmt.Errorf("cannot use both equality and other comparisons")
	}

	criteria := sqlbuilder.FilterCriteria{
		FieldName: fieldName,
		// FieldValues:   fieldValues,
		PredicatesArr: predicatesArr,
	}
	return &criteria, nil
}

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

			criteria, err := constructFilterCriteriaFromFieldNameAndFieldValue(fieldName, fieldValues)
			if err != nil {
				return db, err
			}

			// We used the fact that repeatedly call AddWhereStmt genereates only ONE WHERE with multiple filters
			db, err = sqlbuilder.AddWhereStmt(db, typeString, models.GetTableNameFromTypeString(typeString), *criteria)
			if _, ok := err.(*datatypes.FieldNotInModelError); ok {
				// custom url parameter
				continue
			}
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

			criteria, err := constructFilterCriteriaFromFieldNameAndFieldValue(fieldName, fieldValues)
			// if _, ok := err.(*datatypes.FieldNotInModelError); ok {
			// 	// custom url parameter
			// 	continue
			// }
			if err != nil {
				return db, err
			}

			filters = append(filters, *criteria)
		}

		db, err = sqlbuilder.AddLatestJoinWithOneLevelFilter(db, typeString, models.GetTableNameFromTypeString(typeString), *latestn, filters)
		if err != nil {
			return db, err
		}
	}

	return db, nil
}

func constructDbFromURLInnerFieldQuery(db *gorm.DB, typeString string, urlParams map[string][]string, latestn *int) (*gorm.DB, error) {
	urlParamDic, err := urlLevel2ParametersToMapOfMap(urlParams)
	if err != nil {
		return db, err
	}

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

func urlLevel2ParametersToMapOfMap(urlParameters map[string][]string) (map[string][]sqlbuilder.FilterCriteria, error) {
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

		criteria, err := constructFilterCriteriaFromFieldNameAndFieldValue(innerFieldName, fieldValues)
		if err != nil {
			return nil, err
		}

		dic[outerFieldName] = append(dic[outerFieldName], *criteria)
	}
	return dic, nil
}
