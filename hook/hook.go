package hook

import (
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/hook/rest"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/qry/mdl"
)

// Interface for handlers
// It seems better to pass structure than to use signature
// Changing signature without changing hook will silently ignore them

// ------------------------------------------------------------------------------------
// New REST and others

type InitData struct {
	// Role of this user in relation to this data, only available during read
	Roles []userrole.UserRole

	// Info is endpoint information
	Ep *EndPoint
}

// Cargo is payload between hookpoints
type Cargo struct {
	Payload interface{}
}

// Data is the data send to batch model hookpoints
type Data struct {
	// Ms is the slice of IModels
	Ms []mdl.IModel
	// DB is the DB handle
	DB *gorm.DB
	// Cargo between Before and After hookpoints (not used in AfterRead since there is before read hookpoint.)
	Cargo *Cargo
	// Role of this user in relation to this data, only available during read
	Roles []userrole.UserRole
}

// Endpoint information
type EndPoint struct {
	// TypeString
	TypeString string

	URL         string
	Op          rest.Op
	Cardinality rest.Cardinality

	// URL parameters
	URLParams map[urlparam.Param]interface{}

	// Who is operating this CRUPD right now
	Who mdlutil.UserIDFetchable
}

// End new REST Op
// ------------------------------------------------------------------------------------

// Type for all handlers

type IHook interface {
	// Init data for this REST operation
	Init(data *InitData, args ...interface{})
}

// IBeforeApply before patching operation occurred. Only called for Patch.
// This comes before patch is applied. Before "Before"
type IBeforeApply interface {
	BeforeApply(data *Data, info *EndPoint) *webrender.RetError
}

// IBefore supports method to be called before data is fetched for all CRUPD operations
type IBefore interface {
	Before(data *Data, info *EndPoint) *webrender.RetError
}

// IAfter supports method to be called after data is after all CRUPD operations
type IAfter interface {
	After(data *Data, info *EndPoint) *webrender.RetError
}

// IAfterTransact is the method to be called after data is after the entire database
// transaction is done. No error is returned because database transaction is already committed.
type IAfterTransact interface {
	AfterTransact(data *Data, info *EndPoint)
}

// IRender is for formatting IModel with a custom function
// basically do your own custom output
// If return false, use the default JSON output
type IRender interface {
	Render(c *gin.Context, data *Data, ep *EndPoint, total *int) bool
}
