package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"time"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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

	// 和 clientset 差异点
	informerFactory := informers.NewSharedInformerFactory(clientset, time.Second*30)
	deployInformer := informerFactory.Apps().V1().Deployments()
	informer := deployInformer.Informer()
	deployLister := deployInformer.Lister()

	informer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			deploy := obj.(*v1.Deployment)
			fmt.Println("add", deploy.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeploy := oldObj.(*v1.Deployment)
			newDeploy := newObj.(*v1.Deployment)
			fmt.Println("update", oldDeploy.Name)
			fmt.Println("update", newDeploy.Name)
		},
		DeleteFunc: func(obj interface{}) {
			deploy := obj.(*v1.Deployment)
			fmt.Println("dle", deploy.Name)
		},
	})

	stopChan := make(chan struct{})
	defer close(stopChan)

	informerFactory.Start(stopChan)

	informerFactory.WaitForCacheSync(stopChan)

	deploys, err := deployLister.Deployments("configmap-test").List(labels.Everything())
	if err != nil {
		return
	}

	//
	for i, dep := range deploys {
		fmt.Printf("%d  ->  %#v\n", i+1, dep.Name)
	}

	// fixme
	<-stopChan
}
