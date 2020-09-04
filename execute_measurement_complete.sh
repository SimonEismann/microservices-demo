# usage: 	./execute_measurement.sh $EXPERIMENT_NAME $LOAD_DURATION $LOAD_INTENSITY
# example: 	./execute_measurement.sh experiments/complete 300 5

EXPERIMENT_NAME=$1			# acts as the directory path to store related files to
LOAD_DURATION=$2 			# in seconds
LOAD_INTENSITY=$3			# requests per second

mkdir -p $EXPERIMENT_NAME

UTIL_FILE_PATH="${EXPERIMENT_NAME}/util_results.txt"
LOAD="${EXPERIMENT_NAME}/load.csv"
LOAD_SCRIPT="${EXPERIMENT_NAME}/load.lua"
LOAD_RESULT="loadgen_result.csv"
NODE_MAP="${EXPERIMENT_NAME}/nodemap.txt"
OVERVIEW="${EXPERIMENT_NAME}/overview.txt"

export PROJECT_ID=`gcloud config get-value project`
export ZONE=us-central1-a
export CLUSTER_NAME=${PROJECT_ID}-1
export MACHINE_TYPE=n1-standard-1
services=(adservice cartservice checkoutservice currencyservice emailservice frontend paymentservice prodcatservice recommservice shippingservice zipkin)
gcloud container clusters create $CLUSTER_NAME --min-nodes=${#services[@]} --max-nodes=${#services[@]} --num-nodes=${#services[@]} --zone $ZONE --machine-type=${MACHINE_TYPE}
nodes_string=`kubectl get nodes | grep -vP '^NAME' | grep -oP '^[\w\-0-9]+'`
readarray -t nodes <<< "$nodes_string"
rm -f $NODE_MAP
touch $NODE_MAP
for index in "${!services[@]}"
do 
	kubectl label nodes ${nodes[index]} service=${services[index]}
	printf "${services[index]},${nodes[index]}\n" >> $NODE_MAP
done
kubectl apply -k ./kubernetes-manifests		# deploys without loadgen
kubectl get pods -o wide	# show deployment of pods for verification

echo "waiting for system to boot up... (3 minutes)"
sleep 180
FRONTEND_ADDR="$(kubectl -n default get service frontend -o jsonpath='{.status.loadBalancer.ingress[0].ip}'):8080"
echo "generate config files for loadgenerator..."
# generate load.csv
rm -f $LOAD
touch $LOAD
for ((n=1;n<=$LOAD_DURATION;n++))
do
	timestamp=$((n - 1)).5
	printf "$timestamp,$LOAD_INTENSITY\n" >> $LOAD
done
# complete lua script
rm -f $LOAD_SCRIPT
cat src/loadgenerator/example_load.lua | sed 's/frontend:8080/${FRONTEND_ADDR}/g' > $LOAD_SCRIPT
echo "starting load generator..."
pkill -f 'java -jar'
java -jar src/loadgenerator/httploadgenerator.jar loadgenerator & java -jar src/loadgenerator/httploadgenerator.jar director --ip localhost --load $LOAD -o $LOAD_RESULT --lua $LOAD_SCRIPT -t $LOAD_INTENSITY
pkill -f 'java -jar'
echo "saving stackdriver utilization logs to ${UTIL_FILE_PATH}"
cd util/utilization-exporter
go run exporter.go $PROJECT_ID $(($LOAD_DURATION + 10)) > ../../$UTIL_FILE_PATH
cd ../..
echo "wait 2 minutes for zipkin data to settle..."
sleep 120
echo "save MySQL dump to csv..."
MYSQL_ADDR="$(kubectl -n default get service mysql -o jsonpath='{.status.loadBalancer.ingress[0].ip}')"
for tb in $(mysql --protocol=tcp --host=${MYSQL_ADDR} -pzipkin -uzipkin zipkin -sN -e "SHOW TABLES;"); do
    mysql -B --protocol=tcp --host=${MYSQL_ADDR} -pzipkin -uzipkin zipkin -e "SELECT * FROM ${tb};" | sed "s/\"/\"\"/g;s/'/\'/;s/\t/\",\"/g;s/^/\"/;s/$/\"/;s/\n//g" > ${EXPERIMENT_NAME}/${tb}.csv;
done
python3 util/experiment_data_overview.py $EXPERIMENT_NAME > $OVERVIEW
echo "finished measurement successfully! All data can be found in ${EXPERIMENT_NAME}."