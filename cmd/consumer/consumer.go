package main

import (
	"fmt"
	"net"

	"github.com/spf13/viper"
)

func runConsumer() {
	port := viper.GetInt("port")
	// Test if the connection is open
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))

	if err == nil {
		listener.Close()
		becomeLeader() // Does not return
	}

	runFollower()
}
