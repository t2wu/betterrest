package datamapper

import (
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/utils/letters"
	"github.com/t2wu/betterrest/libs/utils/sqlbuilder"
	"github.com/t2wu/betterrest/models"
)

func constructDbFromURLFieldQuery(db *gorm.DB, typeString string, options map[string]interface{}) (*gorm.DB, error) {
	values, ok := options["better_otherqueries"].(url.Values)
	if !ok {
		return db, nil
	}

	var latestN *int
	if n, ok := options["latestn"]; ok {
		if n2, err := strconv.Atoi(n.(string)); err == nil {
			latestN = &n2
		}
	}

	var err error

	// If there is NO latestN, a simple WHERE clause will suffice
	// IF thee IS latestN, then we use INNER JOIN with a dense_rank
	if latestN == nil {
		for fieldName, fieldValues := range values {
			// If querying nested field, skip
			if strings.Contains(fieldName, ".") {
				continue
			}

			criteria := sqlbuilder.FilterCriteria{
				FieldName:   fieldName,
				FieldValues: fieldValues,
			}

			// We used the fact that repeatedly call AddWhereStmt genereates only ONE WHERE with multiple filters
			db, err = sqlbuilder.AddWhereStmt(db, typeString, models.GetTableNameFromTypeString(typeString), criteria)
			if err != nil {
				return db, err
			}
		}
	} else {
		filters := make([]sqlbuilder.FilterCriteria, 0)
		for fieldName, fieldValues := range values {
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

		db, err = sqlbuilder.AddLatestJoinWithOneLevelFilter(db, typeString, models.GetTableNameFromTypeString(typeString), *latestN, filters)
		if err != nil {
			return db, err
		}
	}

	// log.Println("==================>RETURN HERE", db)
	return db, nil
}

func constructDbFromURLInnerFieldQuery(db *gorm.DB, typeString string, options map[string]interface{}) (*gorm.DB, error) {
	urlParams, ok := options["better_otherqueries"].(url.Values)
	if !ok {
		return db, nil
	}
	urlParamDic := urlParametersToMapOfMap(urlParams)

	if len(urlParamDic) != 0 {
		if latestn, ok := options["latestn"]; ok {
			if latestn != "" {
				return db, errors.New("latestn with two-level query is currently not supported")
			}
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

// constructDbFromLatestN constructs the latest N queries. Bool indicates that the latest does exists.
// func constructDbFromLatestN(db *gorm.DB, typeString string, options map[string]interface{}) (*gorm.DB, bool, error) {
// 	latest := -1
// 	if _, ok := options["latest"]; ok {
// 		latest, _ = options["latest"].(int)
// 	}

// 	if latest == -1 {
// 		return db, false, nil
// 	}

// 	/*
// 		// WITH CTE (same as bove)
// 		WITH latestn AS (
// 		  SELECT *, DENSE_RANK() OVER (order by created_at desc)
// 		  FROM dock_status
// 		)
// 		SELECT dock_status.dock_id, dock_status.created_at, latestn.dense_rank
// 		FROM dock_status
// 		INNER JOIN latestn
// 		ON dock_status.dock_id = latestn.dock_id AND dock_status.created_at = latestn.created_at
// 		AND latestn.dense_rank <= 2;
// 	*/
// 	// stmt := "WITH latestn as (" +
// 	// 	"SELECT dock_id"
// 	// ")"

// 	return db, true, nil
// }

// ***************************************
// Private within this file
// ***************************************

func urlParametersToMapOfMap(urlParameters map[string][]string) map[string][]sqlbuilder.FilterCriteria {
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
