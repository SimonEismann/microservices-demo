export PROJECT_ID=`gcloud config get-value project`
export ZONE=us-central1-a
export CLUSTER_NAME=${PROJECT_ID}-1
export MACHINE_TYPE=n1-standard-1		# smallest machine

services=(adservice cartservice checkoutservice currencyservice emailservice frontend paymentservice prodcatservice recommservice shippingservice zipkin)

# gcloud services enable container.googleapis.com
# gcloud container clusters create $CLUSTER_NAME --enable-autoupgrade --enable-autoscaling --min-nodes=3 --max-nodes=10 --num-nodes=5 --zone $ZONE

gcloud container clusters create $CLUSTER_NAME --min-nodes=${#services[@]} --max-nodes=${#services[@]} --num-nodes=${#services[@]} --zone $ZONE --machine-type=${MACHINE_TYPE}

nodes=(kubectl get nodes | grep -vP '^NAME' | grep -oP '^[\w\-0-9]+')

for index in "${!services[@]}"; do kubectl label nodes ${nodes[index]} service=${services[index]}; done		# label each node for a specific service

# gcloud services enable containerregistry.googleapis.com
# gcloud auth configure-docker -q
# gcloud container clusters get-credentials $CLUSTER_NAME --zone $ZONE --project $PROJECT_ID
# kubectl create clusterrolebinding cluster-admin-binding --clusterrole=cluster-admin --user=$(gcloud config get-value core/account)

kubectl apply -k ./kubernetes-manifests		# deploys without loadgen