# Online Boutique: Cloud-Native Microservices Demo Application
This repo is a fork of Tetrate's modified version of the GCP Hipstershop/Online Boutique with modifications to allow for in-depth Zipkin tracing. Changes include:
- Native 100% tracing of every service with Zipkin with respect to parent and child spans.
- Deployment with pre-built Zipkin and MySQL instances to allow for fast data generation and extraction.
- Rewrite of `adservice`, `cartservice`, `currencyservice` and `paymentservice` in Go. Usage of another [loadgenerator](https://github.com/SimonEismann/HTTP-Load-Generator).
- Lots of smaller fixes.
- Ready to use deployment and data extraction scripts. All services are deployed to separate nodes.
- Artificial delays (constant workload) with matrix multiplication for all microservices.

# Overview
The following picture shows the connection graph of the services as defined by Tetrate. We reimplemented `cartservice`, `currencyservice` and `paymentservice` in **Go**. We do not build or deploy `apiservice` in our scripts and setup. Our `adservice` implementation works with grpc (again). The ingress gateway is no longer necessary to access the webpage.
![Overview Image Coarse](/doc/overview_tetrate.svg)
Here is our (slightly) updated and more detailed service architecture, which also shows the functionalities of the services:
![Overview Image Detailed](/doc/overview_detail.svg)

# Artificial Delays
Artificial delays can be activated by setting the `DELAY_*` environment variables in the `.yaml` deployment files in the `kubernetes-manifests` folder. The variables (64-bit signed) have to be set to positive integers to activate the feature.
The delay variable describes the amount of matrix (random values, constant size) multiplications computed before the actual task.

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
Either execute `deploy_gcp_raw.sh` or `deploy_gcp_loadgen.sh` from the repository root to deploy the system. A deprecated Istio setup script can be found in the `doc` folder for reference.

## MySQL dump to CSV
Install a MySQL client:
```shell
sudo apt-get install mariadb-client
```
Then create a shell script, which takes the hostname (or IP) of the mysql host as a parameter:
```shell
HOST=$1
mkdir dump
for tb in $(mysql --protocol=tcp --host=${HOST} -pzipkin -uzipkin zipkin -sN -e "SHOW TABLES;"); do
    mysql -B --protocol=tcp --host=${HOST} -pzipkin -uzipkin zipkin -e "SELECT * FROM ${tb};" | sed "s/\"/\"\"/g;s/'/\'/;s/\t/\",\"/g;s/^/\"/;s/$/\"/;s/\n//g" > dump/${tb}.csv;
done
```