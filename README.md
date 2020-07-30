# Hipster Shop: Cloud-Native Microservices Demo Application
 
This repo is a fork of http://go/microservices-demo with modifications to suite the Tetrate Service Bridge.

## building images
Images are built automatically using a Github Action.
They are published in Docker Hub in the `microservicesdemomesh` registry.

You can build the images using the scripts located in the `hack` folder:

```
# build only the image of emailservice
TAG=v0.1.8 REPO_PREFIX=my.docker.hub ./hack/make-docker-images-nopush.sh emailservice

# build all images locally (no push)
TAG=v0.1.8 REPO_PREFIX=my.docker.hub ./hack/make-docker-images-nopush.sh

# build all and push to Docker Registry
TAG=v0.1.8 REPO_PREFIX=my.docker.hub ./hack/make-docker-images.sh
```

# Deployment in Google Cloud Shell
```shell
# install files
curl -L https://istio.io/downloadIstio | sh - # download newest istio release
git clone https://github.com/SimonEismann/microservices-demo se-microservices-demo

# execute
cd istio-1.6.5 # version may change!
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
echo "$INGRESS_HOST"

# dashboard
istioctl dashboard kiali # username=admin, password=admin, kiali graph filter: hide -> name*=whitelist OR name*=Passthrough
```
