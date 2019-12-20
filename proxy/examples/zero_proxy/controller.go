// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2019 Intel Corporation

//+build ignore

package main

import (
	"context"
	"fmt"
	logger "github.com/open-ness/common/log"
	"log"
	"log/syslog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/open-ness/common/proxy/examples/zero_proxy/pb"
	"github.com/open-ness/common/proxy/progutil"
)

var lastCallSucceeded = false

func dialAppliance(id string, ctx context.Context, pl *progutil.PrefaceListener) {
	var cli pb.GoodbyeServiceClient
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
			if cli == nil {
				log.Print(id + ": Dial()'ling appliance")
				cc, err := grpc.Dial("127.0.0.1", grpc.WithBlock(),
					grpc.WithInsecure(), grpc.WithDialer(pl.DialEva))
				if err != nil {
					log.Printf(id+": [cloud] error dialing appliance: %v", err)
					continue
				}
				cli = pb.NewGoodbyeServiceClient(cc)
				log.Print(id + ": Dial()'ling success")
			}
			log.Print(id + ": Sending SayGoodbye RPC")
			goodbyeResp, err := cli.SayGoodbye(ctx, &wrappers.StringValue{Value: id + " Cloud"})
			if err != nil {
				log.Printf(id+": [cloud] error making RPC to appliance: %v", err)
				continue
			}
			log.Printf(id+": [cloud] got response from appliance: %s", goodbyeResp.GetValue())
			lastCallSucceeded = true
		}
	}
}

func main() {
	fmt.Println("[cloud] Running Cloud Controller")
	logger.SetLevel(syslog.LOG_DEBUG)

	// Listen
	lis, err := net.Listen("tcp", "127.0.0.1:3333")
	if err != nil {
		log.Fatal(err)
	}
	defer lis.Close()
	pl := progutil.NewPrefaceListener(lis)

	// Print ephemeral port so example app can discover it by parsing logs
	host, port, _ := net.SplitHostPort(lis.Addr().String())
	fmt.Println("[cloud] Listening on port " + port)

	// Create gRPC server
	srv := grpc.NewServer()
	pb.RegisterHelloServiceServer(srv, new(helloService))

	// Register our appliance into proxy
	pl.RegisterHost(host)

	// Dial appliance and call every second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = ctx
	go dialAppliance("goroutine1", ctx, pl)
	go dialAppliance("goroutine2", ctx, pl) // Attempt multiple parallel calls

	// Shutdown on interrupt
	intC := make(chan os.Signal, 1)
	signal.Notify(intC, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-intC
		cancel()
		srv.GracefulStop()
	}()

	// Run gRPC server
	srv.Serve(pl)
	if !lastCallSucceeded {
		fmt.Println("Controller: exiting with error")
		os.Exit(1)
	}
}

// Implements the HelloService
type helloService struct{}

func (s *helloService) SayHello(ctx context.Context, str *wrappers.StringValue) (*wrappers.StringValue, error) {
	if str.Value == "" {
		str.Value = "Nobody"
	}
	return &wrappers.StringValue{Value: "Hello " + str.Value}, nil
}
