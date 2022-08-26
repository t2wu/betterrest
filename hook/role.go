package hook

import (
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/model/mappertype"
)

type IRoleSorter interface {
	// UserRoleOnCreate is the hook for asking for user role on create
	// It also asks for whether this user has permission to edit the item.
	// If not, returns a RetError
	// UserRoleOnCreate(mapperType mapper.MapperType, data *Data, ep *EndPoint, roles []userrole.UserRole) ([]userrole.UserRole, *webrender.RetError)

	// Permit is for any other hook (read, update, patch, delete)
	// if user is permitted, return a RetError
	PermitOnCreate(mapperType mappertype.MapperType, data *Data, ep *EndPoint) *webrender.RetError

	// Returns permitted role as a map key.
	// If a role is not in it, it means it's denied.
	// If a role is in it but the map value is not nil, it means it is rejected with a custom error.
	// If error occurs while trying to generate the map, returns an error
	Permitted(mapperType mappertype.MapperType, ep *EndPoint) (map[userrole.UserRole]*webrender.RetError, error)
}
