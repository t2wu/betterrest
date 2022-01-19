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
	BuildQuery(modelObj models.IModel) (*gorm.DB, error)
	Take(modelObj models.IModel) IQuery
	First(modelObj models.IModel) IQuery
	Find(modelObjs interface{}) IQuery
	Count(modelObj models.IModel, no *int) IQuery
	Create(modelObj models.IModel) IQuery
	CreateMany(modelObjs []models.IModel) IQuery
	Delete(modelObj models.IModel) IQuery
	DeleteMany(modelObjs []models.IModel) IQuery
	Save(modelObj models.IModel) IQuery
	// Update(modelObjs interface{}, attrs ...interface{}) IQuery
	Update(modelObj models.IModel, p *PredicateRelationBuilder) IQuery
	GetDB() *gorm.DB
	Reset() IQuery
	Error() error
}
