# Hipster Shop: Cloud-Native Microservices Demo Application
This repo is a fork of Tetrate's modified version of Google's http://go/microservices-demo with modifications to allow for in-depth Zipkin tracing. Changes include:
- Native 100% tracing of every service with Zipkin with respect to parent and child spans.
- Deployment with pre-built Zipkin and MySQL instances to allow for fast data generation and extraction.
- Rewrite of `adservice` and `cartservice` in Go.
- Lots of smaller fixes.
- Ready to use deployment (with Istio) and data extraction scripts.

# Overview
The following picture shows the connection graph of the services as defined by Tetrate. We reimplemented `cartservice` in Go. We do not build or deploy `apiservice` in our scripts and setup.
![Overview Image Coarse](/doc/overview_tetrate.svg)
Here is our updated and more detailed service architecture, which also shows the functionalities of the services:
![Overview Image Detailed](/doc/overview_detail.svg)

# Building Images
Images are built automatically using a Github Action.
They are published in Docker Hub in the `simoneismann` registry.

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

## MySQL dump to CSV
Install a MySQL client:
```shell
sudo apt-get install mariadb-client
```
Then create a shell script, which takes the hostname (or IP) as a parameter:
```shell
HOST=$1
mkdir dump
for tb in $(mysql --protocol=tcp --host=${HOST} -pzipkin -uzipkin zipkin -sN -e "SHOW TABLES;"); do
    mysql -B --protocol=tcp --host=${HOST} -pzipkin -uzipkin zipkin -e "SELECT * FROM ${tb};" | sed "s/\"/\"\"/g;s/'/\'/;s/\t/\",\"/g;s/^/\"/;s/$/\"/;s/\n//g" > dump/${tb}.csv;
done
```