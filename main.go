/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jessevdk/go-flags"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog"
	core "k8s.io/kubernetes/pkg/apis/core"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

var phVersion = "v0.0.0-unset"
var phBuildDate = ""

// MaxUint MaxUint
const MaxUint = ^uint(0)

// MinUint MinUint
const MinUint = 0

// MaxInt MaxInt
const MaxInt = int(MaxUint >> 1)

// MinInt MinInt
const MinInt = -MaxInt - 1

var namespace string

// Dependency a k8s depency
type Dependency struct {
	_kind      string
	_namespace string
	_name      string
	_retry     int
}

func makeDependency(maxRetry int, depend string, ignoreError bool) *Dependency {
	v := strings.Split(depend, "/")

	if len(v) == 2 {
		n := strings.Split(v[1], ":")

		if len(n) > 1 {
			return &Dependency{v[0], n[0], n[1], maxRetry}
		}

		return &Dependency{v[0], namespace, n[1], maxRetry}
	}

	if ignoreError == false {
		klog.Fatalf("Unable to parse dependency: %v", depend)
	}
	klog.Warningf("Unable to parse dependency: %v, ignoring", depend)

	return nil
}

func (t *Dependency) String() string {
	return t._kind + "/" + t._namespace + ":" + t._name
}

func (t *Dependency) isValid(client *clientset.Clientset) {

	switch t._kind {
	case "po", "deploy", "ds", "rc", "rs", "sts", "svc":
	default:
		klog.Fatalf("Unknown resource type %v", t._kind)
	}

	if t._namespace == "" {
		klog.Fatalf("Namespace not defined for dependency %v", t._name)
	}

	namespace, err := client.Core().Namespaces().Get(t._namespace, metav1.GetOptions{})

	if err != nil || namespace == nil {
		klog.Fatalf("Namespace %v doesn't exists", t._namespace)
	}
}

func (t *Dependency) podReady(pod *core.Pod, verbose bool) (bool, error) {
	ready := false
	numOfContainer := len(pod.Status.ContainerStatuses)
	numOfReady := 0

	if pod.Status.Phase == core.PodRunning {
		ready = true
		for _, container := range pod.Status.ContainerStatuses {
			if container.Ready && container.State.Running != nil {
				numOfReady++
			}
		}
	}

	return numOfContainer == numOfReady && ready, nil
}

func (t *Dependency) ready(client *clientset.Clientset, verbose bool) (bool, error) {

	if verbose {
		klog.Infof("Check if %v dependency is ready, retry:%d", t.String(), t._retry)
	}

	if t._retry != MaxInt {
		t._retry--
	}

	options := metav1.GetOptions{}

	if t._retry >= 0 {
		if t._kind == "po" {

			if pod, err := client.Core().Pods(t._namespace).Get(t._name, options); err != nil {
				return false, err
			} else if pod == nil {
				return false, fmt.Errorf("The pod %v doesn't exists", t)
			} else {
				return t.podReady(pod, verbose)
			}

		} else if t._kind == "deploy" {

			if deployment, err := client.Apps().Deployments(t._namespace).Get(t._name, options); err != nil {
				return false, err
			} else if deployment == nil {
				return false, fmt.Errorf("The deployment %v doesn't exists", t)
			} else {
				return deployment.Status.Replicas == deployment.Status.ReadyReplicas, err
			}

		} else if t._kind == "ds" {

			if daemonset, err := client.Apps().DaemonSets(t._namespace).Get(t._name, options); err != nil {
				return false, err
			} else if daemonset == nil {
				return false, fmt.Errorf("The daemonset %v doesn't exists", t)
			} else {
				return daemonset.Status.NumberAvailable == daemonset.Status.NumberReady, nil
			}

		} else if t._kind == "rs" {

			if replicaset, err := client.Apps().ReplicaSets(t._namespace).Get(t._name, options); err != nil {
				return false, err
			} else if replicaset == nil {
				return false, fmt.Errorf("The replicaset %v doesn't exists", t)
			} else {
				return replicaset.Status.Replicas == replicaset.Status.ReadyReplicas, nil
			}

		} else if t._kind == "rc" {

			if replicationcontroller, err := client.Core().ReplicationControllers(t._namespace).Get(t._name, options); err != nil {
				return false, err
			} else if replicationcontroller == nil {
				return false, fmt.Errorf("The replicationcontroller %v doesn't exists", t)
			} else {
				return replicationcontroller.Status.Replicas == replicationcontroller.Status.ReadyReplicas, nil
			}

		} else if t._kind == "sts" {

			if stateful, err := client.Apps().StatefulSets(t._namespace).Get(t._name, options); err != nil {
				return false, err
			} else if stateful == nil {
				return false, fmt.Errorf("The stateful %v doesn't exists", t)
			} else {
				return stateful.Status.Replicas == stateful.Status.ReadyReplicas, nil
			}

		} else if t._kind == "svc" {
			if service, err := client.Core().Services(t._namespace).Get(t._name, options); err != nil {
				return false, err
			} else if service == nil {
				return false, fmt.Errorf("The service %v doesn't exists", t)
			} else {
				set := labels.Set(service.Spec.Selector)

				if pods, err := client.Core().Pods(t._namespace).List(metav1.ListOptions{LabelSelector: set.String()}); err == nil {
					numOfReady := 0
					for _, pod := range pods.Items {
						if verbose {
							klog.Infof("Check service %v, pod:%v status:%v", t, pod.Name, pod.Status.Phase)
						}
						ready, err := t.podReady(&pod, verbose)

						if err != nil {
							return false, err
						} else if ready {
							numOfReady++
							if verbose {
								klog.Infof("Service %v, pod:%v is ready", t, pod.Name)
							}
						} else if verbose {
							klog.Infof("Service %v, pod:%v not ready", t, pod.Name)
						}
					}

					return numOfReady == len(pods.Items), nil
				}

				return false, err
			}
		}
	}

	return false, nil
}

func (t *Dependency) retry() int {
	return t._retry
}

func (t *Dependency) kind() string {
	return t._kind
}

func (t *Dependency) namespace() string {
	return t._namespace
}

func (t *Dependency) name() string {
	return t._name
}

// DependencyList contains all dependencies
type DependencyList struct {
	dependencies    []*Dependency
	errdependencies []*Dependency
}

func makeDependencyList(maxRetry int, depends []string, ignoreError bool) *DependencyList {
	d := &DependencyList{}

	d.set(maxRetry, depends, ignoreError)

	return d
}

func (t *DependencyList) set(maxRetry int, depends []string, ignoreError bool) {
	for _, ss := range depends {
		ss = strings.TrimSpace(ss)
		if ss != "" {
			depend := makeDependency(maxRetry, ss, ignoreError)
			if depend != nil {
				t.dependencies = append(t.dependencies, depend)
			}
		}
	}
}

func (t *DependencyList) isValid(client *clientset.Clientset) {
	for _, depend := range t.dependencies {
		depend.isValid(client)
	}
}

func (t *DependencyList) ready(client *clientset.Clientset, ignoreError bool, keepOnerror bool, verbose bool) (bool, error) {
	dependencies := make([]*Dependency, len(t.dependencies))

	copy(dependencies, t.dependencies)

	// Create an empty slice
	t.dependencies = []*Dependency{}

	for _, depend := range dependencies {
		ready, err := depend.ready(client, verbose)

		if err != nil {
			// Insure...
			ready = false

			if verbose {
				klog.Infof("%v dependency got an error:%v", depend, err)
			}

			if keepOnerror == false || depend.retry() <= 0 {
				t.errdependencies = append(t.errdependencies, depend)

				if ignoreError == false {
					return false, err
				}
			} else if verbose {
				klog.Infof("Will retry %v dependency", depend.String())
			}
		} else if ready {
			klog.Infof("The dependency %v is ready", depend.String())
		} else if verbose {
			klog.Infof("The dependency %v is not ready", depend.String())
		}

		if ready == false {
			t.dependencies = append(t.dependencies, depend)
		}
	}

	if len(t.dependencies) == 0 {
		return true, nil
	}

	return false, nil
}

func buildConfigFromEnvs(masterURL, kubeconfigPath string) (*restclient.Config, error) {
	if kubeconfigPath == "" && masterURL == "" {
		kubeconfig, err := restclient.InClusterConfig()
		if err != nil {
			return nil, err
		}

		return kubeconfig, nil
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{ClusterInfo: clientapi.Cluster{Server: masterURL}}).ClientConfig()
}

// Options arguments
type Options struct {
	Namespace   string                          `short:"n" long:"namespace" description:"Default namespace"`
	Kubeconfig  string                          `short:"k" long:"kubeconfig" description:"Kubeconfig file"`
	Apiserver   string                          `short:"a" long:"apiserver" description:"apiserver host"`
	MaxRetry    string                          `short:"r" long:"maxretry" description:"[always|The number of retry before a depency is considered as unready]"`
	KeepOnError bool                            `short:"b" long:"keeponerror" description:"Try always to reach the dependency"`
	IgnoreError bool                            `short:"i" long:"ignoreerror" description:"Ignore error"`
	Verbose     bool                            `short:"v" long:"verbose" description:"Verbose"`
	Sleep       string                          `short:"s" long:"sleep" description:"Time interval in time.Duration unit"`
	Timeout     string                          `short:"t" long:"timeout" description:"Time to wait before to declare service down in time.Duration unit"`
	Dep         struct{ Dependencies []string } `positional-args:"yes" required:"1" positional-arg-name:"dependency" description:"Enumeration of dependency service"`
}

func mainExitCode(arguments []string) int {

	var args Options
	var maxRetry int

	args.MaxRetry = "always"
	args.IgnoreError = false
	args.Sleep = "10s"
	args.Timeout = "300s"

	klog.Infof("Start kubernetes dependencies version:%v, build at:%v", phVersion, phBuildDate)

	_, err := flags.ParseArgs(&args, arguments)

	if err != nil {
		if err.(*flags.Error).Type == flags.ErrHelp {
			return 0
		}
	}

	if err != nil {
		klog.Errorf("Failed %v", err)
		return -1
	}

	cc, err := buildConfigFromEnvs(args.Apiserver, args.Kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to make client: %v", err)
	}
	client, err := clientset.NewForConfig(cc)

	if err != nil {
		klog.Fatalf("Failed to make client: %v", err)
	}

	if args.MaxRetry == "always" {
		maxRetry = MaxInt
	} else if args.MaxRetry == "" {
		maxRetry = 1
	} else {
		maxRetry, err = strconv.Atoi(args.MaxRetry)
		if err != nil {
			klog.Errorf("Unable to parse maxretry value:%v", args.MaxRetry)
			return -1
		}
	}

	timeout, err := time.ParseDuration(args.Timeout)
	if err != nil {
		klog.Errorf("Unable to parse timeout value:%v", args.Timeout)
		return -1
	}

	sleep, err := time.ParseDuration(args.Sleep)
	if err != nil {
		klog.Errorf("Unable to parse sleep value:%v", args.Sleep)
		return -1
	}

	namespace = metav1.NamespaceSystem
	envNamespace := args.Namespace

	if envNamespace != "" {
		if _, err := client.Core().Namespaces().Get(envNamespace, metav1.GetOptions{}); err != nil {
			klog.Fatalf("%s namespace doesn't exist: %v", envNamespace, err)
		}
		namespace = envNamespace
	}

	dependencies := makeDependencyList(maxRetry, args.Dep.Dependencies, args.IgnoreError)
	var ready bool

	dependencies.isValid(client)

	// Look for endpoints associated with the Elasticsearch logging service.
	// First wait for the service to become available.
	for t := time.Now(); time.Since(t) < timeout; time.Sleep(sleep) {
		ready, err = dependencies.ready(client, args.IgnoreError, args.KeepOnError, args.Verbose)

		if err != nil && args.IgnoreError == false {
			klog.Errorf("Failed to got ready: %v", err)
			return -1
		}

		if ready {
			break
		}
	}

	if ready == false {
		klog.Errorf("Failed to got ready dependencies: %v", dependencies.dependencies)

		return -1
	}

	klog.Info("All dependencies are ready")

	return 0
}

func main() {
	arguments := os.Args[1:]

	flag.CommandLine.Parse([]string{"-logtostderr=true"})

	os.Exit(mainExitCode(arguments))
}
