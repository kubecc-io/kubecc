package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/cobalt77/kube-cc/mgr/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	var (
		agents string
	)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		var conn *grpc.ClientConn
		conn, err := grpc.Dial("---:443",
			grpc.WithTransportCredentials(credentials.NewTLS(
				&tls.Config{
					InsecureSkipVerify: false,
				},
			)))
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		client := api.NewApiClient(conn)
		status, err := client.Status(context.Background(), &api.StatusRequest{
			Command: strings.Join(os.Args[1:], " "),
		})

		if err != nil {
			log.Fatal(err)
		}
		agentList := []string{}

		for _, a := range status.Agents {
			agentList = append(agentList,
				fmt.Sprintf("%s:%d", a.Address, a.Port))
		}
		agents = strings.Join(agentList, " ")
	}()

	// go func() {
	// 	defer wg.Done()

	// 	f, err := openDistcc()
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	defer f.Close()
	// 	mfd, err := memfd.Create()

	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	b, err := mfd.Map()
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	buf := bytes.NewBuffer(b)
	// 	_, err = io.Copy(buf, f)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	_, err = mfd.Write(buf.Bytes())
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	fd = mfd.Fd()
	// }()

	wg.Wait()
	distcc, err := exec.LookPath("distcc")
	if err != nil {
		log.Fatal(err)
	}
	syscall.Exec(distcc,
		append([]string{"distcc"}, os.Args[1:]...),
		append(os.Environ(),
			fmt.Sprintf("DISTCC_HOSTS=%s", agents)))
	// syscall.Exec(
	// 	fmt.Sprintf("/proc/self/fd/%d", fd),
	// 	append([]string{"distcc"}, os.Args[1:]...),
	// 	append(os.Environ(),
	// 		fmt.Sprintf("DISTCC_HOSTS=localhost %s", agents)))
}
