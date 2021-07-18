# informers机制分析

```
package main

import (
	"context"
	"flag"
	"fmt"
	v1 "k8s.io/api/apps/v1"
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
	deployInformer := factory.Apps().V1().StatefulSets()
	informer:=deployInformer.Informer()
	deployLister := deployInformer.Lister()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			deploy := obj.(*v1.StatefulSet)
			// 业务逻辑
			fmt.Println("add a deploy:", deploy.Name,time.Now().Format("2006-01-02 15:04:05"))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeploy := oldObj.(*v1.StatefulSet)
			newDeploy := newObj.(*v1.StatefulSet)
			fmt.Println("update a deploy:", oldDeploy.Name, newDeploy.Name,time.Now().Format("2006-01-02 15:04:05"))
		},
		DeleteFunc: func(obj interface{}) {
			deploy := obj.(*v1.StatefulSet)
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
	StatefulSet, err := deployLister.StatefulSets("default").List(labels.Everything())
	if err != err {
		panic(err.Error())
	}
	for idx, deploy := range StatefulSet {
		fmt.Printf("%d -->> %s \n", idx+1, deploy.Name)
	}
	<-stopChan
}
```

Inform_k8s():

```
//初始化一个只关心StatefulSets的informer对象
deployInformer := factory.Apps().V1().StatefulSets()
具体可查看代码：vendor\k8s.io\client-go\informers\apps\v1\interface.go
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

