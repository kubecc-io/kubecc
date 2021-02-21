package metrics

import "reflect"

type KeyedMetric interface {
	Key() string
	Type() reflect.Type
}
