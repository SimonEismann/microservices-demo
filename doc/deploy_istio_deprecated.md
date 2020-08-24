```shell
# install files (only needed once)
curl -L https://istio.io/downloadIstio | sh - # download newest istio release
git clone https://github.com/SimonEismann/microservices-demo se-microservices-demo
```
```shell
# execute
cd istio-1.6.5 # version may change! check after installation!
export PATH=$PWD/bin:$PATH
export PROJECT_ID=`gcloud config get-value project`
export ZONE=us-central1-a
export CLUSTER_NAME=${PROJECT_ID}-1
gcloud services enable container.googleapis.com
gcloud container clusters create $CLUSTER_NAME --enable-autoupgrade --enable-autoscaling --min-nodes=3 --max-nodes=10 --num-nodes=5 --zone $ZONE
gcloud services enable containerregistry.googleapis.com
gcloud auth configure-docker -q
gcloud container clusters get-credentials $CLUSTER_NAME --zone $ZONE --project $PROJECT_ID
kubectl create clusterrolebinding cluster-admin-binding --clusterrole=cluster-admin --user=$(gcloud config get-value core/account)
istioctl install --set profile=demo
cd ..
cd se-microservices-demo/
kubectl label namespace default istio-injection=enabled
kubectl apply -f ./istio-manifests
kubectl apply -f ./kubernetes-manifests
istioctl analyze
INGRESS_HOST="$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')"
echo "$INGRESS_HOST"	# website can be accessed by local browser at this address
```

## Kiali Dashboard
Opens a web dashboard with live traffic observation and further inspection functions.
```shell
istioctl dashboard kiali # username=admin, password=admin, kiali graph filter: hide -> name*=whitelist OR name*=Passthrough
```