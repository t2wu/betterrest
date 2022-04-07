package urlparam

import "strconv"

// Param is the URL parameter
type Param string

const (
	ParamOffset        Param = "offset"
	ParamLimit         Param = "limit"
	ParamOrder         Param = "order"
	ParamLatestN       Param = "latestn"
	ParamLatestNOn     Param = "latestnon"
	ParamCstart        Param = "cstart"
	ParamCstop         Param = "cstop"
	ParamHasTotalCount Param = "totalcount"
	ParamOtherQueries  Param = "better_otherqueries"
)

func GetOptions(options map[Param]interface{}) (offset *int, limit *int, cstart *int, cstop *int, order *string, latestn *int, latestnons []string, count bool) {
	// If key is in it, even if value is nil, ok will be true

	if _, ok := options[ParamOffset]; ok {
		offset, _ = options[ParamOffset].(*int)
	}

	if _, ok := options[ParamLimit]; ok {
		limit, _ = options[ParamLimit].(*int)
	}

	if _, ok := options[ParamOrder]; ok {
		order, _ = options[ParamOrder].(*string)
	}

	if _, ok := options[ParamCstart]; ok {
		cstart, _ = options[ParamCstart].(*int)
	}
	if _, ok := options[ParamCstop]; ok {
		cstop, _ = options[ParamCstop].(*int)
	}

	latestn = nil
	if n, ok := options[ParamLatestN]; ok {
		if n != nil {
			if n2, err := strconv.Atoi(*(n.(*string))); err == nil {
				latestn = &n2
			}
		}
	}

	if _, ok := options[ParamLatestNOn]; ok {
		latestnons = options[ParamLatestNOn].([]string)
	}

	hasTotalCount := false
	if _, ok := options[ParamHasTotalCount]; ok {
		hasTotalCount = options[ParamHasTotalCount].(bool)
	}

	return offset, limit, cstart, cstop, order, latestn, latestnons, hasTotalCount
}
