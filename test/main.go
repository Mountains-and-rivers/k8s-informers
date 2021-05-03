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
		fmt.Print("3333333333333333333333333333")
		panic(err.Error())
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE")
}
