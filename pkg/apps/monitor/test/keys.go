package test

//go:generate msgp
type TestKey1 struct {
	Counter int `msg:"counter"`
}

func (k TestKey1) Key() string {
	return "TestKey1"
}

type TestKey2 struct {
	Value string `msg:"value"`
}

func (k TestKey2) Key() string {
	return "TestKey2"
}
