package datamapper

import (
	"sync"

	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/model/mappertype"
)

// ---------------------------------------

var (
	onceOrganizationMapper sync.Once
	underOrgMapper         IDataMapper
)

// SetOrganizationMapper allows one to mock OrganizationMapper for testing
func SetOrganizationMapper(mapper IDataMapper) {
	onceOrganizationMapper.Do(func() {
		underOrgMapper = mapper
	})
}

// SharedOrganizationMapper creats a singleton of Crud object
func SharedOrganizationMapper() IDataMapper {
	onceOrganizationMapper.Do(func() {
		underOrgMapper = &DataMapper{
			Service: &service.UnderOrgService{BaseService: service.BaseService{}},
			MapperType: mappertype.UnderOrg,
		}
	})

	return underOrgMapper
}
