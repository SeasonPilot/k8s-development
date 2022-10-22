package main

import (
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	NamespaceIndexName = "namespaceIndexFunc"
	NodeNameIndexName  = "nodeName"
)

func namespaceIndexFunc(obj interface{}) ([]string, error) {
	pods, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	return []string{pods.GetNamespace()}, nil
}
func nodeNameIndexFunc(obj interface{}) ([]string, error) {
	pod, ok := obj.(*v1.Pod) // 指针 fixme: indexer.Add(pod1) 传入的 pod 类型是指针
	if !ok {
		return nil, errors.New("断言失败，类型不对")
	}

	return []string{pod.Spec.NodeName}, nil
}

func main() {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, map[string]cache.IndexFunc{
		NamespaceIndexName: namespaceIndexFunc,
		NodeNameIndexName:  nodeNameIndexFunc,
	})

	pod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			NodeName: "node1",
		},
	}

	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-2",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			NodeName: "node2",
		},
	}

	pod3 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-3",
			Namespace: "kube-system",
		},
		Spec: v1.PodSpec{
			NodeName: "node2",
		},
	}

	indexer.Add(pod1)
	indexer.Add(pod2)
	indexer.Add(pod3)

	// pods, err := indexer.ByIndex(NamespaceIndexName, "default")
	// if err != nil {
	// 	return
	// }

	pods, err := indexer.ByIndex(NodeNameIndexName, "node2")
	if err != nil {
		return
	}

	for _, pod := range pods {
		p, ok := pod.(*v1.Pod) // fixme: indexer.Add(pod1) 传入的 pod 类型是指针
		if !ok {
			fmt.Println("断言失败")
			return
		}
		fmt.Println(p.Name)
	}
}
