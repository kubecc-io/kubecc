package run

// A Resizer is an object that is able to dynamically resize its
// contained resources.
type Resizer interface {
	// Resize should set the target number of contained resources to the given
	// value. It should block until the resize operation is complete.
	Resize(int64)
}
