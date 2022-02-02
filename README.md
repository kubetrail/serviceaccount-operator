# serviceaccount-operator
Kubernetes operator to provision new service account tokens
that can be rotated and deleted with grace periods

## installation
first download the code, build container image and push
to your container registry.
> please make sure go toolchain and docker are installed
> at relatively newer versions and also update the
> IMG value to point to your registry
```bash
export IMG=docker.io/your-account-name/serviceaccount-operator:0.0.1
make generate
make manifests
make docker-build
make docker-push
```
once the container image is available in your registry you can
deploy the controller.
> please make sure you have cert-manager and prometheus running
> on your cluster

install `cert-manager`
```bash
kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.6.1/cert-manager.yaml
```

install `prometheus` after creating namespace for it and making sure
your `helm` repos are updated
```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm --namespace=prometheus-system upgrade --install \
                prometheus prometheus-community/kube-prometheus-stack \
                --set=grafana.enabled=false \
                --version=27.0.1
```

install `CRD's` and the controller
```bash
make install
make deploy
```

Make sure everything is running properly:
```bash
kubectl --namespace=serviceaccount-operator-system get pods,svc,configmaps,secrets,servicemonitors
NAME                                                              READY   STATUS    RESTARTS       AGE
pod/serviceaccount-operator-controller-manager-56767956d4-5gz96   2/2     Running   14 (27m ago)   20h

NAME                                                                 TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)    AGE
service/serviceaccount-operator-controller-manager-metrics-service   ClusterIP   00.000.00.000   <none>        8443/TCP   20h
service/serviceaccount-operator-webhook-service                      ClusterIP   00.000.00.000   <none>        443/TCP    20h

NAME                                               DATA   AGE
configmap/6e1ce403.kubetrail.io                    0      20h
configmap/kube-root-ca.crt                         1      20h
configmap/serviceaccount-operator-manager-config   1      20h

NAME                                                            TYPE                                  DATA   AGE
secret/artifact-registry-key                                    kubernetes.io/dockerconfigjson        1      20h
secret/default-token-9vd9c                                      kubernetes.io/service-account-token   3      20h
secret/serviceaccount-operator-controller-manager-token-rqj54   kubernetes.io/service-account-token   3      20h
secret/webhook-server-cert                                      kubernetes.io/tls                     3      20h

NAME                                                                                              AGE
servicemonitor.monitoring.coreos.com/serviceaccount-operator-controller-manager-metrics-monitor   20h
```

## create tokens
Token below is created for service account `default` that will be rotated
every 3000 seconds and then deleted 600 seconds after rotation
```yaml
apiVersion: serviceaccount.kubetrail.io/v1beta1
kind: Token
metadata:
  name: token-sample
spec:
  serviceAccountName: default
  rotationPeriodSeconds: 3000
  deletionGracePeriodSeconds: 600
```

The associated secret name can be found in the status:
```bash
kubectl get tokens.serviceaccount.kubetrail.io token-sample -o=jsonpath='{.status.secretName}
```
