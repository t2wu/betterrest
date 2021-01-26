package datamapper

import (
	"sync"

	"github.com/t2wu/betterrest/datamapper/service"
)

// ---------------------------------------

var onceGlobal sync.Once
var crudGlobal IDataMapper

// SharedGlobalMapper creats a singleton of Crud object
func SharedGlobalMapper() IDataMapper {
	onceGlobal.Do(func() {
		crudGlobal = &BaseMapper{Service: &service.GlobalService{BaseService: service.BaseService{}}}
	})

	return crudGlobal
}
