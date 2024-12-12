package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc/genericmap"
	"google.golang.org/protobuf/types/known/anypb"
)

func sendMock(ch chan struct{}, client genericmap.CollectorClient) {

	gm := config.GenericMap{
		"DstAddr":          fmt.Sprintf("10.0.0.%d", rand.Intn(255)),
		"DstPort":          rand.Intn(9999),
		"Proto":            rand.Intn(255),
		"SrcAddr":          fmt.Sprintf("10.0.0.%d", rand.Intn(255)),
		"SrcPort":          rand.Intn(9999),
		"SrcK8S_Name":      fmt.Sprintf("Pod-%d", rand.Intn(100)),
		"SrcK8S_Namespace": fmt.Sprintf("Namespace-%d", rand.Intn(100)),
		"SrcK8S_Type":      "Pod",
		"TimeFlowEndMs":    time.Now().UnixMilli(),
	}
	value, err := json.Marshal(gm)
	if err != nil {
		log.Error(err)
		return
	}
	_, err = client.Send(context.Background(), &genericmap.Flow{
		GenericMap: &anypb.Any{
			Value: value,
		},
	})
	if err != nil {
		log.Error(err)
		return
	}

	time.Sleep(10 * time.Millisecond)
	ch <- struct{}{}
}

func MockForever() {
	wait := make(chan struct{})

	for !collectorStarted {
		log.Info("Waiting for collector to start...")
		time.Sleep(1 * time.Second)
	}

	for _, port := range ports {
		cc, err := grpc.ConnectClient("127.0.0.1", port)
		if err != nil {
			log.Fatal(err)
		}
		for {
			go sendMock(wait, cc.Client())
			<-wait
		}
	}
}
