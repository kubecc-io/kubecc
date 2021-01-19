package rec

import (
	"reflect"

	"github.com/cobalt77/kubecc/internal/lll"
)

func Equal(current interface{}, desired interface{}) bool {
	return reflect.DeepEqual(current, desired)
}

func Diff(current interface{}, desired interface{}) []reflect.StructField {
	cur := reflect.ValueOf(current)
	des := reflect.ValueOf(desired)
	if cur.Type() != des.Type() {
		lll.DPanic("Tried to diff two different types")
	}
	fields := []reflect.StructField{}
	for i := 0; i < cur.NumField(); i++ {
		cf := cur.Field(i)
		df := des.Field(i)
		if !reflect.DeepEqual(cf, df) {
			fields = append(fields, cur.Type().Field(i))
		}
	}
	return fields
}
