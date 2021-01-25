package datamapper

import (
	"sync"

	"github.com/t2wu/betterrest/datamapper/service"
)

// ---------------------------------------

var onceLinkTableMapper sync.Once
var linkeTableMapper IDataMapper

// SharedLinkTableMapper creats a singleton of Crud object
func SharedLinkTableMapper() IDataMapper {
	onceLinkTableMapper.Do(func() {
		linkeTableMapper = &BaseMapper{Service: &service.LinkTableService{}}
	})

	return linkeTableMapper
}
