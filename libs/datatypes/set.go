package datatypes

// https://gist.github.com/bgadrian/cb8b9344d9c66571ef331a14eb7a2e80

type SetString struct {
	List map[string]struct{} //empty structs occupy 0 memory
}

func (s *SetString) Has(v string) bool {
	_, ok := s.List[v]
	return ok
}

func (s *SetString) Add(v string) {
	s.List[v] = struct{}{}
}

func (s *SetString) Remove(v string) {
	delete(s.List, v)
}

func (s *SetString) Clear() {
	s.List = make(map[string]struct{})
}

func (s *SetString) Size() int {
	return len(s.List)
}

func NewSetString() *SetString {
	s := &SetString{}
	s.List = make(map[string]struct{})
	return s
}

//optional functionalities

//AddMulti Add multiple values in the set
func (s *SetString) AddMulti(List ...string) {
	for _, v := range List {
		s.Add(v)
	}
}

type FilterFunc func(v string) bool

// Filter returns a subset, that contains only the values that satisfies the given predicate P
func (s *SetString) Filter(P FilterFunc) *SetString {
	res := NewSetString()
	for v := range s.List {
		if P(v) == false {
			continue
		}
		res.Add(v)
	}
	return res
}

func (s *SetString) Union(s2 *SetString) *SetString {
	res := NewSetString()
	for v := range s.List {
		res.Add(v)
	}

	for v := range s2.List {
		res.Add(v)
	}
	return res
}

func (s *SetString) Intersect(s2 *SetString) *SetString {
	res := NewSetString()
	for v := range s.List {
		if s2.Has(v) == false {
			continue
		}
		res.Add(v)
	}
	return res
}

// Difference returns the subset from s, that doesn't exists in s2 (param)
func (s *SetString) Difference(s2 *SetString) *SetString {
	res := NewSetString()
	for v := range s.List {
		if s2.Has(v) {
			continue
		}
		res.Add(v)
	}
	return res
}
