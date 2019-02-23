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
	"strconv"
	"time"

	"k8s.io/klog"
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

func (args *Options) getMaxRetry() int {
	var maxRetry int
	var err error

	if args.MaxRetry == "always" {
		maxRetry = MaxInt
	} else if args.MaxRetry == "" {
		maxRetry = 1
	} else if maxRetry, err = strconv.Atoi(args.MaxRetry); err != nil {
		klog.Fatalf("Unable to parse maxretry value:%v", args.MaxRetry)
	}

	return maxRetry
}

func (args *Options) getTimeout() time.Duration {
	timeout, err := time.ParseDuration(args.Timeout)
	if err != nil {
		klog.Fatalf("Unable to parse timeout value:%v", args.Timeout)
	}

	return timeout
}

func (args *Options) getSleepTime() time.Duration {
	sleep, err := time.ParseDuration(args.Sleep)
	if err != nil {
		klog.Fatalf("Unable to parse sleep value:%v", args.Sleep)
	}

	return sleep
}
