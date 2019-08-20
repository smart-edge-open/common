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
	"github.com/otcshare/common/proxy/examples/zero_proxy/pb"
	"github.com/otcshare/common/proxy/progutil"
)

func main() {
	fmt.Println("[cloud] Running Cloud Controller")

	// Listen on an ephemeral port
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal(err)
	}
	defer lis.Close()
	pl := &progutil.PrefaceListener{Listener: lis}

	// Print ephemeral port so example app can discover it by parsing logs
	_, port, _ := net.SplitHostPort(lis.Addr().String())
	fmt.Println("[cloud] Listening on port " + port)

	// Create gRPC server
	srv := grpc.NewServer()
	pb.RegisterHelloServiceServer(srv, new(helloService))

	// Dial appliance and call every second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lastCallSucceeded := false
	go func() {
		var cli pb.GoodbyeServiceClient
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				if cli == nil {
					cc, err := grpc.Dial("", grpc.WithBlock(),
						grpc.WithInsecure(), grpc.WithDialer(pl.Dial))
					if err != nil {
						log.Printf("[cloud] error dialing appliance: %v", err)
						continue
					}
					cli = pb.NewGoodbyeServiceClient(cc)
				}
				goodbyeResp, err := cli.SayGoodbye(ctx, &wrappers.StringValue{Value: "Cloud"})
				if err != nil {
					log.Printf("[cloud] error making RPC to appliance: %v", err)
					continue
				}
				log.Printf("[cloud] got response from appliance: %s", goodbyeResp.GetValue())
				lastCallSucceeded = true

			}
		}
	}()

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
