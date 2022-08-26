package userrole

// UserRole specifies the type of relationship (role) a user has to a REST resource (Ownershipmapper).
// Or, if the reource is an organization (OrganizationMapper), the all the resources
// within this organization.
// The permissions this role has is defined is by REST actions CRUPD to each resource.
// Several possibilities
// 1. The resource doensn't exist.
// 2. The resource exists, and there is a role the user has with this resource.
// If 2, the resource can be rejected in CRUP's before or after endpoint.
// But it's probably better to be centralized, because it is easier to view. Or at least able to be printed in
// a centralized fashion.

// TODO: This file should specify the relationships that the
// user can have with the subject, not something like UserRoleAny.

// UserRole type with enum
type UserRole int

const (
	// Negatives are not stored in DB

	// UserRoleAny not for value in db, but for permission where any is fine (link table)
	UserRoleAny UserRole = -2

	// UserRoleInvalid is invalid for this resource
	UserRoleInvalid UserRole = -1

	// UserRoleAdmin is admin UserRole
	UserRoleAdmin UserRole = 0

	// UserRoleGuest is guest UserRole (screw go-lint man)
	UserRoleGuest UserRole = 1

	// UserRolePublic to all (global object)
	UserRolePublic UserRole = 2

	// UserRoleTableBased is like admin but permission is subject to table control
	// Cannot delete site or alter permissions
	UserRoleTableBased UserRole = 3
)
