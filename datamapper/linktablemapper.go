package datamapper

import (
	"sync"

	"github.com/t2wu/betterrest/datamapper/service"
)

// ---------------------------------------

var (
	onceLinkTableMapper sync.Once
	linkeTableMapper    IDataMapper
)

// SetLinkTableMapper allows one to mock LinkTableMapper for testing
func SetLinkTableMapper(mapper IDataMapper) {
	onceLinkTableMapper.Do(func() {
		linkeTableMapper = mapper
	})
}

// SharedLinkTableMapper creats a singleton of Crud object
func SharedLinkTableMapper() IDataMapper {
	onceLinkTableMapper.Do(func() {
		linkeTableMapper = &BaseMapper{Service: &service.LinkTableService{BaseService: service.BaseService{}}}
	})

	return linkeTableMapper
}
