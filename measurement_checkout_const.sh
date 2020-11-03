# usage: 	./measurement_checkout_const.sh $EXPERIMENT_NAME $LOAD_DURATION $LOAD_INTENSITY
# example: 	./measurement_checkout_const.sh experiments/checkout 300 20

# Open port 22442 for all VM instances to external!

EXPERIMENT_NAME=$1			# acts as the directory path to store related files to
LOAD_DURATION=$2 			# in seconds
LOAD_INTENSITY=$3			# requests per second

rm -rf $EXPERIMENT_NAME
mkdir -p $EXPERIMENT_NAME/training_data

USER_AMOUNT=$(($LOAD_DURATION * $LOAD_INTENSITY))
USER_ID_FILE="${EXPERIMENT_NAME}/userids.txt"
LOAD="${EXPERIMENT_NAME}/load.csv"
LOAD_SCRIPT="${EXPERIMENT_NAME}/load.lua"
LOAD_RESULT="loadgen_result.csv"
NODE_MAP="${EXPERIMENT_NAME}/nodemap.txt"
OVERVIEW="${EXPERIMENT_NAME}/overview.txt"

export PROJECT_ID=`gcloud config get-value project`
export ZONE=us-central1-a
export CLUSTER_NAME=${PROJECT_ID}-1
export MACHINE_TYPE=n1-standard-4
services=(adservice cartservice checkoutservice currencyservice emailservice frontend paymentservice productcatalogservice recommendationservice shippingservice zipkin)
gcloud container clusters create $CLUSTER_NAME --min-nodes=${#services[@]} --max-nodes=${#services[@]} --num-nodes=${#services[@]} --zone $ZONE --machine-type=${MACHINE_TYPE} --no-enable-autoupgrade
nodes_string=`kubectl get nodes | grep -vP '^NAME' | grep -oP '^[\w\-0-9]+'`
readarray -t nodes <<< "$nodes_string"
IP_LIST=()
rm -f $NODE_MAP
touch $NODE_MAP
for index in "${!services[@]}"
do 
	kubectl label nodes ${nodes[index]} service=${services[index]}	# label the nodes to specific services
	gcloud compute scp util/utilization-servlet/utilization-servlet ${nodes[index]}:~ --zone=$ZONE --quiet	# copy and start our utilization measurement tool, port: 22442
	gcloud compute ssh ${nodes[index]} --zone=$ZONE --quiet --command="chmod +x utilization-servlet; sudo mount -o remount,rw,exec /home; nohup ~/utilization-servlet > /dev/null 2>&1 &"
	NODE_IP="$(gcloud compute instances describe ${nodes[index]} --zone=${ZONE} --format='get(networkInterfaces[0].accessConfigs[0].natIP)')"
	IP_LIST+=($NODE_IP)
	printf "${services[index]},${nodes[index]},${NODE_IP}\n" >> $NODE_MAP
done
kubectl apply -f ./kubernetes-manifests-checkout-only	# deploys specially prepared delays
kubectl get pods -o wide	# show deployment of pods for verification

echo "waiting for system to boot up... (3 minutes)"
sleep 180
REDIS_ADDR="$(kubectl -n default get service redis-cart -o jsonpath='{.status.loadBalancer.ingress[0].ip}'):6379"
FRONTEND_ADDR="$(kubectl -n default get service frontend -o jsonpath='{.status.loadBalancer.ingress[0].ip}'):8080"
echo "populating cart data base with ${USER_AMOUNT} carts..."
cd util/cart-populator-1
go run populator.go $REDIS_ADDR $USER_AMOUNT
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
LMDAEMON_PORT="22442"
IP_STRING=""
for ip in "${IP_LIST[@]}"; do IP_STRING+="${ip}:${LMDAEMON_PORT}," ; done
java -jar src/loadgenerator/httploadgenerator.jar loadgenerator --user-id-file $USER_ID_FILE & java -jar src/loadgenerator/httploadgenerator.jar director --ip localhost --load $LOAD -o $LOAD_RESULT --lua $LOAD_SCRIPT -t 300 -p="${IP_STRING::${#IP_STRING}-1}" --timeout=10000 > ${EXPERIMENT_NAME}/loaddriverlogs.txt
pkill -f 'java -jar'
echo "wait 2 minutes for zipkin data to settle..."
sleep 120
echo "save MySQL dump to csv..."
MYSQL_ADDR="$(kubectl -n default get service mysql -o jsonpath='{.status.loadBalancer.ingress[0].ip}')"
for tb in $(mysql --protocol=tcp --host=${MYSQL_ADDR} -pzipkin -uzipkin zipkin -sN -e "SHOW TABLES;"); do
    mysql -B --protocol=tcp --host=${MYSQL_ADDR} -pzipkin -uzipkin zipkin -e "SELECT * FROM ${tb};" | sed "s/\"/\"\"/g;s/'/\'/;s/\t/\",\"/g;s/^/\"/;s/$/\"/;s/\n//g" > ${EXPERIMENT_NAME}/${tb}.csv;
done
python3 util/experiment_data_overview.py $EXPERIMENT_NAME export=true > $OVERVIEW
# parse to training data from logs
python3 util/parse_training_data.py ${EXPERIMENT_NAME}/training_data cartservice.csv hipstershop.cartservice.emptycart.csv,hipstershop.cartservice.getcart.csv
python3 util/parse_training_data.py ${EXPERIMENT_NAME}/training_data checkoutservice.csv hipstershop.checkoutservice.placeorder.csv
python3 util/parse_training_data.py ${EXPERIMENT_NAME}/training_data currencyservice.csv hipstershop.currencyservice.convert.csv
python3 util/parse_training_data.py ${EXPERIMENT_NAME}/training_data emailservice.csv recv.hipstershop.emailservice.sendorderconfirmation.csv
python3 util/parse_training_data.py ${EXPERIMENT_NAME}/training_data frontend.csv cartcheckout.csv
python3 util/parse_training_data.py ${EXPERIMENT_NAME}/training_data paymentservice.csv hipstershop.paymentservice.charge.csv
python3 util/parse_training_data.py ${EXPERIMENT_NAME}/training_data productcatalogservice.csv hipstershop.productcatalogservice.getproduct.csv,hipstershop.productcatalogservice.listproducts.csv
python3 util/parse_training_data.py ${EXPERIMENT_NAME}/training_data recommendationservice.csv recv.hipstershop.recommendationservice.listrecommendations.csv
python3 util/parse_training_data.py ${EXPERIMENT_NAME}/training_data shippingservice.csv hipstershop.shippingservice.shiporder.csv,hipstershop.shippingservice.getquote.csv
echo "finished measurement successfully! All data can be found in ${EXPERIMENT_NAME}."
gcloud container clusters delete $CLUSTER_NAME --zone=$ZONE --quiet