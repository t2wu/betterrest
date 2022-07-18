package userrole

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
