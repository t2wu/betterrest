package rest

func HTTPMethodToRESTOp(method string) Op {
	switch method {
	case "GET":
		return OpRead
	case "POST":
		return OpCreate
	case "UPDATE":
		return OpUpdate
	case "PATCH":
		return OpPatch
	case "DELETE":
		return OpDelete
	default:
		return OpOther // shouldn't be here
	}
}

// Method designates the type of operations for BeforeCRUPD and AfterCRUPD hookpoints
type Op int

const (
	OpOther Op = iota // should not be used
	OpRead
	OpCreate
	OpUpdate
	OpPatch
	OpDelete
)

type Cardinality int

const (
	CardinalityOne  Cardinality = 1
	CardinalityMany Cardinality = 2
)
