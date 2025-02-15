package main

import (
	"log"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/adapter"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	evadapter "knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/signals"
)

const (
	PACControllerLogKey = "pipelinesascode"
)

func main() {
	ctx := signals.NewContext()

	run := params.New()
	err := run.Clients.NewClients(ctx, &run.Info)
	if err != nil {
		log.Fatal("failed to init clients : ", err)
	}

	kinteract, err := kubeinteraction.NewKubernetesInteraction(run)
	if err != nil {
		log.Fatal("failed to init kinit client : ", err)
	}

	c := make(chan struct{})
	go func() {
		log.Println("started goroutine to watch configmap changes for controller")
		c <- struct{}{}
		if err := run.WatchConfigMapChanges(ctx); err != nil {
			log.Fatal("error from WatchConfigMapChanges for controller : ", err)
		}
	}()
	// Force WatchConfigMapChanges go routines to actually start
	<-c

	evadapter.MainWithContext(ctx, PACControllerLogKey, adapter.NewEnvConfig, adapter.New(run, kinteract))
}
