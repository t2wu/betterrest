package query

import (
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/models"
)

// IQuery so we can stubb out the DB
type IQuery interface {
	Q(args ...interface{}) IQuery
	Order(order string) IQuery
	Limit(limit int) IQuery
	Offset(offset int) IQuery
	InnerJoin(modelObj models.IModel, foreignObj models.IModel, args ...interface{}) IQuery
	First(modelObj models.IModel) IQuery
	Find(modelObjs interface{}) IQuery
	Create(modelObj models.IModel) IQuery
	Delete(modelObj models.IModel) IQuery
	Save(modelObj models.IModel) IQuery
	// Update(modelObjs interface{}, attrs ...interface{}) IQuery
	Update(modelObj models.IModel, p *PredicateRelationBuilder) IQuery
	GetDB() *gorm.DB
	GetDBOri() *gorm.DB
	Reset() IQuery
	Error() error
}
