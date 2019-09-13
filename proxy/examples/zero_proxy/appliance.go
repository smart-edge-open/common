// Copyright 2019 Smart-Edge.com, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//+build ignore

//go:generate protoc -I . -I $HOME/go/src --go_out=plugins=protobuf:. pb/test.proto

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/otcshare/common/proxy/examples/zero_proxy/pb"
	"github.com/otcshare/common/proxy/progutil"
	"google.golang.org/grpc"
)

func dialController(ctx context.Context, addr string) {
	var cli pb.HelloServiceClient
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
			if cli == nil {
				cc, err := grpc.Dial(addr, grpc.WithBlock(), grpc.WithInsecure())
				if err != nil {
					log.Printf("[edge] error dialing controller: %v", err)
					continue
				}
				cli = pb.NewHelloServiceClient(cc)
			}
			helloResp, err := cli.SayHello(ctx, &wrappers.StringValue{Value: "Edge"})
			if err != nil {
				log.Printf("[edge] error making RPC to controller: %v", err)
				continue
			}
			log.Printf("[edge] got response from controller: %s", helloResp.GetValue())
		}
	}
}

func main() {
	var port = flag.String("port", "", "port to connect to cloud proxy on")

	flag.Parse()
	if *port == "" {
		log.Fatal("[edge] Required flag 'port' was missing")
	}

	fmt.Println("[edge] Running Edge Appliance")

	// Connect to cloud controller
	cloudAddr, err := net.ResolveTCPAddr("tcp", "localhost:"+*port)
	if err != nil {
		log.Fatal("error resolving cloud addr: %v", err)
	}
	fmt.Println("[edge] Connecting to Cloud Controller at " + cloudAddr.String())

	// Create gRPC server
	srv := grpc.NewServer()
	pb.RegisterGoodbyeServiceServer(srv, new(goodbyeService))

	// Dial controller and call every second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = ctx
	go dialController(ctx, cloudAddr.String())

	// Shutdown on interrupt
	intC := make(chan os.Signal, 1)
	signal.Notify(intC, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-intC
		go func() {
			time.Sleep(3 * time.Second)
			log.Printf("Shutdown timeout: forcibly terminating the appliance")
			os.Exit(2)
		}()
		cancel()
		srv.GracefulStop()
	}()

	// Run gRPC server
	lis := &progutil.DialListener{RemoteAddr: cloudAddr, Name: "EVA"}
	defer lis.Close()
	srv.Serve(lis)
}

// Implements the GoodbyeService
type goodbyeService struct{}

func (s *goodbyeService) SayGoodbye(ctx context.Context, str *wrappers.StringValue) (*wrappers.StringValue, error) {
	if str.Value == "" {
		str.Value = "Nobody"
	}
	return &wrappers.StringValue{Value: "Goodbye " + str.Value}, nil
}
