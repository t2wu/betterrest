package hookhandler

import (
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
)

// Interface for controllers
// It seems better to pass structure than to use signature
// Changing signature without changing controller will silently ignore them

// ------------------------------------------------------------------------------------
// New REST and others

type ControllerInitData struct {
	Who models.UserIDFetchable
	// TypeString
	TypeString string
	// Role of this user in relation to this data, only available during read
	Roles []models.UserRole
	// URL parameters
	URLParams map[urlparam.Param]interface{}
	// Info is endpoint information
	Info *EndPointInfo
}

// Cargo is payload between hookpoints
type Cargo struct {
	Payload interface{}
}

// Data is the data send to batch model hookpoints
type Data struct {
	// Ms is the slice of IModels
	Ms []models.IModel
	// DB is the DB handle
	DB *gorm.DB
	// Who is operating this CRUPD right now
	Who models.UserIDFetchable
	// TypeString
	TypeString string
	// Cargo between Before and After hookpoints (not used in AfterRead since there is before read hookpoint.)
	Cargo *Cargo
	// Role of this user in relation to this data, only available during read
	Roles []models.UserRole
	// URL parameters
	URLParams map[urlparam.Param]interface{}
}

// Endpoint information
type EndPointInfo struct {
	Op          RESTOp
	Cardinality APICardinality
}

func HTTPMethodToRESTOp(method string) RESTOp {
	switch method {
	case "GET":
		return RESTOpRead
	case "POST":
		return RESTOpCreate
	case "UPDATE":
		return RESTOpUpdate
	case "PATCH":
		return RESTOpPatch
	case "DELETE":
		return RESTOpDelete
	default:
		return RESTOpOther // shouldn't be here
	}
}

// Method designates the type of operations for BeforeCRUPD and AfterCRUPD hookpoints
type RESTOp int

const (
	RESTOpOther RESTOp = iota // should not be used
	RESTOpRead
	RESTOpCreate
	RESTOpUpdate
	RESTOpPatch
	RESTOpDelete
)

type APICardinality int

const (
	APICardinalityOne  APICardinality = 1
	APICardinalityMany APICardinality = 2
)

// End new REST Op
// ------------------------------------------------------------------------------------

// Type for all controllers

type IHookhandler interface {
	// Initialize data for this REST operation
	Initialize(data *ControllerInitData)
}

// IBeforeApply before patching operation occurred. Only called for Patch.
// This comes before patch is applied. Before "Before"
type IBeforeApply interface {
	BeforeApply(data *Data, info *EndPointInfo) *webrender.RetError
}

// IBefore supports method to be called before data is fetched for all CRUPD operations
type IBefore interface {
	Before(data *Data, info *EndPointInfo) *webrender.RetError
}

// IAfter supports method to be called after data is after all CRUPD operations
type IAfter interface {
	After(data *Data, info *EndPointInfo) *webrender.RetError
}

// IAfterTransact is the method to be called after data is after the entire database
// transaction is done. No error is returned because database transaction is already committed.
type IAfterTransact interface {
	AfterTransact(data *Data, info *EndPointInfo)
}

// IHasRenderer is for formatting IModel with a custom function
// basically do your own custom output
// If return false, use the default JSON output
// For batch renderer, register a Render(r UserRole, who models.UserIDFetchable, modelObjs []IModel) bool
type IHasRenderer interface {
	Render(c *gin.Context, data *Data, op *EndPointInfo) bool
}
