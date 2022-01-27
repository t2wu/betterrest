package datamapper

import (
	"fmt"
	"log"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/utils/letters"
	"github.com/t2wu/betterrest/libs/utils/sqlbuilder"
	"github.com/t2wu/betterrest/models"
	qry "github.com/t2wu/betterrest/query"
)

func getPredicateAndValueFromFieldValue2(fieldVal string) (string, string) {
	if strings.HasPrefix(fieldVal, "<") {
		return fieldVal[0:1], fieldVal[1:]
	} else if strings.HasPrefix(fieldVal, "<=") {
		return fieldVal[0:2], fieldVal[2:]
	} else if strings.HasPrefix(fieldVal, ">") {
		return fieldVal[0:1], fieldVal[1:]
	} else if strings.HasPrefix(fieldVal, ">=") {
		return fieldVal[0:2], fieldVal[2:]
	} else { // no sign
		return "=", fieldVal
	}
}

func getPredicateAndValueFromFieldValue(fieldVal string) (qry.PredicateCond, string) {
	if strings.HasPrefix(fieldVal, "<") {
		return qry.PredicateCondLT, fieldVal[1:]
	} else if strings.HasPrefix(fieldVal, "<=") {
		return qry.PredicateCondLTEQ, fieldVal[2:]
	} else if strings.HasPrefix(fieldVal, ">") {
		return qry.PredicateCondGT, fieldVal[1:]
	} else if strings.HasPrefix(fieldVal, ">=") {
		return qry.PredicateCondGTEQ, fieldVal[2:]
	} else { // no sign
		return qry.PredicateCondEQ, fieldVal
	}
}

func constructFilterCriteriaFromFieldNameAndFieldValue(fieldName string, fieldValues []string) (*sqlbuilder.FilterCriteria, error) {
	predicatesArr := make([][]sqlbuilder.Predicate, len(fieldValues))

	// Check if there is any predicate
	hasFilterPredicateEQ := false
	hasFilterPredicateNotEQ := false

	// query parameter is fieldName, but it can have multiple fieldValues
	// And within a single fieldValue I could have ">30;<20"
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

			predicate.PredicateLogic, predicate.FieldValue = getPredicateAndValueFromFieldValue(fieldVal)
			if predicate.PredicateLogic != qry.PredicateCondEQ {
				hasFilterPredicateNotEQ = true
			} else {
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
		FieldName:     fieldName,
		PredicatesArr: predicatesArr,
	}
	return &criteria, nil
}

func createFiltersAndLatestnonFilters(urlParams map[string][]string, latestnons []string) ([]sqlbuilder.FilterCriteria, []sqlbuilder.FilterCriteria, error) {
	latestnmap := make(map[string]bool)
	for _, latestnon := range latestnons {
		latestnmap[latestnon] = true
	}

	// First, get the latestnon fields vs other fields into two arrays
	// 1. Get lateston fields and the other fields part
	filters := make([]sqlbuilder.FilterCriteria, 0)
	filterslatestnons := make([]sqlbuilder.FilterCriteria, 0)
	for fieldName, fieldValues := range urlParams {
		// If querying nested field, skip
		if strings.Contains(fieldName, ".") {
			continue
		}

		// not latestn on fields
		criteria, err := constructFilterCriteriaFromFieldNameAndFieldValue(fieldName, fieldValues)
		if err != nil {
			return nil, nil, err
		}

		if _, ok := latestnmap[fieldName]; ok {
			filterslatestnons = append(filterslatestnons, *criteria)
		} else {
			filters = append(filters, *criteria)
		}
	}
	return filters, filterslatestnons, nil
}

func constructDbFromURLFieldQuery(db *gorm.DB, typeString string, urlParams map[string][]string, latestn *int, latestnons []string) (*gorm.DB, error) {
	var err error
	filters, filterslatestnons, err := createFiltersAndLatestnonFilters(urlParams, latestnons)
	if err != nil {
		return db, err
	}

	for _, filter := range filters {
		// We used the fact that repeatedly call AddWhereStmt genereates only ONE WHERE with multiple filters
		db, err = sqlbuilder.AddWhereStmt(db, typeString, models.GetTableNameFromTypeString(typeString), filter)
		if _, ok := err.(*datatypes.FieldNotInModelError); ok {
			// custom url parameter
			continue
		}
		if err != nil {
			return db, err
		}
	}

	// If there is a latestn clause, construct it
	// something like this:
	// INNER JOIN (
	// 	SELECT id, DENSE_RANK() OVER (PARTITION by model, num_usb_type_as ORDER BY created_at DESC)
	// 	FROM dock WHERE "dock"."model" IN ('GOOD_WAY_DUD8070') AND "dock"."num_usb_type_as" IN ('3')
	// ) AS latestn
	// ON dock.id = latestn.id AND latestn.dense_rank <= 1

	// if latestn != nil && len(filterslatestnons) > 0 {
	if latestn != nil && len(latestnons) > 0 {
		// log.Println("filterslatestnons:", filterslatestnons)
		db, err = sqlbuilder.AddLatestNCTEJoin(db, typeString, models.GetTableNameFromTypeString(typeString), *latestn, latestnons, filterslatestnons)
		if err != nil {
			return db, err
		}
	} else if latestn != nil {
		log.Println("GOING INTO DEPRECATED")
		// DEPRECATED: old behavior where there may is latestn but not latestnons
		db, err = sqlbuilder.AddLatestJoinWithOneLevelFilter(db, typeString, models.GetTableNameFromTypeString(typeString), *latestn, filters)
		if err != nil {
			return db, err
		}
	} else {
		for _, filter := range filters {
			// We used the fact that repeatedly call AddWhereStmt genereates only ONE WHERE with multiple filters
			db, err = sqlbuilder.AddWhereStmt(db, typeString, models.GetTableNameFromTypeString(typeString), filter)
			if _, ok := err.(*datatypes.FieldNotInModelError); ok {
				// custom url parameter
				continue
			}
			if err != nil {
				return db, err
			}
		}
	}

	return db, nil
}

func constructDbFromURLInnerFieldQuery(db *gorm.DB, typeString string, urlParams map[string][]string, latestn *int) (*gorm.DB, error) {
	urlParamDic, err := urlLevel2ParametersToMapOfMap(urlParams)
	if err != nil {
		return db, err
	}

	// if len(urlParamDic) != 0 {
	// 	if latestn != nil {
	// 		return db, errors.New("latestn with two-level query is currently not supported")
	// 	}
	// }

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
