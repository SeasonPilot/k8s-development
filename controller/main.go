package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// 第一步
func initClient() (*kubernetes.Clientset, error) {
	var (
		config     *rest.Config
		err        error
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
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	}

	// fixme
	// clientSet, err := rest.RESTClientFor(config)
	return kubernetes.NewForConfig(config)
}

// 第二步
type Controller struct { // 自定义控制器里面有哪些组件
	informer cache.Controller // 命名为 informer
	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
}

// 第三步
func NewController(informer cache.Controller, indexer cache.Indexer, queue workqueue.RateLimitingInterface) *Controller {
	return &Controller{
		informer: informer,
		indexer:  indexer,
		queue:    queue,
	}
}

// 第四步 processNextItem
func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	// 停止控制器后关掉队列
	defer c.queue.ShutDown()
	klog.Info("Starting Pod controller")

	// 1. 启动通用控制器框架
	go c.informer.Run(stopCh)

	// 2. 等待缓存同步 // 等待所有相关的缓存同步，然后再开始处理队列中的项目
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) { // fixme: 缺少第二个参数
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return // fixme:  这里要 return
	}

	// c.informer.  只是接口不是实例化对象无法调用 AddEventHandler   Informer 架构说明 第 6）步

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh // 通道关闭后，读到零值，解除阻塞
	klog.Info("Stopping Pod controller")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

// 第七步 syncToStdout 是控制器的业务逻辑实现
// 此外重试逻辑不应成为业务逻辑的一部分。
func (c *Controller) syncToStdout(key string) error {
	// Handle Object。 从 indexer本地存储 中取出对象
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}
	if !exists {
		klog.Error("对象不存在")
		// return errors.New("对象不存在") // return 代表需要重试 5 次
	} else {
		fmt.Printf("%#v\n", obj.(*v1.Pod).GetName())
	}

	return nil
}

// 第六步
func (c *Controller) processNextItem() bool {
	// 从 workqueue 中取出的是 key。 要不断的取出,所以放在 for 循环内
	key, quit := c.queue.Get()
	if quit {
		klog.Error("queue is quit")
		return false
	}

	// fixme: 当处理完一个元素后，必须调用 Done() 函数
	defer c.queue.Done(key)

	// 业务逻辑 Handle Object
	err := c.syncToStdout(key.(string))
	c.handleErr(key.(string), err)

	return true
}

// 第八步 检查是否发生错误，并确保我们稍后重试
func (c *Controller) handleErr(key interface{}, err error) { // fixme: key 的类型应该是 interface
	if err == nil {
		// 忘记每次成功同步时密钥的 AddRateLimited 历史记录。这可确保将来对此密钥的更新处理不会因为过时的错误历史记录而延迟。
		c.queue.Forget(key)
		return // fixme: 忘记 return
	}

	// 重试逻辑。 如果出现问题，此控制器将重试5次
	if c.queue.NumRequeues(key) < 5 {
		klog.Infof("Error syncing pod %v: %v", key, err)
		// 重新加入 key 到限速队列
		// 根据队列上的速率限制器和重新入队历史记录，稍后将再次处理该 key
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	klog.Error("重试 5 次后仍然失败")
	runtime.HandleError(err)
}

func main() {
	clientset, err := initClient()
	if err != nil {
		klog.Fatal(err)
	}

	// 第五步 接2. 等待缓存同步。
	// 实例化 informer, 实际是 cache.controller，目的是用来把 Reflector、DeltaFIFO 这些组件组合起来形成一个相对固定的、标准的处理流程。

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	podListWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "pods", v1.NamespaceDefault, fields.Everything())
	indexer, informer := cache.NewIndexerInformer(podListWatcher, &v1.Pod{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// fixme: !!! 不应该调用 cache.MetaNamespaceIndexFunc(obj)， 应该调用 cache.MetaNamespaceKeyFunc(obj)
			//   key, err := cache.MetaNamespaceIndexFunc(obj) // IndexFunc  用于计算一个对象的 索引键 集合  ["default"]
			// 这里是要将 对象键 放入 工作队列 workqueue
			key, err := cache.MetaNamespaceKeyFunc(obj) //  "default/ping-dest-6c5bb69c88-zmx4c" 计算资源 对象键 的函数
			if err == nil {
				queue.Add(key) // workqueue 中添加的是 key
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err != nil { // todo: 这里获取对象键错误，需不需要 panic ？
				panic(err.Error())
			}
			queue.Add(key)
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err != nil {
				panic(err.Error())
			}
			queue.Add(key)
		},
	}, cache.Indexers{})

	controller := NewController(informer, indexer, queue)

	indexer.Add(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mypod",
			Namespace: v1.NamespaceDefault,
		},
	})

	// start controller
	// fixme stopCh要在 main 中初始化。使用协程启动controller.Run()，并传入stopCh
	stopCh := make(chan struct{}) // 哪里写入值的？谁来写入值？ // stopCh监听是什么信号？ 作用是啥？
	defer close(stopCh)
	go controller.Run(1, stopCh)

	select {}
}
