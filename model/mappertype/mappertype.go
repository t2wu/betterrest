package mappertype

// MapperType is the mapper type
type MapperType int

const (
	// Ownership is for type which user owns something
	DirectOwnership MapperType = iota

	// User is user itself
	User

	// ViaOrg is for type where an organization owns something
	UnderOrg

	// ViaOrgPartition is for type where an organization owns something and it's in partitioned table
	UnderOrgPartition

	// MapperTypeGlobal is for type where data is public to all
	Global

	// MapperTypeLinkTable is for table linking user and regular mdl
	LinkTable
)
