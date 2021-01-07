package main

type StringSet struct {
	data map[string]struct{}
}

func NewStringSet(args ...string) *StringSet {
	set := &StringSet{data: make(map[string]struct{})}
	for _, arg := range args {
		set.data[arg] = struct{}{}
	}
	return set
}

func (s *StringSet) Contains(arg string) (contains bool) {
	_, contains = s.data[arg]
	return
}

func (s *StringSet) Add(arg string) {
	s.data[arg] = struct{}{}
}
