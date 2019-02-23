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
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
	core "k8s.io/kubernetes/pkg/apis/core"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

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
			return &Dependency{
				_kind:      v[0],
				_namespace: n[0],
				_name:      n[1],
				_retry:     maxRetry,
			}
		}

		return &Dependency{
			_kind:      v[0],
			_namespace: namespace,
			_name:      n[1],
			_retry:     maxRetry,
		}
	}

	if !ignoreError {
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

func (t *Dependency) isPodReady(client *clientset.Clientset, verbose bool) (bool, error) {
	if pod, err := client.Core().Pods(t._namespace).Get(t._name, metav1.GetOptions{}); err != nil {
		return false, err
	} else if pod == nil {
		return false, fmt.Errorf("The pod %v doesn't exists", t)
	} else {
		return t.podReady(pod, verbose)
	}
}

func (t *Dependency) isDeploymentReady(client *clientset.Clientset, verbose bool) (bool, error) {
	if deployment, err := client.Apps().Deployments(t._namespace).Get(t._name, metav1.GetOptions{}); err != nil {
		return false, err
	} else if deployment == nil {
		return false, fmt.Errorf("The deployment %v doesn't exists", t)
	} else {
		return deployment.Status.Replicas == deployment.Status.ReadyReplicas, err
	}
}

func (t *Dependency) isDaemonSetReady(client *clientset.Clientset, verbose bool) (bool, error) {
	if daemonset, err := client.Apps().DaemonSets(t._namespace).Get(t._name, metav1.GetOptions{}); err != nil {
		return false, err
	} else if daemonset == nil {
		return false, fmt.Errorf("The daemonset %v doesn't exists", t)
	} else {
		return daemonset.Status.NumberAvailable == daemonset.Status.NumberReady, nil
	}
}

func (t *Dependency) isReplicaSetReady(client *clientset.Clientset, verbose bool) (bool, error) {
	if replicaset, err := client.Apps().ReplicaSets(t._namespace).Get(t._name, metav1.GetOptions{}); err != nil {
		return false, err
	} else if replicaset == nil {
		return false, fmt.Errorf("The replicaset %v doesn't exists", t)
	} else {
		return replicaset.Status.Replicas == replicaset.Status.ReadyReplicas, nil
	}
}

func (t *Dependency) isReplicationControllerReady(client *clientset.Clientset, verbose bool) (bool, error) {
	if replicationcontroller, err := client.Core().ReplicationControllers(t._namespace).Get(t._name, metav1.GetOptions{}); err != nil {
		return false, err
	} else if replicationcontroller == nil {
		return false, fmt.Errorf("The replicationcontroller %v doesn't exists", t)
	} else {
		return replicationcontroller.Status.Replicas == replicationcontroller.Status.ReadyReplicas, nil
	}
}

func (t *Dependency) isStatefulSetsReady(client *clientset.Clientset, verbose bool) (bool, error) {
	if stateful, err := client.Apps().StatefulSets(t._namespace).Get(t._name, metav1.GetOptions{}); err != nil {
		return false, err
	} else if stateful == nil {
		return false, fmt.Errorf("The stateful %v doesn't exists", t)
	} else {
		return stateful.Status.Replicas == stateful.Status.ReadyReplicas, nil
	}
}

func (t *Dependency) isServiceReady(client *clientset.Clientset, verbose bool) (bool, error) {
	var service *core.Service
	var pods *core.PodList
	var err error
	var ready bool
	var numOfReady int

	if service, err = client.Core().Services(t._namespace).Get(t._name, metav1.GetOptions{}); err != nil {
		return false, err
	}

	if service == nil {
		return false, fmt.Errorf("The service %v doesn't exists", t)
	}

	set := labels.Set(service.Spec.Selector)

	if pods, err = client.Core().Pods(t._namespace).List(metav1.ListOptions{LabelSelector: set.String()}); err != nil {
		return false, err
	}

	for _, pod := range pods.Items {
		if verbose {
			klog.Infof("Check service %v, pod:%v status:%v", t, pod.Name, pod.Status.Phase)
		}

		if ready, err = t.podReady(&pod, verbose); err != nil {
			return false, err
		}

		if ready {
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

func (t *Dependency) ready(client *clientset.Clientset, verbose bool) (bool, error) {

	if verbose {
		klog.Infof("Check if %v dependency is ready, retry:%d", t.String(), t._retry)
	}

	if t._retry != MaxInt {
		t._retry--
	}

	if t._retry > 0 {
		switch t._kind {
		case "po":
			return t.isPodReady(client, verbose)
		case "deploy":
			return t.isDeploymentReady(client, verbose)
		case "ds":
			return t.isDaemonSetReady(client, verbose)
		case "rs":
			return t.isReplicaSetReady(client, verbose)
		case "rc":
			return t.isReplicationControllerReady(client, verbose)
		case "sts":
			return t.isStatefulSetsReady(client, verbose)
		case "svc":
			return t.isServiceReady(client, verbose)
		}
	}

	return false, fmt.Errorf("Max retries reached for %v", t)
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
