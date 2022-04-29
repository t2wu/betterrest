package datamapper

import (
	"sync"

	"github.com/t2wu/betterrest/datamapper/service"
)

// ---------------------------------------

var (
	onceOrganizationMapper sync.Once
	organizationMapper     IDataMapper
)

// SetOrganizationMapper allows one to mock OrganizationMapper for testing
func SetOrganizationMapper(mapper IDataMapper) {
	onceOrganizationMapper.Do(func() {
		organizationMapper = mapper
	})
}

// SharedOrganizationMapper creats a singleton of Crud object
func SharedOrganizationMapper() IDataMapper {
	onceOrganizationMapper.Do(func() {
		organizationMapper = &DataMapper{Service: &service.OrganizationService{BaseService: service.BaseService{}}}
	})

	return organizationMapper
}
