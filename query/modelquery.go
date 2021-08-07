package query

import (
	"fmt"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

func ByID(tx *gorm.DB, modelObj models.IModel, id *datatypes.UUID) error {
	if err := tx.Set("gorm:auto_preload", true).Model(modelObj).Where("id = ?", id).
		First(modelObj).Error; err != nil {
		return err
	}
	return nil
}

func FirstByField(tx *gorm.DB, modelObj models.IModel, fieldName string, fieldValue interface{}) error {
	fieldQuery := fmt.Sprintf("%s = ?", fieldName)
	if err := tx.Set("gorm:auto_preload", true).Where(fieldQuery, fieldValue).
		Order("created_at DESC").First(modelObj).Error; err != nil {
		return err
	}
	return nil
}

type FieldQuery struct {
	Name  string
	Value interface{}
}

func FirstByFields(tx *gorm.DB, modelObj models.IModel, fqs []*FieldQuery) error {
	tx2 := tx.Set("gorm:auto_preload", true)
	for _, fq := range fqs {
		fieldQuery := fmt.Sprintf("%s = ?", fq.Name)
		tx2 = tx2.Where(fieldQuery, fq.Value)
	}

	if err := tx2.Order("created_at DESC").First(modelObj).Error; err != nil {
		return err
	}
	return nil
}
