# usage: 	./execute_measurement_checkout_only.sh $EXPERIMENT_NAME $LOAD_DURATION $LOAD_INTENSITY $ITEMS_PER_CART
# example: 	./execute_measurement_checkout_only.sh experiments/checkout 300 10 7

EXPERIMENT_NAME=$1			# acts as the directory path to store related files to
LOAD_DURATION=$2 			# in seconds
LOAD_INTENSITY=$3			# requests per second
ITEMS_PER_CART=$4			# avg of a normal distribution, stddev=ITEMS_PER_CART/9

rm -rf $EXPERIMENT_NAME
mkdir -p $EXPERIMENT_NAME

USER_AMOUNT=$(($LOAD_DURATION * $LOAD_INTENSITY))
UTIL_FILE_PATH="${EXPERIMENT_NAME}/util_results.txt"
USER_ID_FILE="${EXPERIMENT_NAME}/userids.txt"
LOAD="${EXPERIMENT_NAME}/load.csv"
LOAD_SCRIPT="${EXPERIMENT_NAME}/load.lua"
LOAD_RESULT="loadgen_result.csv"
NODE_MAP="${EXPERIMENT_NAME}/nodemap.txt"
OVERVIEW="${EXPERIMENT_NAME}/overview.txt"

export PROJECT_ID=`gcloud config get-value project`
export ZONE=us-central1-a
export CLUSTER_NAME=${PROJECT_ID}-1
export MACHINE_TYPE=n1-standard-2
services=(adservice cartservice checkoutservice currencyservice emailservice frontend paymentservice productcatalogservice recommendationservice shippingservice zipkin)
gcloud container clusters create $CLUSTER_NAME --min-nodes=${#services[@]} --max-nodes=${#services[@]} --num-nodes=${#services[@]} --zone $ZONE --machine-type=${MACHINE_TYPE} --no-enable-autoupgrade
nodes_string=`kubectl get nodes | grep -vP '^NAME' | grep -oP '^[\w\-0-9]+'`
readarray -t nodes <<< "$nodes_string"
rm -f $NODE_MAP
touch $NODE_MAP
for index in "${!services[@]}"
do 
	kubectl label nodes ${nodes[index]} service=${services[index]}
	printf "${services[index]},${nodes[index]}\n" >> $NODE_MAP
done
kubectl apply -f ./kubernetes-manifests-checkout-only	# deploys specially prepared delays
kubectl get pods -o wide	# show deployment of pods for verification

echo "waiting for system to boot up... (3 minutes)"
sleep 180
REDIS_ADDR="$(kubectl -n default get service redis-cart -o jsonpath='{.status.loadBalancer.ingress[0].ip}'):6379"
FRONTEND_ADDR="$(kubectl -n default get service frontend -o jsonpath='{.status.loadBalancer.ingress[0].ip}'):8080"
echo "populating cart data base with ${USER_AMOUNT} carts..."
cd util/cart-populator
go run populator.go $REDIS_ADDR $USER_AMOUNT $ITEMS_PER_CART
cd ../..
echo "generate config files for loadgenerator..."
# generate user id file
rm -f $USER_ID_FILE
touch $USER_ID_FILE
for ((n=100000000;n<$(($USER_AMOUNT + 100000000));n++))
do
	printf "$n\n" >> $USER_ID_FILE
done
# generate load.csv
rm -f $LOAD
touch $LOAD
for ((n=1;n<=$LOAD_DURATION;n++))
do
	timestamp=$((n - 1)).5
	printf "$timestamp,$LOAD_INTENSITY\n" >> $LOAD
done
# checkout only lua script
rm -f $LOAD_SCRIPT
touch $LOAD_SCRIPT
printf "frontendIP = \"http://${FRONTEND_ADDR}\"\nfunction onCycle(id_new_user)\n\tuserId = id_new_user\nend\nfunction frontend_cart_checkout(user_id)\n\treturn \"[POST]{user_id=\"..user_id..\"&email=someone%%40example.com&street_address=1600+Amphitheatre+Parkway&zip_code=94043&city=Mountain+View&state=CA&country=United+States&credit_card_number=4432-8015-6152-0454&credit_card_expiration_month=1&credit_card_expiration_year=2021&credit_card_cvv=672}\"..frontendIP..\"/cart/checkout\"\nend\nfunction onCall(callnum)\n\tif (callnum == 1) then\n\t\treturn frontend_cart_checkout(userId)\n\telse\n\t\treturn nil\n\tend\nend" > $LOAD_SCRIPT
echo "starting load generator..."
pkill -f 'java -jar'
java -jar src/loadgenerator/httploadgenerator.jar loadgenerator --user-id-file $USER_ID_FILE & java -jar src/loadgenerator/httploadgenerator.jar director --ip localhost --load $LOAD -o $LOAD_RESULT --lua $LOAD_SCRIPT -t 200
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