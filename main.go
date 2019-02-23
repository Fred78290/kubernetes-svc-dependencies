/*
Copyright 2019 Fred78290.

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
	"os"
	"strconv"
	"time"

	flags "github.com/jessevdk/go-flags"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

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

	os.Exit(mainExitCode(arguments))
}
