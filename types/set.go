package types

// StringSet is a simple hash set for strings.
type StringSet struct {
	data map[string]struct{}
}

// NewStringSet creates a new StringSet from the given strings.
func NewStringSet(args ...string) *StringSet {
	set := &StringSet{data: make(map[string]struct{})}
	for _, arg := range args {
		set.data[arg] = struct{}{}
	}
	return set
}

// Contains returns true if the given string is contained
// in the set, otherwise false.
func (s *StringSet) Contains(arg string) (contains bool) {
	_, contains = s.data[arg]
	return
}

// Add adds a string into the set.
func (s *StringSet) Add(arg string) {
	s.data[arg] = struct{}{}
}
