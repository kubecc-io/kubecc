package util

func Must(arg interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}
	return arg
}
