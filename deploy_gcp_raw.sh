export PROJECT_ID=`gcloud config get-value project`
export ZONE=us-central1-a
export CLUSTER_NAME=${PROJECT_ID}-1
export MACHINE_TYPE=n1-standard-1		# smallest machine

services=(adservice cartservice checkoutservice currencyservice emailservice frontend paymentservice prodcatservice recommservice shippingservice zipkin)

gcloud container clusters create $CLUSTER_NAME --min-nodes=${#services[@]} --max-nodes=${#services[@]} --num-nodes=${#services[@]} --zone $ZONE --machine-type=${MACHINE_TYPE}

nodes_string=`kubectl get nodes | grep -vP '^NAME' | grep -oP '^[\w\-0-9]+'`
readarray -t nodes <<< "$nodes_string"

for index in "${!services[@]}"; do kubectl label nodes ${nodes[index]} service=${services[index]}; done		# label each node for a specific service

kubectl apply -k ./kubernetes-manifests		# deploys without loadgen

kubectl get pods -o wide	# show deployment of pods for verification