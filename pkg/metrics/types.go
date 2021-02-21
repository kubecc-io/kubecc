package metrics

import (
	"github.com/tinylib/msgp/msgp"
)

type KeyedMetric interface {
	msgp.Decodable
	msgp.Encodable
	Key() string
}
