package datamapper

import (
	"sync"

	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/model/mappertype"
)

// ---------------------------------------

var (
	onceOwnership sync.Once
	crudOwnership IDataMapper
)

// SetMockOwnershipMapper allows one to mock OrganizationMapper for testing
func SetMockOwnershipMapper(mapper IDataMapper) {
	onceOwnership.Do(func() {
		crudOwnership = mapper
	})
}

// SharedOwnershipMapper creats a singleton of Crud object
func SharedOwnershipMapper() IDataMapper {
	onceOwnership.Do(func() {
		crudOwnership = &DataMapper{
			Service:    &service.OwnershipService{BaseService: service.BaseService{}},
			MapperType: mappertype.DirectOwnership,
		}
	})

	return crudOwnership
}
