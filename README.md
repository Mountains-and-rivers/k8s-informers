# k8s-informers
## Clientset 使用

Clientset 是调用 Kubernetes 资源对象最常用的客户端，可以操作所有的资源对象。

前面我们说了在 `staging/src/k8s.io/api` 下面定义了各种类型资源的规范，然后将这些规范注册到了全局的 Scheme 中，这样就可以在 `Clientset` 中使用这些资源了。那么我们应该如何使用 Clientset 呢？

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
	 Con_k8s()

	//informer 测试
	//Inform_k8s()

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

	// 初始化 informers factory
	factory := informers.NewSharedInformerFactory(clientset, time.Second*30)
	// 监听想要获取的资源对象 informer
	deployInformer := factory.Apps().V1().Deployments()
	// 相当于注册下Informer
	informer := deployInformer.Informer()
	// 创建Lister，从缓存当中去获取
	deployLister := deployInformer.Lister()
	// 注册时间处理程序 (Add，Delete，Update)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			deploy := obj.(*v1.Deployment)
			// 业务逻辑
			fmt.Println("add a deploy:", deploy.Name,time.Now().Format("2006-01-02 15:04:05"))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeploy := oldObj.(*v1.Deployment)
			newDeploy := newObj.(*v1.Deployment)
			fmt.Println("update a deploy:", oldDeploy.Name, newDeploy.Name,time.Now().Format("2006-01-02 15:04:05"))
		},
		DeleteFunc: func(obj interface{}) {
			deploy := obj.(*v1.Deployment)
			fmt.Println("delete a deploy:", deploy.Name)
		},
	})

	stopCh := make(chan struct{})
	defer close(stopCh)
	// 启动informer (List Watch)
	factory.Start(stopCh)
	//等待所有启动的informer同步完成
	factory.WaitForCacheSync(stopCh)
	// 通过Lister获取缓存的Deployment
	deployments, err := deployLister.Deployments("default").List(labels.Everything())
	if err != err {
		panic(err.Error())
	}
	for idx, deploy := range deployments {
		fmt.Printf("%d -->> %s \n", idx+1, deploy.Name)
	}
	<-stopCh
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
		deployment, err := clientset.AppsV1().Deployments("metallb-system").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}

		for idx, deploy := range deployment.Items {
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

运行Con_k8s()函数可以获得default 命名空间之下的 Pods：

```
[root@node test]# go run main.go 
1 -> nginx
```

这是一个非常典型的访问 Kubernetes 集群资源的方式，通过 client-go 提供的 Clientset 对象来获取资源数据，主要有以下三个步骤：

1. 使用 kubeconfig 文件或者 ServiceAccount（InCluster 模式）来创建访问 Kubernetes API 的 Restful 配置参数，也就是代码中的 `rest.Config` 对象
2. 使用 rest.Config 参数创建 Clientset 对象，这一步非常简单，直接调用 `kubernetes.NewForConfig(config)` 即可初始化
3. 然后是 Clientset 对象的方法去获取各个 Group 下面的对应资源对象进行 CRUD 操作

## Clientset 对象

上面我们了解了如何使用 Clientset 对象来获取集群资源，接下来我们来分析下 Clientset 对象的实现。

上面我们使用的 Clientset 实际上是对各种资源类型的 Clientset 的一次封装：

```
// NewForConfig 使用给定的 config 创建一个新的 Clientset
func NewForConfig(c *rest.Config) (*Clientset, error) {
	configShallowCopy := *c
	if configShallowCopy.RateLimiter == nil && configShallowCopy.QPS > 0 {
		if configShallowCopy.Burst <= 0 {
			return nil, fmt.Errorf("burst is required to be greater than 0 when RateLimiter is not set and QPS is set to greater than 0")
		}
		configShallowCopy.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(configShallowCopy.QPS, configShallowCopy.Burst)
	}
	var cs Clientset
	var err error
	cs.admissionregistrationV1, err = admissionregistrationv1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.admissionregistrationV1beta1, err = admissionregistrationv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.internalV1alpha1, err = internalv1alpha1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	// 将其他 Group 和版本的资源的 RESTClient 封装到全局的 Clientset 对象中
	
	cs.appsV1, err = appsv1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.appsV1beta1, err = appsv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.appsV1beta2, err = appsv1beta2.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.authenticationV1, err = authenticationv1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.authenticationV1beta1, err = authenticationv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.authorizationV1, err = authorizationv1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.authorizationV1beta1, err = authorizationv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.autoscalingV1, err = autoscalingv1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.autoscalingV2beta1, err = autoscalingv2beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.autoscalingV2beta2, err = autoscalingv2beta2.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.batchV1, err = batchv1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.batchV1beta1, err = batchv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.batchV2alpha1, err = batchv2alpha1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.certificatesV1, err = certificatesv1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.certificatesV1beta1, err = certificatesv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.coordinationV1beta1, err = coordinationv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.coordinationV1, err = coordinationv1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.coreV1, err = corev1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.discoveryV1alpha1, err = discoveryv1alpha1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.discoveryV1beta1, err = discoveryv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.eventsV1, err = eventsv1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.eventsV1beta1, err = eventsv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.extensionsV1beta1, err = extensionsv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.flowcontrolV1alpha1, err = flowcontrolv1alpha1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.networkingV1, err = networkingv1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.networkingV1beta1, err = networkingv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.nodeV1alpha1, err = nodev1alpha1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.nodeV1beta1, err = nodev1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.policyV1beta1, err = policyv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.rbacV1, err = rbacv1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.rbacV1beta1, err = rbacv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.rbacV1alpha1, err = rbacv1alpha1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.schedulingV1alpha1, err = schedulingv1alpha1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.schedulingV1beta1, err = schedulingv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.schedulingV1, err = schedulingv1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.storageV1beta1, err = storagev1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.storageV1, err = storagev1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.storageV1alpha1, err = storagev1alpha1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}

	cs.DiscoveryClient, err = discovery.NewDiscoveryClientForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	return &cs, nil
}
```

上面的 NewForConfig 函数里面就是将其他的各种资源的 RESTClient 封装到了全局的 Clientset 中，这样当我们需要访问某个资源的时候只需要使用 Clientset 里面包装的属性即可，比如 clientset.CoreV1() 就是访问 Core 这个 Group 下面 v1 这个版本的 RESTClient。这些局部的 RESTClient 都定义在 staging/src/k8s.io/client-go/typed/<group>/<version>/<plug>_client.go 文件中，比如 staging/src/k8s.io/client-go/kubernetes/typed/apps/v1/apps_client.go 这个文件中就是定义的 apps 这个 Group 下面的 v1 版本的 RESTClient，这里同样以 Deployment 为例：

```
k8s.io\Kubernetes\staging\src\k8s.io\client-go\kubernetes\typed\apps\v1\apps_client.go

// NewForConfig 根据 rest.Config 创建一个 AppsV1Client
func NewForConfig(c *rest.Config) (*AppsV1Client, error) {
	config := *c
	// 为 rest.Config 设置资源对象默认的参数
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	 // 实例化 AppsV1Client 的 RestClient
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &AppsV1Client{client}, nil
}

func setConfigDefaults(config *rest.Config) error {
    // 资源对象的 GroupVersion
	gv := v1.SchemeGroupVersion
	config.GroupVersion = &gv
	// 资源对象的 root path
	config.APIPath = "/apis"
	// 使用注册的资源类型 Scheme 对请求和响应进行编解码，Scheme 就是前文中分析的资源类型的规范
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

func (c *AppsV1Client) Deployments(namespace string) DeploymentInterface {
	return newDeployments(c, namespace)
}

k8s.io\Kubernetes\staging\src\k8s.io\client-go\kubernetes\typed\apps\v1\deployment.go

// deployments 实现了 DeploymentInterface 接口
type deployments struct {
	client rest.Interface
	ns     string
}
// newDeployments 实例化 deployments 对象
func newDeployments(c *AppsV1Client, namespace string) *deployments {
	return &deployments{
		client: c.RESTClient(),
		ns:     namespace,
	}
}
```

通过上面代码我们就可以很清晰的知道可以通过 `clientset.AppsV1().Deployments("default")`来获取一个 deployments 对象，然后该对象下面定义了 deployments 对象的 CRUD 操作，比如我们调用的 `List()` 函数：

```
func (c *deployments) List(ctx context.Context, opts metav1.ListOptions) (result *v1.DeploymentList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.DeploymentList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("deployments").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}
```

从上面代码可以看出最终是通过 c.client 去发起的请求，也就是局部的 restClient 初始化的函数中通过 `rest.RESTClientFor(&config)` 创建的对象，也就是将 `rest.Config` 对象转换成一个 Restful 的 Client 对象用于网络操作：

```
// RESTClientFor 返回一个满足客户端 Config 对象上的属性的 RESTClient 对象。
// 注意在初始化客户端的时候，RESTClient 可能需要一些可选的属性。

func RESTClientFor(config *Config) (*RESTClient, error) {
	if config.GroupVersion == nil {
		return nil, fmt.Errorf("GroupVersion is required when initializing a RESTClient")
	}
	if config.NegotiatedSerializer == nil {
		return nil, fmt.Errorf("NegotiatedSerializer is required when initializing a RESTClient")
	}

	baseURL, versionedAPIPath, err := defaultServerUrlFor(config)
	if err != nil {
		return nil, err
	}

	transport, err := TransportFor(config)
	if err != nil {
		return nil, err
	}

	var httpClient *http.Client
	if transport != http.DefaultTransport {
		httpClient = &http.Client{Transport: transport}
		if config.Timeout > 0 {
			httpClient.Timeout = config.Timeout
		}
	}

	rateLimiter := config.RateLimiter
	if rateLimiter == nil {
		qps := config.QPS
		if config.QPS == 0.0 {
			qps = DefaultQPS
		}
		burst := config.Burst
		if config.Burst == 0 {
			burst = DefaultBurst
		}
		if qps > 0 {
			rateLimiter = flowcontrol.NewTokenBucketRateLimiter(qps, burst)
		}
	}

	var gv schema.GroupVersion
	if config.GroupVersion != nil {
		gv = *config.GroupVersion
	}
	clientContent := ClientContentConfig{
		AcceptContentTypes: config.AcceptContentTypes,
		ContentType:        config.ContentType,
		GroupVersion:       gv,
		Negotiator:         runtime.NewClientNegotiator(config.NegotiatedSerializer, gv),
	}

	restClient, err := NewRESTClient(baseURL, versionedAPIPath, clientContent, rateLimiter, httpClient)
	if err == nil && config.WarningHandler != nil {
		restClient.warningHandler = config.WarningHandler
	}
	return restClient, err
}
```

到这里我们就知道了 **Clientset 是基于 RESTClient 的**，RESTClient 是底层的用于网络请求的对象，可以直接通过 RESTClient 提供的 RESTful 方法如 Get()、Put()、Post()、Delete() 等和 APIServer 进行交互

- 同时支持 JSON 和 protobuf 两种序列化方式
- 支持所有原生资源

当 初始化 RESTClient 过后就可以发起网络请求了，比如对于 Deployments 的 List 操作：

```
func (c *deployments) List(ctx context.Context, opts metav1.ListOptions) (result *v1.DeploymentList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.DeploymentList{}
	// 调用 RestClient 发起真正的网络请求
	err = c.client.Get().
		Namespace(c.ns).
		Resource("deployments").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}
```

上面通过调用 RestClient 发起网络请求，真正发起网络请求的代码如下所示：

```
k8s.io\Kubernetes\staging\src\k8s.io\client-go\rest\request.go

func (r *Request) request(ctx context.Context, fn func(*http.Request, *http.Response)) error {
	//Metrics for total request latency
	start := time.Now()
	defer func() {
		metrics.RequestLatency.Observe(r.verb, r.finalURLTemplate(), time.Since(start))
	}()

	if r.err != nil {
		klog.V(4).Infof("Error in request: %v", r.err)
		return r.err
	}

	if err := r.requestPreflightCheck(); err != nil {
		return err
	}
    // 初始化网络客户端
	client := r.c.Client
	if client == nil {
		client = http.DefaultClient
	}

	// Throttle the first try before setting up the timeout configured on the
	// client. We don't want a throttled client to return timeouts to callers
	// before it makes a single request.
	if err := r.tryThrottle(ctx); err != nil {
		return err
	}
    // 超时处理
	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	// Right now we make about ten retry attempts if we get a Retry-After response.
	// 重试机制，最多重试10次
	retries := 0
	for {

		url := r.URL().String()
		// 构造请求对象
		req, err := http.NewRequest(r.verb, url, r.body)
		if err != nil {
			return err
		}
		req = req.WithContext(ctx)
		req.Header = r.headers

		r.backoff.Sleep(r.backoff.CalculateBackoff(r.URL()))
		if retries > 0 {
			// We are retrying the request that we already send to apiserver
			// at least once before.
			// This request should also be throttled with the client-internal rate limiter.
			if err := r.tryThrottle(ctx); err != nil {
				return err
			}
		}
		// 发起网络调用
		resp, err := client.Do(req)
		updateURLMetrics(r, resp, err)
		if err != nil {
			r.backoff.UpdateBackoff(r.URL(), err, 0)
		} else {
			r.backoff.UpdateBackoff(r.URL(), err, resp.StatusCode)
		}
		if err != nil {
			// "Connection reset by peer" or "apiserver is shutting down" are usually a transient errors.
			// Thus in case of "GET" operations, we simply retry it.
			// We are not automatically retrying "write" operations, as
			// they are not idempotent.
			if r.verb != "GET" {
				return err
			}
			// For connection errors and apiserver shutdown errors retry.
			if net.IsConnectionReset(err) || net.IsProbableEOF(err) {
				// For the purpose of retry, we set the artificial "retry-after" response.
				// TODO: Should we clean the original response if it exists?
				resp = &http.Response{
					StatusCode: http.StatusInternalServerError,
					Header:     http.Header{"Retry-After": []string{"1"}},
					Body:       ioutil.NopCloser(bytes.NewReader([]byte{})),
				}
			} else {
				return err
			}
		}

		done := func() bool {
			// Ensure the response body is fully read and closed
			// before we reconnect, so that we reuse the same TCP
			// connection.
			defer func() {
				const maxBodySlurpSize = 2 << 10
				if resp.ContentLength <= maxBodySlurpSize {
					io.Copy(ioutil.Discard, &io.LimitedReader{R: resp.Body, N: maxBodySlurpSize})
				}
				resp.Body.Close()
			}()

			retries++
			if seconds, wait := checkWait(resp); wait && retries <= r.maxRetries {
				if seeker, ok := r.body.(io.Seeker); ok && r.body != nil {
					_, err := seeker.Seek(0, 0)
					if err != nil {
						klog.V(4).Infof("Could not retry request, can't Seek() back to beginning of body for %T", r.body)
						// 将req和resp回调
						fn(req, resp)
						return true
					}
				}

				klog.V(4).Infof("Got a Retry-After %ds response for attempt %d to %v", seconds, retries, url)
				r.backoff.Sleep(time.Duration(seconds) * time.Second)
				return false
			}
			fn(req, resp)
			return true
		}()
		if done {
			return nil
		}
	}
}

```

到这里就完成了一次完整的网络请求。

其实 Clientset 对象也就是将 rest.Config 封装成了一个 `http.Client` 对象而已，最终还是利用 golang 中的 http 库来执行一个正常的网络请求而已。

## Informer 使用

前面我们在使用 Clientset 的时候了解到我们可以使用 Clientset 来获取所有的原生资源对象，那么如果我们想要去一直获取集群的资源对象数据呢？岂不是需要用一个轮询去不断执行 List() 操作？这显然是不合理的，实际上除了常用的 CRUD 操作之外，我们还可以进行 Watch 操作，可以监听资源对象的增、删、改、查操作，这样我们就可以根据自己的业务逻辑去处理这些数据了。

Watch 通过一个 event 接口监听对象的所有变化（添加、删除、更新）：

```
k8s.io\Kubernetes\staging\src\k8s.io\apimachinery\pkg\watch\watch.go

// Interface 可以被任何知道如何 watch 和通知变化的对象实现
type Interface interface {
	// Stops watching. Will close the channel returned by ResultChan(). Releases
	// any resources used by the watch.
	Stop()

	// Returns a chan which will receive all the events. If an error occurs
	// or Stop() is called, the implementation will close this channel and
	// release any resources used by the watch.
	ResultChan() <-chan Event
}

```

watch 接口的 ResultChan 方法会返回如下几种事件：

```sql
// staging/src/k8s.io/apimachinery/pkg/watch/watch.go
// EventType 定义可能的事件类型
type EventType string

const (
	Added    EventType = "ADDED"
	Modified EventType = "MODIFIED"
	Deleted  EventType = "DELETED"
	Bookmark EventType = "BOOKMARK"
	Error    EventType = "ERROR"

	DefaultChanSize int32 = 100
)

// Event represents a single event to a watched resource.
// +k8s:deepcopy-gen=true
type Event struct {
	Type EventType

	// Object is:
	//  * If Type is Added or Modified: the new state of the object.
	//  * If Type is Deleted: the state of the object immediately before deletion.
	//  * If Type is Bookmark: the object (instance of a type being watched) where
	//    only ResourceVersion field is set. On successful restart of watch from a
	//    bookmark resourceVersion, client is guaranteed to not get repeat event
	//    nor miss any events.
	//  * If Type is Error: *api.Status is recommended; other types may make sense
	//    depending on context.
	Object runtime.Object
}
```

这个接口虽然我们可以直接去使用，但是实际上并不建议这样使用，因为往往由于集群中的资源较多，我们需要自己在客户端去维护一套缓存，而这个维护成本也是非常大的，为此 client-go 也提供了自己的实现机制，那就是 `Informers`。Informers 是这个事件接口和带索引查找功能的内存缓存的组合，这样也是目前最常用的用法。Informers 第一次被调用的时候会首先在客户端调用 List 来获取全量的对象集合，然后通过 Watch 来获取增量的对象更新缓存。

## 运行原理

一个控制器每次需要获取对象的时候都要访问 APIServer，这会给系统带来很高的负载，Informers 的内存缓存就是来解决这个问题的，此外 Informers 还可以几乎实时的监控对象的变化，而不需要轮询请求，这样就可以保证客户端的缓存数据和服务端的数据一致，就可以大大降低 APIServer 的压力了。

![https://bxdc-static.oss-cn-beijing.aliyuncs.com/images/20200727110511.png](https://bxdc-static.oss-cn-beijing.aliyuncs.com/images/20200727110511.png)

Informers

如上图展示了 Informer 的基本处理流程：

- 以 events 事件的方式从 APIServer 获取数据
- 提供一个类似客户端的 Lister 接口，从内存缓存中 get 和 list 对象
- 为添加、删除、更新注册事件处理程序

此外 Informers 也有错误处理方式，当长期运行的 watch 连接中断时，它们会尝试使用另一个 watch 请求来恢复连接，在不丢失任何事件的情况下恢复事件流。如果中断的时间较长，而且 APIServer 丢失了事件（etcd 在新的 watch 请求成功之前从数据库中清除了这些事件），那么 Informers 就会重新 List 全量数据。

而且在重新 List 全量操作的时候还可以配置一个重新同步的周期参数，用于协调内存缓存数据和业务逻辑的数据一致性，每次过了该周期后，注册的事件处理程序就将被所有的对象调用，通常这个周期参数以分为单位，比如10分钟或者30分钟。

> 注意：重新同步是纯内存操作，不会触发对服务器的调用。

Informers 的这些高级特性以及超强的鲁棒性，都足以让我们不去直接使用客户端的 Watch() 方法来处理自己的业务逻辑，而且在 Kubernetes 中也有很多地方都有使用到 Informers。但是在使用 Informers 的时候，通常每个 GroupVersionResource（GVR）只实例化一个 Informers，但是有时候我们在一个应用中往往有使用多种资源对象的需求，这个时候为了方便共享 Informers，我们可以通过使用共享 Informer 工厂来实例化一个 Informer。

共享 Informer 工厂允许我们在应用中为同一个资源共享 Informer，也就是说不同的控制器循环可以使用相同的 watch 连接到后台的 APIServer，例如，kube-controller-manager 中的控制器数据量就非常多，但是对于每个资源（比如 Pod），在这个进程中只有一个 Informer。

## 示例

首先我们创建一个 Clientset 对象，然后使用 Clientset 来创建一个共享的 Informer 工厂，Informer 是通过 `informer-gen` 这个代码生成器工具自动生成的，位于 `k8s.io/client-go/informers` 中。

这里我们来创建一个用于获取 Deployment 的共享 Informer，代码如下所示：

```go
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

	// 初始化 informers factory
	factory := informers.NewSharedInformerFactory(clientset, time.Second*30)
	// 监听想要获取的资源对象 informer
	deployInformer := factory.Apps().V1().Deployments()
	// 相当于注册下Informer
	informer := deployInformer.Informer()
	// 创建Lister，从缓存当中去获取
	deployLister := deployInformer.Lister()
	// 注册时间处理程序 (Add，Delete，Update)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			deploy := obj.(*v1.Deployment)
			// 业务逻辑
			fmt.Println("add a deploy:", deploy.Name,time.Now().Format("2006-01-02 15:04:05"))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeploy := oldObj.(*v1.Deployment)
			newDeploy := newObj.(*v1.Deployment)
			fmt.Println("update a deploy:", oldDeploy.Name, newDeploy.Name,time.Now().Format("2006-01-02 15:04:05"))
		},
		DeleteFunc: func(obj interface{}) {
			deploy := obj.(*v1.Deployment)
			fmt.Println("delete a deploy:", deploy.Name)
		},
	})

	stopCh := make(chan struct{})
	defer close(stopCh)
	// 启动informer (List Watch)
	factory.Start(stopCh)
	//等待所有启动的informer同步完成
	factory.WaitForCacheSync(stopCh)
	// 通过Lister获取缓存的Deployment
	deployments, err := deployLister.Deployments("default").List(labels.Everything())
	if err != err {
		panic(err.Error())
	}
	for idx, deploy := range deployments {
		fmt.Printf("%d -->> %s \n", idx+1, deploy.Name)
	}
	<-stopCh
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
		deployment, err := clientset.AppsV1().Deployments("metallb-system").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}

		for idx, deploy := range deployment.Items {
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

上面的代码运行可以获得 default 命名空间之下的所有 Deployment 信息以及整个集群的 Deployment 数据：

```bash
[root@node test]# go run main.go 
add a deploy: myapp-demo 2021-05-03 16:03:22
add a deploy: opdemo-controller-manager 2021-05-03 16:03:22
add a deploy: coredns 2021-05-03 16:03:22
add a deploy: nginx 2021-05-03 16:03:22
1 -->> myapp-demo 
update a deploy: myapp-demo myapp-demo 2021-05-03 16:03:52
update a deploy: opdemo-controller-manager opdemo-controller-manager 2021-05-03 16:03:52
update a deploy: coredns coredns 2021-05-03 16:03:52
update a deploy: nginx nginx 2021-05-03 16:03:52
```

这是因为我们首先通过 Informer 注册了事件处理程序，这样当我们启动 Informer 的时候首先会将集群的全量 Deployment 数据同步到本地的缓存中，会触发 `AddFunc` 这个回调函数，然后我们又在下面使用 `Lister()` 来获取 default 命名空间下面的所有 Deployment 数据，这个时候的数据是从本地的缓存中获取的，所以就看到了上面的结果，由于我们还配置了每30s重新全量 List 一次，所以正常每30s我们也可以看到所有的 Deployment 数据出现在 `UpdateFunc` 回调函数下面，我们也可以尝试去删除一个 Deployment，同样也会出现对应的 `DeleteFunc` 下面的事件。

## Informer 架构说明

下图是整个 client-go 的完整架构图，或者说是我们要去实现一个自定义的控制器的一个整体流程，其中黄色图标是开发者需要自行开发的部分，而其它的部分是 client-go 已经提供的，直接使用即可。 由于 client-go 实现非常复杂，我们这里先对上图中最核心的部分 Informer 进行说明。在 Informer 的架构中包含如下几个核心的组件：

Informers 是 client-go 中非常重要得概念，接下来我们来仔细分析下 Informers 的实现原理，下图是 client-go 的官方实现架构图：

![https://bxdc-static.oss-cn-beijing.aliyuncs.com/images/client-go-controller-interaction.jpeg](https://bxdc-static.oss-cn-beijing.aliyuncs.com/images/client-go-controller-interaction.jpeg)

**Reflector（反射器）**

Reflector 用于监控（Watch）指定的 Kubernetes 资源，当监控的资源发生变化时，触发相应的变更事件，例如 Add 事件、Update 事件、Delete 事件，并将其资源对象存放到本地缓存 DeltaFIFO 中。

**DeltaFIFO**

DeltaFIFO 是一个生产者-消费者的队列，生产者是 Reflector，消费者是 Pop 函数，FIFO 是一个先进先出的队列，而 Delta 是一个资源对象存储，它可以保存资源对象的操作类型，例如 Add 操作类型、Update 操作类型、Delete 操作类型、Sync 操作类型等。

**Indexer**

Indexer 是 client-go 用来存储资源对象并自带索引功能的本地存储，Reflector 从 DeltaFIFO 中将消费出来的资源对象存储至 Indexer。Indexer 与 Etcd 集群中的数据保持完全一致。这样我们就可以很方便地从本地存储中读取相应的资源对象数据，而无须每次从远程 APIServer 中读取，以减轻服务器的压力。

这里理论知识太多，直接去查看源码显得有一定困难，我们可以用一个实际的示例来进行说明，比如现在我们删除一个 Pod，一个 Informers 的执行流程是怎样的：

1. 首先初始化 Informer，Reflector 通过 List 接口获取所有的 Pod 对象
2. Reflector 拿到所有 Pod 后，将全部 Pod 放到 Store（本地缓存）中
3. 如果有人调用 Lister 的 List/Get 方法获取 Pod，那么 Lister 直接从 Store 中去拿数据
4. Informer 初始化完成后，Reflector 开始 Watch Pod 相关的事件
5. 此时如果我们删除 Pod1，那么 Reflector 会监听到这个事件，然后将这个事件发送到 DeltaFIFO 中
6. DeltaFIFO 首先先将这个事件存储在一个队列中，然后去操作 Store 中的数据，删除其中的 Pod1
7. DeltaFIFO 然后 Pop 这个事件到事件处理器（资源事件处理器）中进行处理
8. LocalStore 会周期性地把所有的 Pod 信息重新放回 DeltaFIFO 中去

## Reflector 源码分析

前面我们说了 Informer 通过对 APIServer 的资源对象执行 List 和 Watch 操作，把获取到的数据存储在本地的缓存中，其中实现这个的核心功能就是 Reflector，我们可以称其为反射器，从名字我们可以看出来它的主要功能就是反射，就是将 Etcd 里面的数据反射到本地存储（DeltaFIFO）中。Reflector 首先通过 List 操作获取所有的资源对象数据，保存到本地存储，然后通过 Watch 操作监控资源的变化，触发相应的事件处理，比如前面示例中的 Add 事件、Update 事件、Delete 事件。

Reflector 结构体的定义位于 `staging/src/k8s.io/client-go/tools/cache/reflector.go` 下面：

```
k8s.io\Kubernetes\staging\src\k8s.io\client-go\tools\cache\reflector.go

// Reflector(反射器) 监听指定的资源，将所有的变化都反射到给定的存储中去
type Reflector struct {
	// name identifies this reflector. By default it will be a file:line if possible.
// name 标识这个反射器的名称，默认为 文件:行数（比如reflector.go:125）
// 默认名字通过 k8s.io/apimachinery/pkg/util/naming/from_stack.go 下面的 GetNameFromCallsite 函数生成
	name string
 // 期望放到 Store 中的类型名称，如果提供，则是 expectedGVK 的字符串形式
  // 否则就是 expectedType 的字符串，它仅仅用于显示，不用于解析或者比较。
	expectedTypeName string
	// An example object of the type we expect to place in the store.
	// Only the type needs to be right, except that when that is
	// `unstructured.Unstructured` the object's `"apiVersion"` and
	// `"kind"` must also be right.
	// 我们放到 Store 中的对象类型
	expectedType reflect.Type
	// The GVK of the object we expect to place in the store if unstructured.
	// 如果是非结构化的，我们期望放在 Sotre 中的对象的 GVK
	expectedGVK *schema.GroupVersionKind
	// The destination to sync up with the watch source
	// 与 watch 源同步的目标 Store
	store Store
	// listerWatcher is used to perform lists and watches.
	// 用来执行 lists 和 watches 操作的 listerWatcher 接口（最重要的）
	listerWatcher ListerWatcher

	// backoff manages backoff of ListWatch
	backoffManager wait.BackoffManager
	// initConnBackoffManager manages backoff the initial connection with the Watch calll of ListAndWatch.
	initConnBackoffManager wait.BackoffManager

	resyncPeriod time.Duration
	// ShouldResync is invoked periodically and whenever it returns `true` the Store's Resync operation is invoked
	// ShouldResync 会周期性的被调用，当返回 true 的时候，就会调用 Store 的 Resync 操作
	ShouldResync func() bool
	// clock allows tests to manipulate time
	clock clock.Clock
	// paginatedResult defines whether pagination should be forced for list calls.
	// It is set based on the result of the initial list call.
	paginatedResult bool
	// lastSyncResourceVersion is the resource version token last
	// observed when doing a sync with the underlying store
	// it is thread safe, but not synchronized with the underlying store
	// Kubernetes 资源在 APIServer 中都是有版本的，对象的任何修改(添加、删除、更新)都会造成资源版本更新，lastSyncResourceVersion 就是指的这个版本
	lastSyncResourceVersion string
	// isLastSyncResourceVersionUnavailable is true if the previous list or watch request with
	// lastSyncResourceVersion failed with an "expired" or "too large resource version" error.
	// 如果之前的 list 或 watch 带有 lastSyncResourceVersion 的请求中是一个 HTTP 410（Gone）的失败请求，则 isLastSyncResourceVersionGone 为 true
	isLastSyncResourceVersionUnavailable bool
	// lastSyncResourceVersionMutex guards read/write access to lastSyncResourceVersion
	// lastSyncResourceVersionMutex 用于保证对 lastSyncResourceVersion 的读/写访问。
	lastSyncResourceVersionMutex sync.RWMutex
	// WatchListPageSize is the requested chunk size of initial and resync watch lists.
	// If unset, for consistent reads (RV="") or reads that opt-into arbitrarily old data
	// (RV="0") it will default to pager.PageSize, for the rest (RV != "" && RV != "0")
	// it will turn off pagination to allow serving them from watch cache.
	// NOTE: It should be used carefully as paginated lists are always served directly from
	// etcd, which is significantly less efficient and may lead to serious performance and
	// scalability problems.
	WatchListPageSize int64
	// Called whenever the ListAndWatch drops the connection with an error.
	watchErrorHandler WatchErrorHandler
}

// NewReflector 创建一个新的反射器对象，将使给定的 Store 保持与服务器中指定的资源对象的内容同步。
// 反射器只把具有 expectedType 类型的对象放到 Store 中，除非 expectedType 是 nil。
// 如果 resyncPeriod 是非0，那么反射器会周期性地检查 ShouldResync 函数来决定是否调用 Store 的 Resync 操作
// `ShouldResync==nil` 意味着总是要执行 Resync 操作。
// 这使得你可以使用反射器周期性地处理所有的全量和增量的对象。

func NewReflector(lw ListerWatcher, expectedType interface{}, store Store, resyncPeriod time.Duration) *Reflector {
   // 默认的反射器名称为 file:line
	return NewNamedReflector(naming.GetNameFromCallsite(internalPackages...), lw, expectedType, store, resyncPeriod)
}
// NewNamedReflector 与 NewReflector 一样，只是指定了一个 name 用于日志记录
func NewNamedReflector(name string, lw ListerWatcher, expectedType interface{}, store Store, resyncPeriod time.Duration) *Reflector {
	realClock := &clock.RealClock{}
	r := &Reflector{
		name:          name,
		listerWatcher: lw,
		store:         store,
		// We used to make the call every 1sec (1 QPS), the goal here is to achieve ~98% traffic reduction when
		// API server is not healthy. With these parameters, backoff will stop at [30,60) sec interval which is
		// 0.22 QPS. If we don't backoff for 2min, assume API server is healthy and we reset the backoff.
		backoffManager:         wait.NewExponentialBackoffManager(800*time.Millisecond, 30*time.Second, 2*time.Minute, 2.0, 1.0, realClock),
		initConnBackoffManager: wait.NewExponentialBackoffManager(800*time.Millisecond, 30*time.Second, 2*time.Minute, 2.0, 1.0, realClock),
		resyncPeriod:           resyncPeriod,
		clock:                  realClock,
		watchErrorHandler:      WatchErrorHandler(DefaultWatchErrorHandler),
	}
	r.setExpectedType(expectedType)
	return r
}
```

从源码中我们可以看出来通过 NewReflector 实例化反射器的时候，必须传入一个 ListerWatcher 接口对象，这个也是反射器最核心的功能，该接口拥有 List 和 Watch 方法，用于获取和监控资源对象。

```
// Lister is any object that knows how to perform an initial list.
// Lister 是知道如何执行初始化List列表的对象
type Lister interface {
	// List should return a list type object; the Items field will be extracted, and the
	// ResourceVersion field will be used to start the watch in the right place.
	// List 应该返回一个列表类型的对象；
  // Items 字段将被解析，ResourceVersion 字段将被用于正确启动 watch 的地方
	List(options metav1.ListOptions) (runtime.Object, error)
}

// Watcher is any object that knows how to start a watch on a resource.
// Watcher 是知道如何执行 watch 操作的任何对象
type Watcher interface {
	// Watch should begin a watch at the specified version.
	// Watch 在指定的版本开始执行 watch 操作
	Watch(options metav1.ListOptions) (watch.Interface, error)
}

// ListerWatcher is any object that knows how to perform an initial list and start a watch on a resource.
// ListerWatcher 是任何知道如何对一个资源执行初始化List列表和启动Watch监控操作的对象
type ListerWatcher interface {
	Lister
	Watcher
}
```

而 Reflector 对象通过 Run 函数来启动监控并处理监控事件的：

```
k8s.io\Kubernetes\staging\src\k8s.io\client-go\tools\cache\reflector.go
// Run 函数反复使用反射器的 ListAndWatch 函数来获取所有对象和后续的 deltas。
// 当 stopCh 被关闭的时候，Run函数才会退出。

func (r *Reflector) Run(stopCh <-chan struct{}) {
	klog.V(2).Infof("Starting reflector %s (%s) from %s", r.expectedTypeName, r.resyncPeriod, r.name)
	wait.BackoffUntil(func() {
		if err := r.ListAndWatch(stopCh); err != nil {
			r.watchErrorHandler(r, err)
		}
	}, r.backoffManager, true, stopCh)
	klog.V(2).Infof("Stopping reflector %s (%s) from %s", r.expectedTypeName, r.resyncPeriod, r.name)
}

```

所以不管我们传入的 ListWatcher 对象是如何实现的 List 和 Watch 操作，只要实现了就可以，最主要的还是看 ListAndWatch 函数是如何去实现的，如何去调用 List 和 Watch 的：

```
k8s.io\Kubernetes\staging\src\k8s.io\client-go\tools\cache\reflector.go
// ListAndWatch 函数首先列出所有的对象，并在调用的时候获得资源版本，然后使用该资源版本来进行 watch 操作。
// 如果 ListAndWatch 没有初始化 watch 成功就会返回错误。

func (r *Reflector) ListAndWatch(stopCh <-chan struct{}) error {
	klog.V(3).Infof("Listing and watching %v from %s", r.expectedTypeName, r.name)
	var resourceVersion string

	options := metav1.ListOptions{ResourceVersion: r.relistResourceVersion()}

	if err := func() error {
		initTrace := trace.New("Reflector ListAndWatch", trace.Field{"name", r.name})
		defer initTrace.LogIfLong(10 * time.Second)
		var list runtime.Object
		var paginatedResult bool
		var err error
		listCh := make(chan struct{}, 1)
		panicCh := make(chan interface{}, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicCh <- r
				}
			}()
			// Attempt to gather list in chunks, if supported by listerWatcher, if not, the first
			// list request will return the full response.
			// 如果 listWatcher 支持，会尝试 chunks（分块）收集 List 列表数据
      // 如果不支持，第一个 List 列表请求将返回完整的响应数据。
			pager := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
				return r.listerWatcher.List(opts)
			}))
			switch {
			case r.WatchListPageSize != 0:
				pager.PageSize = r.WatchListPageSize
			case r.paginatedResult:
				// We got a paginated result initially. Assume this resource and server honor
				// paging requests (i.e. watch cache is probably disabled) and leave the default
				// pager size set.
				// 获得一个初始的分页结果。假定此资源和服务器符合分页请求，并保留默认的分页器大小设置
			case options.ResourceVersion != "" && options.ResourceVersion != "0":
				// User didn't explicitly request pagination.
				//
				// With ResourceVersion != "", we have a possibility to list from watch cache,
				// but we do that (for ResourceVersion != "0") only if Limit is unset.
				// To avoid thundering herd on etcd (e.g. on master upgrades), we explicitly
				// switch off pagination to force listing from watch cache (if enabled).
				// With the existing semantic of RV (result is at least as fresh as provided RV),
				// this is correct and doesn't lead to going back in time.
				//
				// We also don't turn off pagination for ResourceVersion="0", since watch cache
				// is ignoring Limit in that case anyway, and if watch cache is not enabled
				// we don't introduce regression.
				pager.PageSize = 0
			}

			list, paginatedResult, err = pager.List(context.Background(), options)
			if isExpiredError(err) || isTooLargeResourceVersionError(err) {
				r.setIsLastSyncResourceVersionUnavailable(true)
				// Retry immediately if the resource version used to list is unavailable.
				// The pager already falls back to full list if paginated list calls fail due to an "Expired" error on
				// continuation pages, but the pager might not be enabled, the full list might fail because the
				// resource version it is listing at is expired or the cache may not yet be synced to the provided
				// resource version. So we need to fallback to resourceVersion="" in all to recover and ensure
				// the reflector makes forward progress.
				list, paginatedResult, err = pager.List(context.Background(), metav1.ListOptions{ResourceVersion: r.relistResourceVersion()})
			}
			close(listCh)
		}()
		select {
		case <-stopCh:
			return nil
		case r := <-panicCh:
			panic(r)
		case <-listCh:
		}
		if err != nil {
			return fmt.Errorf("failed to list %v: %v", r.expectedTypeName, err)
		}

		// We check if the list was paginated and if so set the paginatedResult based on that.
		// However, we want to do that only for the initial list (which is the only case
		// when we set ResourceVersion="0"). The reasoning behind it is that later, in some
		// situations we may force listing directly from etcd (by setting ResourceVersion="")
		// which will return paginated result, even if watch cache is enabled. However, in
		// that case, we still want to prefer sending requests to watch cache if possible.
		//
		// Paginated result returned for request with ResourceVersion="0" mean that watch
		// cache is disabled and there are a lot of objects of a given type. In such case,
		// there is no need to prefer listing from watch cache.
		if options.ResourceVersion == "0" && paginatedResult {
			r.paginatedResult = true
		}

		r.setIsLastSyncResourceVersionUnavailable(false) // list was successful
		initTrace.Step("Objects listed")
		listMetaInterface, err := meta.ListAccessor(list)
		if err != nil {
			return fmt.Errorf("unable to understand list result %#v: %v", list, err)
		}
		// 将资源对象列表中的资源对象和资源版本号存储在 Store 中
		resourceVersion = listMetaInterface.GetResourceVersion()
		initTrace.Step("Resource version extracted")
		items, err := meta.ExtractList(list)
		if err != nil {
			return fmt.Errorf("unable to understand list result %#v (%v)", list, err)
		}
		initTrace.Step("Objects extracted")
		if err := r.syncWith(items, resourceVersion); err != nil {
			return fmt.Errorf("unable to sync list result: %v", err)
		}
		initTrace.Step("SyncWith done")
		r.setLastSyncResourceVersion(resourceVersion)
		initTrace.Step("Resource version updated")
		return nil
	}(); err != nil {
		return err
	}

	resyncerrc := make(chan error, 1)
	cancelCh := make(chan struct{})
	defer close(cancelCh)
	go func() {
		resyncCh, cleanup := r.resyncChan()
		defer func() {
			cleanup() // Call the last one written into cleanup
		}()
		for {
			select {
			case <-resyncCh:
			case <-stopCh:
				return
			case <-cancelCh:
				return
			}
			// 如果 ShouldResync 为 nil 或者调用返回true，则执行 Store 的 Resync 操作
			if r.ShouldResync == nil || r.ShouldResync() {
				klog.V(4).Infof("%s: forcing resync", r.name)
				if err := r.store.Resync(); err != nil {
					resyncerrc <- err
					return
				}
			}
			cleanup()
			resyncCh, cleanup = r.resyncChan()
		}
	}()

	for {
		// give the stopCh a chance to stop the loop, even in case of continue statements further down on errors
		select {
		// stopCh 一个停止循环的机会
		case <-stopCh:
			return nil
		default:
		}

		timeoutSeconds := int64(minWatchTimeout.Seconds() * (rand.Float64() + 1.0))
		// 设置watch的选项，因为前期列举了全量对象，从这里只要监听最新版本以后的资源就可以了
    // 如果没有资源变化总不能一直挂着吧？也不知道是卡死了还是怎么了，所以设置一个超时会好一点
		options = metav1.ListOptions{
			ResourceVersion: resourceVersion,
			// We want to avoid situations of hanging watchers. Stop any wachers that do not
			// receive any events within the timeout window.
			TimeoutSeconds: &timeoutSeconds,
			// To reduce load on kube-apiserver on watch restarts, you may enable watch bookmarks.
			// Reflector doesn't assume bookmarks are returned at all (if the server do not support
			// watch bookmarks, it will ignore this field).
			AllowWatchBookmarks: true,
		}

		// start the clock before sending the request, since some proxies won't flush headers until after the first watch event is sent
		start := r.clock.Now()
		// 执行 Watch 操作
		w, err := r.listerWatcher.Watch(options)
		if err != nil {
			// If this is "connection refused" error, it means that most likely apiserver is not responsive.
			// It doesn't make sense to re-list all objects because most likely we will be able to restart
			// watch where we ended.
			// If that's the case begin exponentially backing off and resend watch request.
			if utilnet.IsConnectionRefused(err) {
				<-r.initConnBackoffManager.Backoff().C()
				continue
			}
			return err
		}
        // 调用 watchHandler 来处理分发 watch 到的事件对象
		if err := r.watchHandler(start, w, &resourceVersion, resyncerrc, stopCh); err != nil {
			if err != errorStopRequested {
				switch {
				case isExpiredError(err):
					// Don't set LastSyncResourceVersionUnavailable - LIST call with ResourceVersion=RV already
					// has a semantic that it returns data at least as fresh as provided RV.
					// So first try to LIST with setting RV to resource version of last observed object.
					klog.V(4).Infof("%s: watch of %v closed with: %v", r.name, r.expectedTypeName, err)
				default:
					klog.Warningf("%s: watch of %v ended with: %v", r.name, r.expectedTypeName, err)
				}
			}
			return nil
		}
	}
}
```

首先通过反射器的 relistResourceVersion 函数获得反射器 relist 的资源版本，如果资源版本非 0，则表示根据资源版本号继续获取，当传输过程中遇到网络故障或者其他原因导致中断，下次再连接时，会根据资源版本号继续传输未完成的部分。可以使本地缓存中的数据与Etcd集群中的数据保持一致，该函数实现如下所示：

```
k8s.io\Kubernetes\staging\src\k8s.io\client-go\tools\cache\reflector.go

// relistResourceVersion 决定了反射器应该list或者relist的资源版本。
// 如果最后一次relist的结果是HTTP 410（Gone）状态码，则返回""，这样relist将通过quorum读取etcd中可用的最新资源版本。
// 返回使用 lastSyncResourceVersion，这样反射器就不会使用在relist结果或watch事件中watch到的资源版本更老的资源版本进行relist了

func (r *Reflector) relistResourceVersion() string {
	r.lastSyncResourceVersionMutex.RLock()
	defer r.lastSyncResourceVersionMutex.RUnlock()

	if r.isLastSyncResourceVersionUnavailable {
		// Since this reflector makes paginated list requests, and all paginated list requests skip the watch cache
		// if the lastSyncResourceVersion is unavailable, we set ResourceVersion="" and list again to re-establish reflector
		// to the latest available ResourceVersion, using a consistent read from etcd.
		// 因为反射器会进行分页List请求，如果 lastSyncResourceVersion 过期了，所有的分页列表请求就都会跳过 watch 缓存
    // 所以设置 ResourceVersion=""，然后再次 List，重新建立反射器到最新的可用资源版本，从 etcd 中读取，保持一致性。
		return ""
	}
	if r.lastSyncResourceVersion == "" {
		// For performance reasons, initial list performed by reflector uses "0" as resource version to allow it to
		// be served from the watch cache if it is enabled.
		// 反射器执行的初始 List 操作的时候使用0作为资源版本。
		return "0"
	}
	return r.lastSyncResourceVersion
}
```

上面的 ListAndWatch 函数实现看上去虽然非常复杂，但其实大部分是对分页的各种情况进行处理，最核心的还是调用 r.listerWatcher.List(opts) 获取全量的资源对象，而这个 List 其实 ListerWatcher 实现的 List 方法，这个 ListerWatcher 接口实际上在该接口定义的同一个文件中就有一个 ListWatch 结构体实现了：

```
k8s.io\Kubernetes\staging\src\k8s.io\client-go\tools\cache\listwatch.go

// ListFunc 知道如何 List 资源
type ListFunc func(options metav1.ListOptions) (runtime.Object, error)

// WatchFunc 知道如何 watch 资源
type WatchFunc func(options metav1.ListOptions) (watch.Interface, error)

// ListWatch 结构体知道如何 list 和 watch 资源对象，它实现了 ListerWatcher 接口。
// 它为 NewReflector 使用者提供了方便的函数。其中 ListFunc 和 WatchFunc 不能为 nil。

// ListWatch 结构体知道如何 list 和 watch 资源对象，它实现了 ListerWatcher 接口。
// 它为 NewReflector 使用者提供了方便的函数。其中 ListFunc 和 WatchFunc 不能为 nil。

type ListWatch struct {
	ListFunc  ListFunc
	WatchFunc WatchFunc
	// DisableChunking requests no chunking for this list watcher.
	// DisableChunking 对 list watcher 请求不分块。
	DisableChunking bool
}

// 列出一组 APIServer 资源
func (lw *ListWatch) List(options metav1.ListOptions) (runtime.Object, error) {
	// ListWatch is used in Reflector, which already supports pagination.
	// Don't paginate here to avoid duplication.
	return lw.ListFunc(options)
}

// Watch 一组 APIServer 资源
func (lw *ListWatch) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return lw.WatchFunc(options)
}
```

当我们真正使用一个 Informer 对象的时候，实例化的时候就会调用这里的 ListWatch 来进行初始化，比如前面我们实例中使用的 Deployment Informer.

```
k8s.io\Kubernetes\staging\src\k8s.io\client-go\informers\apps\v1\deployment.go

// NewFilteredDeploymentInformer 为 Deployment 构造一个新的 Informer。
// 总是倾向于使用一个 informer 工厂来获取一个 shared informer，而不是获取一个独立的 informer，这样可以减少内存占用和服务器的连接数。

func NewFilteredDeploymentInformer(client kubernetes.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.AppsV1().Deployments(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.AppsV1().Deployments(namespace).Watch(context.TODO(), options)
			},
		},
		&appsv1.Deployment{},
		resyncPeriod,
		indexers,
	)
}

func (f *deploymentInformer) defaultInformer(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredDeploymentInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *deploymentInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&appsv1.Deployment{}, f.defaultInformer)
}
```

从上面代码我们就可以看出来当我们去调用一个资源对象的 Informer() 的时候，就会去调用上面的 `NewFilteredDeploymentInformer` 函数进行初始化，而在初始化的使用就传入了 `cache.ListWatch` 对象，其中就有 List 和 Watch 的实现操作，也就是说前面反射器在 ListAndWatch 里面调用的 ListWatcher 的 List 操作是在一个具体的资源对象的 Informer 中实现的，比如我们这里就是通过的 ClientSet 客户端与 APIServer 交互获取到 Deployment 的资源列表数据的，通过在 ListFunc 中的 `client.AppsV1().Deployments(namespace).List(context.TODO(), options)` 实现的，这下应该好理解了吧。

获取到了全量的 List 数据过后，通过 `listMetaInterface.GetResourceVersion()` 来获取资源的版本号，ResourceVersion（资源版本号）非常重要，Kubernetes 中所有的资源都拥有该字段，它标识当前资源对象的版本号，每次修改（CUD）当前资源对象时，Kubernetes API Server 都会更改 ResourceVersion，这样 client-go 执行 Watch 操作时可以根据ResourceVersion 来确定当前资源对象是否发生了变化。

然后通过 meta.ExtractList 函数将资源数据转换成资源对象列表，将 `runtime.Object` 对象转换成 `[]runtime.Object` 对象，因为全量获取的是一个资源列表。

接下来是通过反射器的 `syncWith` 函数将资源对象列表中的资源对象和资源版本号存储在 Store 中，这个会在后面的章节中详细说明。

最后处理完成后通过 `r.setLastSyncResourceVersion(resourceVersion)` 操作来设置最新的资源版本号，其他的就是启动一个 goroutine 去定期检查是否需要执行 Resync 操作，调用存储中的 `r.store.Resync()` 来执行，关于存储的实现在后面和大家进行讲解。

紧接着就是 Watch 操作了，Watch 操作通过 HTTP 协议与 APIServer 建立长连接，接收Kubernetes API Server 发来的资源变更事件，和 List 操作一样，Watch 的真正实现也是具体的 Informer 初始化的时候传入的，比如上面的 Deployment Informer 中初始化的时候传入的 WatchFunc，底层也是通过 ClientSet 客户端对 Deployment 执行 Watch 操作 `client.AppsV1().Deployments(namespace).Watch(context.TODO(), options)` 实现的。

获得 watch 的资源数据后，通过调用 `r.watchHandler` 来处理资源的变更事件，当触发Add 事件、Update 事件、Delete 事件时，将对应的资源对象更新到本地缓存（DeltaFIFO）中并更新 ResourceVersion 资源版本号。

```
k8s.io\Kubernetes\staging\src\k8s.io\client-go\tools\cache\reflector.go

// watchHandler 监听 w 保持资源版本最新

func (r *Reflector) watchHandler(start time.Time, w watch.Interface, resourceVersion *string, errc chan error, stopCh <-chan struct{}) error {
	eventCount := 0

	// Stopping the watcher should be idempotent and if we return from this function there's no way
	// we're coming back in with the same watch interface.
	defer w.Stop()

loop:
	for {
		select {
		case <-stopCh:
			return errorStopRequested
		case err := <-errc:
			return err
		case event, ok := <-w.ResultChan():  // 从 watch 中获取事件对象
			if !ok {
				break loop
			}
			if event.Type == watch.Error {
				return apierrors.FromObject(event.Object)
			}
			if r.expectedType != nil {
				if e, a := r.expectedType, reflect.TypeOf(event.Object); e != a {
					utilruntime.HandleError(fmt.Errorf("%s: expected type %v, but watch event object had type %v", r.name, e, a))
					continue
				}
			}
			if r.expectedGVK != nil {
				if e, a := *r.expectedGVK, event.Object.GetObjectKind().GroupVersionKind(); e != a {
					utilruntime.HandleError(fmt.Errorf("%s: expected gvk %v, but watch event object had gvk %v", r.name, e, a))
					continue
				}
			}
			meta, err := meta.Accessor(event.Object)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("%s: unable to understand watch event %#v", r.name, event))
				continue
			}
			// 获得当前 watch 到资源的资源版本号
			newResourceVersion := meta.GetResourceVersion()
			switch event.Type { // 分发事件
			case watch.Added: //  Add 事件
				err := r.store.Add(event.Object)
				if err != nil {
					utilruntime.HandleError(fmt.Errorf("%s: unable to add watch event object (%#v) to store: %v", r.name, event.Object, err))
				}
			case watch.Modified: // Update 事件
				err := r.store.Update(event.Object)
				if err != nil {
					utilruntime.HandleError(fmt.Errorf("%s: unable to update watch event object (%#v) to store: %v", r.name, event.Object, err))
				}
			case watch.Deleted: // Delete 事件
				// TODO: Will any consumers need access to the "last known
				// state", which is passed in event.Object? If so, may need
				// to change this.
				err := r.store.Delete(event.Object)
				if err != nil {
					utilruntime.HandleError(fmt.Errorf("%s: unable to delete watch event object (%#v) from store: %v", r.name, event.Object, err))
				}
			case watch.Bookmark:
			// `Bookmark` 意味着 watch 已经同步到这里了，只要更新资源版本即可。
				// A `Bookmark` means watch has synced here, just update the resourceVersion
			default:
				utilruntime.HandleError(fmt.Errorf("%s: unable to understand watch event %#v", r.name, event))
			}
			// 更新资源版本号
			*resourceVersion = newResourceVersion
			r.setLastSyncResourceVersion(newResourceVersion)
			if rvu, ok := r.store.(ResourceVersionUpdater); ok {
				rvu.UpdateResourceVersion(newResourceVersion)
			}
			eventCount++
		}
	}

	watchDuration := r.clock.Since(start)
	if watchDuration < 1*time.Second && eventCount == 0 {
		return fmt.Errorf("very short watch: %s: Unexpected watch close - watch lasted less than a second and no items received", r.name)
	}
	klog.V(4).Infof("%s: Watch close - %v total %v items received", r.name, r.expectedTypeName, eventCount)
	return nil
}
```

这就是 Reflector 反射器中最核心的 ListAndWatch 实现，从上面的实现我们可以看出获取到的数据最终都流向了本地的 Store，也就是 DeltaFIFO，所以接下来我们需要来分析 DeltaFIFO 的实现。