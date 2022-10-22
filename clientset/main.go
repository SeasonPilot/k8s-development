package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var (
		err        error
		config     *rest.Config
		kubeconfig *string
	)

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "")
	}
	// fixme
	flag.Parse()

	if config, err = rest.InClusterConfig(); err != nil {
		if config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig); err != nil {
			panic(err.Error())
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	deploys, err := clientset.AppsV1().Deployments("configmap-test").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Print(err.Error())
		return
	}

	for i, dep := range deploys.Items {
		fmt.Printf("%d  ->  %#v\n", i+1, dep.Name)
	}
}
