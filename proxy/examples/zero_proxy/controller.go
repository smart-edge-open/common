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

package main

import (
	"context"
	"fmt"
	"log"
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
				cc, err := grpc.Dial("", grpc.WithBlock(),
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

	// Listen
	lis, err := net.Listen("tcp", ":3333")
	if err != nil {
		log.Fatal(err)
	}
	defer lis.Close()
	pl := progutil.NewPrefaceListener(lis)

	// Print ephemeral port so example app can discover it by parsing logs
	_, port, _ := net.SplitHostPort(lis.Addr().String())
	fmt.Println("[cloud] Listening on port " + port)

	// Create gRPC server
	srv := grpc.NewServer()
	pb.RegisterHelloServiceServer(srv, new(helloService))

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
