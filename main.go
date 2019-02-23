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
	"time"

	flags "github.com/jessevdk/go-flags"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog"
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

var namespace = metav1.NamespaceSystem

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

	klog.Infof("Start kubernetes dependencies version:%v, build at:%v", phVersion, phBuildDate)

	args := Options{
		MaxRetry:    "always",
		IgnoreError: false,
		Sleep:       "10s",
		Timeout:     "300s",
	}

	_, err := flags.ParseArgs(&args, arguments)

	if err != nil {
		if err.(*flags.Error).Type == flags.ErrHelp {
			return 0
		}

		klog.Errorf("Failed %v", err)

		return -1
	}

	cc, err := buildConfigFromEnvs(args.Apiserver, args.Kubeconfig)

	if err != nil {
		klog.Errorf("Failed to make client: %v", err)
	}

	client, err := clientset.NewForConfig(cc)

	if err != nil {
		klog.Errorf("Failed to make client: %v", err)
		return -1
	}

	maxRetry := args.getMaxRetry()
	timeout := args.getTimeout()
	sleep := args.getSleepTime()

	if args.Namespace != "" {
		if _, err := client.Core().Namespaces().Get(args.Namespace, metav1.GetOptions{}); err != nil {
			klog.Errorf("%s namespace doesn't exist: %v", args.Namespace, err)
			return -1
		}

		namespace = args.Namespace
	}

	dependencies := makeDependencyList(maxRetry, args.Dep.Dependencies, args.IgnoreError)

	var ready bool

	dependencies.isValid(client)

	// Look for endpoints associated with the Elasticsearch logging service.
	// First wait for the service to become available.
	for t := time.Now(); time.Since(t) < timeout; time.Sleep(sleep) {
		ready, err = dependencies.ready(client, args.IgnoreError, args.KeepOnError, args.Verbose)

		if err != nil && !args.IgnoreError {
			klog.Errorf("Failed to got ready: %v", err)
			return -1
		}

		if ready {
			break
		}
	}

	if !ready {
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
