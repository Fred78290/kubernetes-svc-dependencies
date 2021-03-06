[![Build Status](https://github.com/fred78290/kubernetes-svc-dependencies/actions/workflows/ci.yml/badge.svg)](https://github.com/Fred78290/kubernetes-svc-dependencies/actions)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Fred78290_kubernetes-svc-dependencies&metric=alert_status)](https://sonarcloud.io/dashboard?id=Fred78290_kubernetes-svc-dependencies)
[![Licence](https://img.shields.io/hexpm/l/plug.svg)](https://github.com/Fred78290/kubernetes-svc-dependencies/blob/master/LICENSE)

# Kubernetes service dependencies ready checker #

when you create resources in kubernetes often yours containers failed to start because they need somes already running resources before starting. For example you deploy a php website and it must connect to a MySQL server. With kubernetes service dependencies embeded in the deployment, the container doesn't start until his all dependencies are ready

### How it work ###

kubernetes service dependencies use "initContainers" mechanism of kubernetes. You can see the sample [examples/helloworld.json](examples/helloworld.json).

This sample is helloworld waiting a ready mongodb instance before to start.

### Releases ###

* 1.13.3 (unmaintened)
    - This version is supported kubernetes v1.13.3
* 1.15.0 (unmaintened)
    - This version is supported kubernetes v1.15.0
* 1.16 (unmaintened)
    - This version is supported kubernetes v1.15
* 1.17.5 (unmaintened)
    - This version is supported kubernetes v1.17
* 1.18.2 (unmaintened)
    - This version is supported kubernetes v1.18
* 1.20.5
    - This version is supported kubernetes v1.20
* 1.21.0
    - This version is supported kubernetes v1.21


### How to use? ###

When you create kubernetes resources, just add an init container belong to your container.

```json
"spec": {
    "initContainers": [
        {
            "image": "fred78290/kubernetes-svc-dependencies:v1.20.5",
            "command": [
                "/usr/local/bin/check-dependencies"
            ],
            "args": [
                "-v",
                "--namespace=kube-public",
                "--kubeconfig=/etc/kubernetes/config",
                "--maxretry=5",
                "--sleep=15",
                "--timeout=300",
                "svc/kube-public:mongodb"
            ],
            "name": "helloworld-init",
            "volumeMounts": [
                {
                    "name": "kubeconfig",
                    "mountPath": "/etc/kubernetes/config",
                    "readOnly": true
                }
            ]
        }
    ],
    "containers": [
        ...
    ],
    "volumes": [
        {
            "name": "kubeconfig",
            "hostPath": {
                "path": "/etc/kubernetes/admin.conf",
                "type": "File"
            }
        }
    ]
}
```

### Command line arguments ###

| Name | Description |
| --- | --- |
| `-n \| --namespace` | Default namespace |
| `-k \| --kubeconfig` | Kubeconfig file  |
| `-a \| --apiserver` | apiserver host  |
| `-r \| --maxretry` | The number of retry before a dependency is considered as unready  |
| `-b \| --keeponerror` | Try always to reach the dependency else the process exit  |
| `-i \| --ignoreerror` | Ignore error  |
| `-v \| --verbose` | Verbose  |
| `-s \| --sleep` | Time interval in `time.Duration` unit  |
| `-t \| --timeout` | Time to wait before to declare service down in `time.Duration` unit  |
| `dependencies` | Enumeration of dependencies |

### Syntax to enumerate dependencies ###

The dependency is composed of 3 parts: type of kubernetes resource, namespace and resource name. The syntax is < resource >/< namespace >:< name >

| Resource | Description |Example |
| --- | --- | --- |
| `po` | Pod |`po/kube-public:mongodb-023a4` |
| `deploy` | Deployment | `deploy/kube-public:mongodb` |
| `ds` | DaemonSet | `ds/kube-public:mongodb` |
| `rs` | Replicaset | `rs/kube-public:mongodb` |
| `rc` | Replication controller | `rc/kube-public:mongodb` |
| `sts` | Statefulset | `sts/kube-public:mongodb` |
| `svc` | Service | `svc/kube-public:mongodb` |

## Build ##

To build the docker image, enter `make container`
