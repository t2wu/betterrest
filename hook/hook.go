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
	// TODO: To be removed
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
	TypeString string `json:"typeString"`

	URL         string           `json:"url"`
	Op          rest.Op          `json:"op"`
	Cardinality rest.Cardinality `json:"cardinality"`

	// URL parameters
	URLParams map[urlparam.Param]interface{} `json:"urlParams"`

	// Who is operating this CRUPD right now
	Who mdlutil.UserIDFetchable `json:"who"`
}

// End new REST Op
// ------------------------------------------------------------------------------------

// Type for all handlers

type IHook interface {
	// Init data for this REST operation
	// (TODO: But role doesn't exists yet before read, and I can't seem to find any use for any of these data)
	Init(data *InitData, args ...interface{})
}

// IBeforeApply before patching operation occurred. Only called for Patch.
// This comes before patch is applied. Before "Before"
type IBeforeApply interface {
	BeforeApply(data *Data, info *EndPoint) *webrender.RetError
}

// IBefore supports method to be called before data is fetched for all operations except Read
type IBefore interface {
	Before(data *Data, info *EndPoint) *webrender.RetError
}

// IAfter supports method to be called after data is after all operations except delete
type IAfter interface {
	After(data *Data, info *EndPoint) *webrender.RetError
}

// ICache supports cache.
type ICache interface {
	// Get data by info. If exists, can be implemented to avoid hitting the database in read endpoints
	// if boolean is false, it means it is not handled and database query will be proceeded
	// unless another hook which implemented this takes over.
	// A maximum of one handler is used at a time, the hook writer has to make sure they are mutally exclusive
	// if found is false, it means it was not found in the database (only used for query with cardinality of 1)
	GetFromCache(info *EndPoint) (handled bool, found bool, ms []mdl.IModel, roles []userrole.UserRole, no *int, retErr *webrender.RetError)

	// A maximum of one handler is used at a time, the hook writer has to make sure they are mutally exclusive
	// if found is false, it means it was not found in the database and the negative result is to be cached.
	AddToCache(info *EndPoint, found bool, ms []mdl.IModel, roles []userrole.UserRole, no *int) (handled bool, retErr *webrender.RetError)
}

// IAfterTransact is the method to be called after data is after the entire database
// transaction is done. No error is returned because database transaction is already committed.
type IAfterTransact interface {
	AfterTransact(data *Data, info *EndPoint)
}

// IRender is for formatting IModel with a custom function
// basically do your own custom output
// If return false, use the default JSON output
// A maximum of one handler is used at a time, the hook writer has to make sure they are mutally exclusive
type IRender interface {
	Render(c *gin.Context, data *Data, ep *EndPoint, total *int) bool
}
