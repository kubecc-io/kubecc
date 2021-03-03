package util

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

func NewJitteredTimer(duration time.Duration, factor float64) <-chan struct{} {
	ch := make(chan struct{})
	go wait.JitterUntil(func() {
		ch <- struct{}{}
	}, duration, factor, false, wait.NeverStop)
	return ch
}
