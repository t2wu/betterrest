package datamapper

import (
	"sync"

	"github.com/t2wu/betterrest/datamapper/service"
)

// ---------------------------------------

var onceOrganizationMapper sync.Once
var organizationMapper IDataMapper

// SharedOrganizationMapper creats a singleton of Crud object
func SharedOrganizationMapper() IDataMapper {
	onceOrganizationMapper.Do(func() {
		organizationMapper = &BaseMapper{Service: &service.OrganizationService{BaseService: service.BaseService{}}}
	})

	return organizationMapper
}
