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
	"context"
	"strings"

	clientset "k8s.io/client-go/kubernetes"
	klog "k8s.io/klog/v2"
)

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

func (t *DependencyList) isValid(ctx context.Context, client *clientset.Clientset) {
	for _, depend := range t.dependencies {
		depend.isValid(ctx, client)
	}
}

func (t *DependencyList) ready(ctx context.Context, client *clientset.Clientset, ignoreError bool, keepOnerror bool, verbose bool) (bool, error) {
	dependencies := make([]*Dependency, len(t.dependencies))

	copy(dependencies, t.dependencies)

	// Create an empty slice
	t.dependencies = []*Dependency{}

	for _, depend := range dependencies {
		ready, err := depend.ready(ctx, client, verbose)

		if err != nil {
			// Insure...
			ready = false

			if verbose {
				klog.Infof("%v dependency got an error:%v", depend, err)
			}

			if !keepOnerror || depend.retry() <= 0 {
				t.errdependencies = append(t.errdependencies, depend)

				if !ignoreError {
					return false, err
				}
			} else if verbose {
				klog.Infof("Will retry %v dependency", depend.String())
			}
		} else if ready {
			klog.Infof("The dependency %v is ready", depend.String())
		} else {
			if verbose {
				klog.Infof("The dependency %v is not ready", depend.String())
			}

			t.dependencies = append(t.dependencies, depend)
		}
	}

	if len(t.dependencies) == 0 {
		return true, nil
	}

	return false, nil
}
