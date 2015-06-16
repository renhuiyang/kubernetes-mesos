### Intro to Kubernetes Usage

Once Kubernetes is running and healthy you can interact with it via REST API or using the kubectl command line tool.

- [Configure the Kubernetes Endpoint](#config)
- [Create a Kubernetes Pod Definition](#pod)
- [Create a Kubernetes Service Definition](#service)
- [Launch a Kubernetes Pod via REST API](#api)
- [Launch a Kubernetes Pod via kubectl](#kubectl)
- [View Usage Metrics](#metrics)

<a name="config"></a>
#### Configure the Kubernetes Endpoint

Export the Kubernetes URL to an environment variable, where `<hostname>` is the Mesos Master hostname:

```
export KUBERNETES_MASTER=http://<hostname>/service/kubernetes/api
```

<a name="pod"></a>
#### Create a Kubernetes Pod Definition

A [pod](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/pods.md) is one or more containers that get co-located on the same host. In this example we’re creating a pod with one container, which runs nginx.

1. Download an example nginx pod definition:

    ```
    mkdir -p examples && curl -o examples/pod-nginx.json https://raw.githubusercontent.com/mesosphere/kubernetes-mesos/master/examples/pod-nginx.json
    ```

<a name="service"></a>
#### Create a Kubernetes Service Definition

A [service inside kubernetes](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/services.md) is an abstraction that gives a pod (or an external service) a named endpoint.
In order to expose the nginx server to the web, you must create a Kubernetes service that refers to it.

1. Download an example nginx service definition:

    ```
    mkdir -p examples && curl -o examples/service-nginx.json https://raw.githubusercontent.com/mesosphere/kubernetes-mesos/master/examples/service-nginx.json
    ```

<a name="api"></a>
#### Launch a Kubernetes Pod via REST API

1. Create the nginx pod:

    ```
    curl $KUBERNETES_MASTER/v1beta3/namespaces/default/pods -XPOST -H'Content-type: json' -d@examples/pod-nginx.json
    ```

2. List all created Kubernetes pods:

    ```
    curl $KUBERNETES_MASTER/v1beta3/namespaces/default/pods
    ```

3. Create the nginx service:

    ```
    curl $KUBERNETES_MASTER/v1beta3/namespaces/default/services -XPOST -H'Content-type: json' -d@examples/service-nginx.json
    ```

4. List all created Kubernetes services:

    ```
    curl $KUBERNETES_MASTER/v1beta3/namespaces/default/services
    ```

5. Use the service through the API server proxy (requires kube-proxy):

    ```
    curl $KUBERNETES_MASTER/v1beta3/proxy/namespaces/default/services/nginx
    ```

6. Delete the nginx service:

    ```
    curl $KUBERNETES_MASTER/v1beta3/namespaces/default/services/nginx -XDELETE
    ```

7. Delete the nginx pod:

    ```
    curl $KUBERNETES_MASTER/v1beta3/namespaces/default/pods/nginx-id-01 -XDELETE
    ```

<a name="kubectl"></a>
#### Launch a Kubernetes Pod via kubectl

1. Download or build kubectl:
    - Build the latest binary from the Kubernetes repo:

        ```
        git clone https://github.com/GoogleCloudPlatform/kubernetes
        cd kubernetes
        export KUBERNETES_CONTRIB=mesos
        make
        cd ..
        ln -s kubernetes/_output/local/bin/$OS/$ARCH/kubectl ..
        ```

    - Download the latest binary from the [kubernetes-mesos v0.5.0](https://github.com/mesosphere/kubernetes-mesos/tree/v0.5.0) release page on github:
        - Mac OS X

            ```
            curl -L https://github.com/mesosphere/kubernetes-mesos/releases/download/v0.5.0/kubectl-v0.5.0-darwin-amd64.tgz | tar -xz
            ```

        - Linux

            ```
            curl -L https://github.com/mesosphere/kubernetes-mesos/releases/download/v0.5.0/kubectl-v0.5.0-linux-amd64.tgz | tar -xz
            ```

2. Add the current directory (where kubectl is) to your PATH:

    ```
    export PATH=$(pwd):$PATH
    ```

3. Create the nginx pod:

    ```
    kubectl create -f examples/pod-nginx.json
    ```

4. List all created Kubernetes pods:

    ```
    kubectl get pods
    ```

5. Create the nginx service:

    ```
    kubectl create -f examples/service-nginx.json
    ```

6. List all created Kubernetes services:

    ```
    kubectl get services
    ```

7. Use the service through the API server proxy:

    ```
    curl $KUBERNETES_MASTER/v1beta3/proxy/namespaces/default/services/nginx
    ```

8. Delete the nginx service:

    ```
    kubectl delete service nginx
    ```

9. Delete the nginx pod:

    ```
    kubectl delete pod nginx-id-01
    ```

For more on how to use kubectl, see the [kubectl documentation](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/kubectl.md).

<a name="metrics"></a>
#### View Usage Metrics

Kubernetes API Server usage metrics are available at `<hostname>/service/kubernetes/metrics`.