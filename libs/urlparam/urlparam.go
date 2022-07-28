package urlparam

import "strconv"

// Param is the URL parameter
type Param string

const (
	ParamOffset        Param = "offset"
	ParamLimit         Param = "limit"
	ParamOrder         Param = "order"
	ParamOrderBy       Param = "orderby"
	ParamLatestN       Param = "latestn"
	ParamLatestNOn     Param = "latestnon"
	ParamCstart        Param = "cstart"
	ParamCstop         Param = "cstop"
	ParamHasTotalCount Param = "totalcount"
	ParamOtherQueries  Param = "better_otherqueries"
)

func GetOptions(options map[Param]interface{}) (offset *int, limit *int, cstart *int, cstop *int, orderby *string, order *string, latestn *int, latestnons []string, count bool) {
	// If key is in it, even if value is nil, ok will be true

	if _, ok := options[ParamOffset]; ok {
		v := options[ParamOffset].(int)
		offset = &v
	}

	if _, ok := options[ParamLimit]; ok {
		v := options[ParamLimit].(int)
		limit = &v
	}

	if _, ok := options[ParamOrderBy]; ok {
		v := options[ParamOrderBy].(string)
		orderby = &v
	}

	if _, ok := options[ParamOrder]; ok {
		v := options[ParamOrder].(string)
		order = &v
	}

	if _, ok := options[ParamCstart]; ok {
		v := options[ParamCstart].(int)
		cstart = &v
	}
	if _, ok := options[ParamCstop]; ok {
		v := options[ParamCstop].(int)
		cstop = &v
	}

	latestn = nil
	if n, ok := options[ParamLatestN]; ok {
		if n != nil {
			if n2, err := strconv.Atoi(n.(string)); err == nil {
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

	return offset, limit, cstart, cstop, orderby, order, latestn, latestnons, hasTotalCount
}
