package datatypes

// ---------------------------------------
// VerificationActionType type with enum
type VerificationActionType int

const (
	// Negatives are not stored in DB

	// UserRoleAny not for value in db, but for permission where any is fine (link table)
	VerificationActionTypeNoAction      VerificationActionType = 0 // no action
	VerificationActionTypeVerifyEmail   VerificationActionType = 1
	VerificationActionTypeResetPassword VerificationActionType = 2
)
