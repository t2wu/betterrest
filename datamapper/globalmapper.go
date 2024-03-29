package datamapper

import (
	"sync"

	"github.com/t2wu/betterrest/datamapper/service"
)

// ---------------------------------------

var (
	onceGlobal sync.Once
	crudGlobal IDataMapper
)

// SetSharedGlobalMapper allows one to mock SharedGlobalMapper for testing
func SetSharedGlobalMapper(mapper IDataMapper) {
	onceGlobal.Do(func() {
		crudGlobal = mapper
	})
}

// SharedGlobalMapper creats a singleton of Crud object
func SharedGlobalMapper() IDataMapper {
	onceGlobal.Do(func() {
		crudGlobal = &BaseMapper{Service: &service.GlobalService{BaseService: service.BaseService{}}}
	})

	return crudGlobal
}
