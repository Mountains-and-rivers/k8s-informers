# informers机制分析

## 示例

```
package main

import (
	"context"
	"flag"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"time"
)

func main() {
	//k8s连接测试
	 //Con_k8s()

	//informer 测试
	Inform_k8s()

}

func Inform_k8s() {
	var err error
	var config *rest.Config
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	flag.Parse()

	// 创建rest.config 对象
	if config, err = rest.InClusterConfig(); err != nil {
		if config, err = clientcmd.BuildConfigFromFlags("", * kubeconfig); err != nil {
			panic(err.Error())
		}
	}
	// 创建clientset 对象
	clientset, err := kubernetes.NewForConfig(config)
	if err == nil {
		if err != nil {
			panic(err.Error())
		}
	}

	//informers.WithNamespace(namespace),
	//	informers.WithTweakListOptions(func(*metav1.ListOptions) {}))
	// 初始化 informers factory
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, time.Second*30, informers.WithTweakListOptions(func(options *metav1.ListOptions) {
	}))
	// 步骤2，使用工厂创建指定API对象的Informer，必须调用到Informer()方法，该方法会回调到工厂方法的InformerFor
	informer:= factory.Core().V1().Pods().Informer()
	deployLister := deployInformer.Lister()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			deploy := obj.(*v1.Pod)
			// 业务逻辑
			fmt.Println("add a deploy:", deploy.Name,time.Now().Format("2006-01-02 15:04:05"))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeploy := oldObj.(*v1.Pod)
			newDeploy := newObj.(*v1.Pod)
			fmt.Println("update a deploy:", oldDeploy.Name, newDeploy.Name,time.Now().Format("2006-01-02 15:04:05"))
		},
		DeleteFunc: func(obj interface{}) {
			deploy := obj.(*v1.Pod)
			fmt.Println("delete a deploy:", deploy.Name)
		},
	})
	stopChan := make(chan struct{})
	// 步骤3，使用factory启动informer,该方法是异步，里面会遍历所有的Informer，并调用informer的Run方法，Run方法核心功能就是开启资源对象的本地缓存
	factory.Start(stopChan)

	// 步骤4，等待该工厂里的所有启动过的informer完成数据缓存到本地
	factory.WaitForCacheSync(stopChan)
	if err != err {
		panic(err.Error())
	}
	StatefulSet, err := deployLister.Pods("default").List(labels.Everything())
	if err != err {
		panic(err.Error())
	}
	for idx, deploy := range StatefulSet {
		fmt.Printf("%d -->> %s \n", idx+1, deploy.Name)
	}
	<-stopChan
}

func Con_k8s() {
	var err error
	var config *rest.Config
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	flag.Parse()

	// 创建rest.config 对象
	if config, err = rest.InClusterConfig(); err != nil {
		if config, err = clientcmd.BuildConfigFromFlags("", * kubeconfig); err != nil {
			panic(err.Error())
		}
	}
	// 创建clientset 对象
	clientset, err := kubernetes.NewForConfig(config)
	if err == nil {
		StatefulSets, err := clientset.AppsV1().StatefulSets("metallb-system").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}

		for idx, deploy := range StatefulSets.Items {
			fmt.Printf("%d -> %s\n", idx+1, deploy.Name)
		}

	} else {
		panic(err.Error())
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE")
}

```

# 解析

##### 简介

- Informer：
  - 同步本地缓存，把 API 资源对象缓存一份到本地
  - 根据发生的事件类型，触发事先注册好的控制器回调
- Lister：从本地缓存中获取 API 资源对象

##### 创建

工厂创建

每一种 API 资源对象都会有对应的 Informer，由工厂 k8s.io/client-go/informers.sharedInformerFactory 负责创建，工厂的创建如下：

- client：与 kube-apiserver 交互的客户端实现
- defaultResync：[minResyncPeriod, 2*minResyncPeriod) 范围的一个随机值，用于设置 reflectors 重新同步的周期，默认 12h
- v1.NamespaceAll：空字符串
- sharedInformerFactory：
  - informers：存储创建好的 API 资源对象对应的 Informer
  - startedInformers：已启动的 Informer
  - customResync：为特定类型的 API 资源对象指定同步周期，目前未用到
  - tweakListOptions：设置请求参数

```
// NewFilteredSharedInformerFactory constructs a new instance of sharedInformerFactory.
// Listers obtained via this SharedInformerFactory will be subject to the same filters
// as specified here.
// Deprecated: Please use NewSharedInformerFactoryWithOptions instead
func NewFilteredSharedInformerFactory(client kubernetes.Interface, defaultResync time.Duration, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) SharedInformerFactory {
	return NewSharedInformerFactoryWithOptions(client, defaultResync, WithNamespace(namespace), WithTweakListOptions(tweakListOptions))
}

// NewSharedInformerFactoryWithOptions constructs a new instance of a SharedInformerFactory with additional options.
func NewSharedInformerFactoryWithOptions(client kubernetes.Interface, defaultResync time.Duration, options ...SharedInformerOption) SharedInformerFactory {
	factory := &sharedInformerFactory{
		client:           client,
		namespace:        v1.NamespaceAll,
		defaultResync:    defaultResync,
		informers:        make(map[reflect.Type]cache.SharedIndexInformer),
		startedInformers: make(map[reflect.Type]bool),
		customResync:     make(map[reflect.Type]time.Duration),
	}
	// Apply all options

	for _, opt := range options {
		factory = opt(factory)
	}
	return factory
}
```

##### API 资源对象对应的 Informer 的创建

以 sharedInformerFactory.core().V1().Pod() 为例，创建的是 k8s.io/client-go/informers/core/v1.NewPodInformer，调用链路如下：

```
// k8s.io\client-go\informers\factory.go
func (f *sharedInformerFactory) Core() core.Interface {
	return core.New(f, f.namespace, f.tweakListOptions)
}
//k8s.io\client-go\informers\core\interface.go

func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &group{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

func (g *group) V1() v1.Interface {
	return v1.New(g.factory, g.namespace, g.tweakListOptions)
}

//k8s.io\client-go\informers\core\v1\interface.go
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

func (v *version) ComponentStatuses() ComponentStatusInformer {
	return &componentStatusInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}
```

#####  k8s.io/client-go/informers/core/v1.NewPodInforme 解析

```
// PodInformer provides access to a shared informer and lister for
// Pods.
type PodInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.PodLister
}

type podInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}
```

- podInformer 实现了 PodInformer
  - factory：工厂，即 k8s.io/client-go/informers.sharedInformerFactory
  - tweakListOptions：sharedInformerFactory.tweakListOptions
  - namespace：命名空间，与 sharedInformerFactory.namespace 保持一致

##### Informer 的创建：

- Informer：调用 sharedInformerFactory.InformerFor
  - &corev1.Pod{}：声明使用API资源对象类型
  - podInformer.defaultInformer: 创建方法
  - 在 sharedInformerFactory.informers 中判断是否已存在，如果存在则返回
  - 如果不存在，调用podInformer.defaultInformer(client kubernetes.Interface, resyncPeriod time.Duration) 创建并存储到 sharedInformerFactory.informers
    - resyncPeriod：如果 sharedInformerFactory.customResync[reflect.TypeOf(&corev1.Pod{})] 存在，则为该值，否则为 sharedInformerFactory.defaultResync
- defaultInformer：
  - cache.Indexers：underlying type 为 map[string]IndexFunc
    - cache.NamespaceIndex：字符串 namespace 字面量
    - cache.MetaNamespaceIndexFunc：返回 API 资源对象的命名空间
- cache.NewSharedIndexInformer：创建 k8s.io/client-go/tools/cache.sharedIndexInformer
  - cache.ListWatch：
    - ListFunc：获取对应的 API 资源对象列表
    - WatchFunc：监听对应的 API 资源对象变更

```
func (f *podInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&corev1.Pod{}, f.defaultInformer)
}
func (f *podInformer) defaultInformer(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredPodInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

// NewFilteredPodInformer constructs a new informer for Pod type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredPodInformer(client kubernetes.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1().Pods(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1().Pods(namespace).Watch(context.TODO(), options)
			},
		},
		&corev1.Pod{},
		resyncPeriod,
		indexers,
	)
}
```

##### Lister 的创建：

- k8s.io/client-go/informers/core/v1.NewPodInformer：返回 podLister，从 podLister从获取 API 资源对象会转换为对 sharedIndexInformer.indexer 的调用
  - podLister.indexer：赋值为 sharedIndexInformer.indexer，实现类型为 k8s.io/client-go/tools/cache.cache

```
func (f *podInformer) Lister() v1.PodLister {
	return v1.NewPodLister(f.Informer().GetIndexer())
}
// podLister implements the PodLister interface.
type podLister struct {
	indexer cache.Indexer
}
// NewPodLister returns a new PodLister.
func NewPodLister(indexer cache.Indexer) PodLister {
	return &podLister{indexer: indexer}
}
// NewPodLister returns a new PodLister.
func NewPodLister(indexer cache.Indexer) PodLister {
	return &podLister{indexer: indexer}
}
//vendor\k8s.io\client-go\tools\cache\shared_informer.go
func (s *sharedIndexInformer) GetIndexer() Indexer {
	return s.indexer
}
```

##### k8s.ioo/client-go/tools/cache.sharedIndexInformer 解析

- indexer：k8s.io/client-go/tools/cache.cache，通过应用 DeltaFIFO.Pop 出来的事件更新缓存，会在 sharedIndexInformer.HandleDeltas 中更新（Update/Add/Delete），用于 Lister 获取对象，作为持久化数据的一份缓存
  - cache.keyFunc：DeletionHandlingMetaNamespaceKeyFunc，检查 API 资源对象是否为 DeletedFinalStateUnknown
    - 如果是，返回 DeletedFinalStateUnknown.Key
    - 否则返回 MetaNamespaceKeyFunc(obj)，格式为 <namespace>/<name>，如果 <namespace> 为空，则返回 <name>
  - cache.cacheStorage：k8s.io/client-go/tools/cache.threadSafeMap
    - threadSafeMap.indexers：k8s.io/client-go/tools/cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
    - threadSafeMap.indices：k8s.io/client-go/tools/cache.Indices，其 underlying type 为 map[string]Index
      - Index：map[string]sets.string，namespace => {name1, name2}

- controller：k8s.io/client-go/tools/cache.controller 负责与 kube-apiserver 交互
- processor：k8s.io/client-go/tools/cache.sharedProcessor 负责事件回调的注册和触发
- cacheMutationDetector：默认为 cache.dummyMutationDetector
  - k8s.io/client-go/tools/cache.dummyMutationDetector：空实现
  - k8s.io/client-go/tools/cache.defaultCacheMutationDetector：周期性检查缓存中的 API 资源对象是否发生了修改，如果发生修改，会触发 defaultCacheMutationDetector.failureFunc 或 panic
- listerWatcher：k8s.io/client-go/tools/cache.ListWatch
- objectType：API 资源类型，例如 &corev1.Pod{}
- resyncCheckPeriod：等于 defaultEventHandlerResyncPeriod
- defaultEventHandlerResyncPeriod：sharedInformerFactory.defaultResync
- clock：k8s.io/apimachinery/pkg/util/clock.RealClock
- started：是否启动了
- stopped：是否停止了
- startedLock：用于访问 started 和 stopped 时的互斥锁
- blockDeltas：停止事件分发，确保启动后新增的事件处理回调能够安全的加入

```
type sharedIndexInformer struct {
	indexer    Indexer
	controller Controller

	processor             *sharedProcessor
	cacheMutationDetector MutationDetector

	listerWatcher ListerWatcher

	// objectType is an example object of the type this informer is
	// expected to handle.  Only the type needs to be right, except
	// that when that is `unstructured.Unstructured` the object's
	// `"apiVersion"` and `"kind"` must also be right.
	objectType runtime.Object

	// resyncCheckPeriod is how often we want the reflector's resync timer to fire so it can call
	// shouldResync to check if any of our listeners need a resync.
	resyncCheckPeriod time.Duration
	// defaultEventHandlerResyncPeriod is the default resync period for any handlers added via
	// AddEventHandler (i.e. they don't specify one and just want to use the shared informer's default
	// value).
	defaultEventHandlerResyncPeriod time.Duration
	// clock allows for testability
	clock clock.Clock

	started, stopped bool
	startedLock      sync.Mutex

	// blockDeltas gives a way to stop all event distribution so that a late event handler
	// can safely join the shared informer.
	blockDeltas sync.Mutex

	// Called whenever the ListAndWatch drops the connection with an error.
	watchErrorHandler WatchErrorHandler
}
```

##### k8s.io/client-go/tools/cache.controller

- controller
  - config：配置相关
    - Queue：k8s.io/client-go/tools/cache.DeltaFIFO
    - ListerWatcher：k8s.io/client-go/tools/cache.ListWatch
    - Process：sharedIndexInformer.HandleDeltas
    - ObjectType：sharedIndexInformer.objectType
    - FullResyncPeriod：sharedIndexInformer.resyncCheckPeriod
    - ShouldResync：sharedProcessor.shouldResync 遍历每个 listener，调用 listener.shouldResync(now) 判断是否需要重新同步
      - processorListener.shouldResync：p.resyncPeriod 必须大于 0，判断是否满足 now.After(p.nextResync) || now.Equal(p.nextResync)
    - RetryOnError：当 Process 返回错误的时候，是否重新入队 obj
  - reflector：k8s.io/client-go/tools/cache.Reflector
    - name：调用者的信息 shortpath/filename:line
    - metrics：目前未使用
    - expectedType：API 资源类型
    - store：k8s.io/client-go/tools/cache.DeltaFIFO
    - listerWatcher：k8s.io/client-go/tools/cache.ListWatch
    - period：time.Second Reflector.ListAndWatch 执行周期
    - resyncPeriod：sharedIndexInformer.resyncCheckPeriod
    - ShouldResync：sharedProcessor.shouldResync 遍历每个 listener，调用 listener.shouldResync(now) 判断是否需要重新同步
    - clock：k8s.io/apimachinery/pkg/util/clock.RealClock
    - lastSyncResourceVersion：同步到的 API 资源对象最新版本号
    - lastSyncResourceVersionMutex：保护 lastSyncResourceVersion 的访问
    - WatchListPageSize：初始化时分页获取对象列表的大小
  - reflectorMutex：用于对 reflector 进行赋值时的互斥访问（目前从代码看感觉不需要）
  - clock：k8s.io/apimachinery/pkg/util/clock.RealClock

```
//controller implements Controller
type controller struct {
	config         Config
	reflector      *Reflector
	reflectorMutex sync.RWMutex
	clock          clock.Clock
}
// Config contains all the settings for one of these low-level controllers.
type Config struct {
	// The queue for your objects - has to be a DeltaFIFO due to
	// assumptions in the implementation. Your Process() function
	// should accept the output of this Queue's Pop() method.
	Queue

	// Something that can list and watch your objects.
	ListerWatcher

	// Something that can process a popped Deltas.
	Process ProcessFunc

	// ObjectType is an example object of the type this controller is
	// expected to handle.  Only the type needs to be right, except
	// that when that is `unstructured.Unstructured` the object's
	// `"apiVersion"` and `"kind"` must also be right.
	ObjectType runtime.Object

	// FullResyncPeriod is the period at which ShouldResync is considered.
	FullResyncPeriod time.Duration

	// ShouldResync is periodically used by the reflector to determine
	// whether to Resync the Queue. If ShouldResync is `nil` or
	// returns true, it means the reflector should proceed with the
	// resync.
	ShouldResync ShouldResyncFunc

	// If true, when Process() returns an error, re-enqueue the object.
	// TODO: add interface to let you inject a delay/backoff or drop
	//       the object completely if desired. Pass the object in
	//       question to this interface as a parameter.  This is probably moot
	//       now that this functionality appears at a higher level.
	RetryOnError bool

	// Called whenever the ListAndWatch drops the connection with an error.
	WatchErrorHandler WatchErrorHandler

	// WatchListPageSize is the requested chunk size of initial and relist watch lists.
	WatchListPageSize int64
}
```

##### k8s.io/client-go/tools/cache.DeltaFIFO

- DeltaFIFO
  - lock/cond：保护对 items 和 queue 的访问
  - items：
    - string：DeltaFIFO.KeyOf(obj)，obj 可能为
      - Deltas：返回 obj.Newest().Object
      - DeletedFinalStateUnknown：返回 obj.Key
      - 返回 DeltaFIFO.keyFunc(obj)
    - Deltas：[]Delta
      - Delta：
        - Type：Added/Updated/Deleted/Sync
        - Object：发生变更后的对象状态
  - queue：如果 DeltaFIFO.KeyOf(obj) 不在 items 中，则入队 DeltaFIFO.KeyOf(obj)
  - populated：当调用过 Replace/Delete/Add/Update 方法时为 true
  - initialPopulationCount：调用 Replace 时，如果 populated = false，那么设置为 len(queue)
  - keyFunc：MetaNamespaceKeyFunc，格式为 <namespace>/<name>，如果 <namespace> 是空，则为 <name>
  - knownObjects：sharedIndexInformer.indexer，记录 DeltaFIFO.Pop 出来的对象，会在 sharedIndexInformer.HandleDeltas 中更新（Update/Add/Delete），用于 Lister 获取对象，作为持久化数据的一份缓存
  - closed：队列是否关闭
  - closedLock：保护对 closed 的访问

```
type DeltaFIFO struct {
	// lock/cond protects access to 'items' and 'queue'.
	lock sync.RWMutex
	cond sync.Cond

	// `items` maps a key to a Deltas.
	// Each such Deltas has at least one Delta.
	items map[string]Deltas

	// `queue` maintains FIFO order of keys for consumption in Pop().
	// There are no duplicates in `queue`.
	// A key is in `queue` if and only if it is in `items`.
	queue []string

	// populated is true if the first batch of items inserted by Replace() has been populated
	// or Delete/Add/Update/AddIfNotPresent was called first.
	populated bool
	// initialPopulationCount is the number of items inserted by the first call of Replace()
	initialPopulationCount int

	// keyFunc is used to make the key used for queued item
	// insertion and retrieval, and should be deterministic.
	keyFunc KeyFunc

	// knownObjects list keys that are "known" --- affecting Delete(),
	// Replace(), and Resync()
	knownObjects KeyListerGetter

	// Used to indicate a queue is closed so a control loop can exit when a queue is empty.
	// Currently, not used to gate any of CRED operations.
	closed bool

	// emitDeltaTypeReplaced is whether to emit the Replaced or Sync
	// DeltaType when Replace() is called (to preserve backwards compat).
	emitDeltaTypeReplaced bool
	closedLock sync.Mutex //新版本不存在
}

```

##### k8s.io/client-go/tools/cache.sharedProcessor

- sharedProcessor：
  - listenersStarted：已启动的 listeners
  - listenersLock：保护 listeners 的访问
  - listeners：控制器注册的回调合集
    - processorListener：
      - nextCh：用于 processorListener.pop() 与 processorListener.handler 之间进行消息通信
      - addCh：用于 DeltaFIFO.pop() 与 processorListener.pop() 之间进行消息通信
      - handler：控制器回调的封装
      - pendingNotifications：存储所有未分发的消息
      - requestedResyncPeriod：sharedIndexInformer.defaultEventHandlerResyncPeriod
      - resyncPeriod：determineResyncPeriod(sharedIndexInformer.defaultEventHandlerResyncPeriod, sharedIndexInformer.resyncCheckPeriod)
      - nextResync：now.Add(p.resyncPeriod)
      - resyncLock：保护 resyncPeriod 和 nextResync 的访问
  - syncingListeners：需要进行同步的 processorListener
  - clock：k8s.io/apimachinery/pkg/util/clock.RealClock
  - wg：等待所有 processorListener 结束

```
type sharedProcessor struct {
	listenersStarted bool
	listenersLock    sync.RWMutex
	listeners        []*processorListener
	syncingListeners []*processorListener
	clock            clock.Clock
	wg               wait.Group
}
```

##### 启动

工作机制流程图
	
	![image](https://github.com/Mountains-and-rivers/k8s-informers/blob/main/images/03.png)
	
##### k8s.ioo/client-go/tools/cache.sharedIndexInformer.Run

分别启动 k8s.io/client-go/tools/cache.controller.Run 和 k8s.io/client-go/tools/cache.sharedProcessor.Run。

```
func (s *sharedIndexInformer) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	fifo := NewDeltaFIFOWithOptions(DeltaFIFOOptions{
		KnownObjects:          s.indexer,
		EmitDeltaTypeReplaced: true,
	})
	cfg := &Config{
		Queue:            fifo,
		ListerWatcher:    s.listerWatcher,
		ObjectType:       s.objectType,
		FullResyncPeriod: s.resyncCheckPeriod,
		RetryOnError:     false,
		ShouldResync:     s.processor.shouldResync,

		Process:           s.HandleDeltas,
		WatchErrorHandler: s.watchErrorHandler,
	}
	func() {
		s.startedLock.Lock()
		defer s.startedLock.Unlock()

		s.controller = New(cfg)
		s.controller.(*controller).clock = s.clock
		s.started = true
	}()
	// Separate stop channel because Processor should be stopped strictly after controller
	processorStopCh := make(chan struct{})
	var wg wait.Group
	defer wg.Wait()              // Wait for Processor to stop
	defer close(processorStopCh) // Tell Processor to stop
	wg.StartWithChannel(processorStopCh, s.cacheMutationDetector.Run)
	wg.StartWithChannel(processorStopCh, s.processor.run)

	defer func() {
		s.startedLock.Lock()
		defer s.startedLock.Unlock()
		s.stopped = true // Don't want any new listeners
	}()
	s.controller.Run(stopCh)
}
```

##### k8s.io/client-go/tools/cache.controller.Run

- Reflector.Run：
  - Reflector.ListAndWatch：每隔 Reflector.period 执行一次
    - items：通过 list 接口获取对象列表
    - Reflector.syncWith(items, resourceVersion)
      - Reflector.store.Replace：即调用 DeltaFIFO.Replace，入队 items 的 Sync Delta，如果 DeltaFIFO.knownObjects 的 key 不在 items 中，则会入队这些 key 的 Deleted Delta
    - Reflector.setLastSyncResourceVersion(resourceVersion)
    - 周期性执行 Reflector.store.Resync()
      - keys = DeltaFIFO.knownObjects.ListKeys()
      - for key in keys: DeltaFIFO.syncKeyLocked(k)
    - for
      - w = Reflector.listerWatcher.Watch(options)：调用对应 API 资源类型的 watch 接口，例如：client.AppsV1().Deployments(namespace).Watch(options)
        - options：
          - ResourceVersion：同步到的最新版本号
          - TimeoutSeconds：[5min, 10min)
          - AllowWatchBookmarks：为了减轻重新 watch 时 kube-apiserver 的压力，alpha 阶段，未启用
      - Reflector.watchHandler(w, &resourceVersion, resyncerrc, stopCh)：
        - for range w.ResultChan()
          - 根据发生的事件类型调用 Reflector.store.Add/Update/Delete（DeltaFIFO）
          - r.setLastSyncResourceVersion(newResourceVersion)
- controller.processLoop：
  - for
    - obj = c.config.Queue.Pop(PopProcessFunc(c.config.Process)) // DeltaFIFO.Pop
      - sharedIndexInformer.HandleDeltas
        - 如果 Delta.Type in (Sync, Added, Updated)，那么执行 sharedIndexInformer.cacheMutationDetector.AddObject(d.Object)
          - 如果 Delta.Object 在 sharedIndexInformer.indexer 中，则执行 sharedIndexInformer.indexer.Update(d.Object) 和 sharedIndexInformer.processor.distribute(updateNotification{oldObj: old, newObj: d.Object}, isSync)
          - 如果 Delta.Object 不在 sharedIndexInformer.indexer 中，则执行 sharedIndexInformer.indexer.Add(d.Object) 和 sharedIndexInformer.processor.distribute(addNotification{newObj: d.Object}, isSync)
        - 如果 Delta.Type = Deleted，那么执行 sharedIndexInformer.indexer.Delete(d.Object) 和 sharedIndexInformer.processor.distribute(deleteNotification{oldObj: d.Object}, false)
    - 如果 c.config.Process 返回错误，并且 c.config.RetryOnError = true，则执行 c.config.Queue.AddIfNotPresent(obj)

```
func (c *controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	go func() {
		<-stopCh
		c.config.Queue.Close()
	}()
	r := NewReflector(
		c.config.ListerWatcher,
		c.config.ObjectType,
		c.config.Queue,
		c.config.FullResyncPeriod,
	)
	r.ShouldResync = c.config.ShouldResync
	r.WatchListPageSize = c.config.WatchListPageSize
	r.clock = c.clock
	if c.config.WatchErrorHandler != nil {
		r.watchErrorHandler = c.config.WatchErrorHandler
	}

	c.reflectorMutex.Lock()
	c.reflector = r
	c.reflectorMutex.Unlock()

	var wg wait.Group

	wg.StartWithChannel(stopCh, r.Run)

	wait.Until(c.processLoop, time.Second, stopCh)
	wg.Wait()
}

func (r *Reflector) Run(stopCh <-chan struct{}) {
	klog.V(2).Infof("Starting reflector %s (%s) from %s", r.expectedTypeName, r.resyncPeriod, r.name)
	wait.BackoffUntil(func() {
		if err := r.ListAndWatch(stopCh); err != nil {
			r.watchErrorHandler(r, err)
		}
	}, r.backoffManager, true, stopCh)
	klog.V(2).Infof("Stopping reflector %s (%s) from %s", r.expectedTypeName, r.resyncPeriod, r.name)
}
func (c *controller) processLoop() {
	for {
		obj, err := c.config.Queue.Pop(PopProcessFunc(c.config.Process))
		if err != nil {
			if err == ErrFIFOClosed {
				return
			}
			if c.config.RetryOnError {
				// This is the safe way to re-enqueue.
				c.config.Queue.AddIfNotPresent(obj)
			}
		}
	}
}
```

##### k8s.io/client-go/tools/cache.sharedProcessor.Run

- listener.pop：从 processorListener.addCh 中接收消息，如果已有消息等待分发，则存入 processorListener.pendingNotifications，否则进行分发，写入到 processorListener.nextCh 中
  - 为何需要 processorListener.pendingNotifications？我的理解应该是为了避免某个 processorListener 阻塞，导致影响到其他 processorListener 的消息分发
- listener.run：从 processorListener.nextCh 中接收消息
  - 如果是 updateNotification，执行 ResourceEventHandlerFuncs.OnUpdate(notification.oldObj, notification.newObj)
  - 如果是 addNotification，执行 ResourceEventHandlerFuncs.OnAdd(notification.newObj)
  - 如果是 deleteNotification，执行 ResourceEventHandlerFuncs.OnDelete(notification.oldObj)

```
func (p *sharedProcessor) run(stopCh <-chan struct{}) {
	func() {
		p.listenersLock.RLock()
		defer p.listenersLock.RUnlock()
		for _, listener := range p.listeners {
			p.wg.Start(listener.run)
			p.wg.Start(listener.pop)
		}
		p.listenersStarted = true
	}()
	<-stopCh
	p.listenersLock.RLock()
	defer p.listenersLock.RUnlock()
	for _, listener := range p.listeners {
		close(listener.addCh) // Tell .pop() to stop. .pop() will tell .run() to stop
	}
	p.wg.Wait() // Wait for all .pop() and .run() to stop
}
```



func (s *sharedIndexInformer) Run(stopCh <-chan struct{}) {}

```
func (s *sharedIndexInformer) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	fifo := NewDeltaFIFOWithOptions(DeltaFIFOOptions{
		KnownObjects:          s.indexer,
		EmitDeltaTypeReplaced: true,
	})

	fmt.Println("uuuuuuuuuuuuuuuuuuuuuu")
	fmt.Printf("%T \n",s.objectType)

	cfg := &Config{
		Queue:            fifo,
		ListerWatcher:    s.listerWatcher,
		ObjectType:       s.objectType,
		FullResyncPeriod: s.resyncCheckPeriod,
		RetryOnError:     false,
		ShouldResync:     s.processor.shouldResync,

		Process:           s.HandleDeltas,
		WatchErrorHandler: s.watchErrorHandler,
	}
	fmt.Println("iiiiiiiiiiiiiiiiiiiiiiiiiii")
	output := Render(cfg)
	fmt.Print(output)
	func() {
		s.startedLock.Lock()
		defer s.startedLock.Unlock()

		s.controller = New(cfg)
		s.controller.(*controller).clock = s.clock
		s.started = true
	}()

( * cache.Config) {
	Queue: ( * cache.DeltaFIFO) {
		lock: sync.RWMutex {
			w: sync.Mutex {
				state: 0,
				sema: 0
			},
			writerSem: 0,
			readerSem: 0,
			readerCount: 0,
			readerWait: 0
		},
		cond: sync.Cond {
			noCopy: sync.noCopy {},
			L: < REC( * sync.RWMutex) > ,
			notify: sync.notifyList {
				wait: 0,
				notify: 0,
				lock: 0,
				head: (unsafe.Pointer)(0x0000000000000000),
				tail: (unsafe.Pointer)(0x0000000000000000)
			},
			checker: sync.copyChecker(0)
		},
		items: map[string] cache.Deltas {},
		queue: [] string {},
		populated: false,
		initialPopulationCount: 0,
		keyFunc: (cache.KeyFunc)(0x00000000013b70e0),
		knownObjects: ( * cache.cache) {
			cacheStorage: ( * cache.threadSafeMap) {
				lock: sync.RWMutex {
					w: sync.Mutex {
						state: 0,
						sema: 0
					},
					writerSem: 0,
					readerSem: 0,
					readerCount: 0,
					readerWait: 0
				},
				items: map[string] interface {} {},
				indexers: cache.Indexers {
					"namespace": (cache.IndexFunc)(0x00000000013aa800)
				},
				indices: cache.Indices {}
			},
			keyFunc: (cache.KeyFunc)(0x00000000013a6c20)
		},
		closed: false,
		emitDeltaTypeReplaced: true
	},
	ListerWatcher: ( * cache.ListWatch) {
		ListFunc: (cache.ListFunc)(0x00000000013f82e0),
		WatchFunc: (cache.WatchFunc)(0x00000000013f8540),
		DisableChunking: false
	},
	Process: (cache.ProcessFunc)(0x00000000013be3e0),
	ObjectType: ( * v1.Pod) {
		TypeMeta: v1.TypeMeta {
			Kind: "",
			APIVersion: ""
		},
		ObjectMeta: v1.ObjectMeta {
			Name: "",
			GenerateName: "",
			Namespace: "",
			SelfLink: "",
			UID: types.UID(""),
			ResourceVersion: "",
			Generation: 0,
			CreationTimestamp: v1.Time {
				Time: time.Time {
					wall: 0,
					ext: 0,
					loc: ( * time.Location)(nil)
				}
			},
			DeletionTimestamp: ( * v1.Time)(nil),
			DeletionGracePeriodSeconds: ( * int64)(nil),
			Labels: map[string] string(nil),
			Annotations: map[string] string(nil),
			OwnerReferences: [] v1.OwnerReference(nil),
			Finalizers: [] string(nil),
			ClusterName: "",
			ManagedFields: [] v1.ManagedFieldsEntry(nil)
		},
		Spec: v1.PodSpec {
			Volumes: [] v1.Volume(nil),
			InitContainers: [] v1.Container(nil),
			Containers: [] v1.Container(nil),
			EphemeralContainers: [] v1.EphemeralContainer(nil),
			RestartPolicy: v1.RestartPolicy(""),
			TerminationGracePeriodSeconds: ( * int64)(nil),
			ActiveDeadlineSeconds: ( * int64)(nil),
			DNSPolicy: v1.DNSPolicy(""),
			NodeSelector: map[string] string(nil),
			ServiceAccountName: "",
			DeprecatedServiceAccount: "",
			AutomountServiceAccountToken: ( * bool)(nil),
			NodeName: "",
			HostNetwork: false,
			HostPID: false,
			HostIPC: false,
			ShareProcessNamespace: ( * bool)(nil),
			SecurityContext: ( * v1.PodSecurityContext)(nil),
			ImagePullSecrets: [] v1.LocalObjectReference(nil),
			Hostname: "",
			Subdomain: "",
			Affinity: ( * v1.Affinity)(nil),
			SchedulerName: "",
			Tolerations: [] v1.Toleration(nil),
			HostAliases: [] v1.HostAlias(nil),
			PriorityClassName: "",
			Priority: ( * int32)(nil),
			DNSConfig: ( * v1.PodDNSConfig)(nil),
			ReadinessGates: [] v1.PodReadinessGate(nil),
			RuntimeClassName: ( * string)(nil),
			EnableServiceLinks: ( * bool)(nil),
			PreemptionPolicy: ( * v1.PreemptionPolicy)(nil),
			Overhead: v1.ResourceList(nil),
			TopologySpreadConstraints: [] v1.TopologySpreadConstraint(nil),
			SetHostnameAsFQDN: ( * bool)(nil)
		},
		Status: v1.PodStatus {
			Phase: v1.PodPhase(""),
			Conditions: [] v1.PodCondition(nil),
			Message: "",
			Reason: "",
			NominatedNodeName: "",
			HostIP: "",
			PodIP: "",
			PodIPs: [] v1.PodIP(nil),
			StartTime: ( * v1.Time)(nil),
			InitContainerStatuses: [] v1.ContainerStatus(nil),
			ContainerStatuses: [] v1.ContainerStatus(nil),
			QOSClass: v1.PodQOSClass(""),
			EphemeralContainerStatuses: [] v1.ContainerStatus(nil)
		}
	},
	FullResyncPeriod: time.Duration(30000000000),
	ShouldResync: (cache.ShouldResyncFunc)(0x00000000013be380),
	RetryOnError: false,
	WatchErrorHandler: (cache.WatchErrorHandler)(0x0000000000000000),
	WatchListPageSize: 0
}
```

