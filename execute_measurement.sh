ITEMS_PER_CART=10	# TODO: as parameter
LOAD_DURATION=300 	# in seconds
LOAD_INTENSITY=5	# requests per second
EXPERIMENT_NAME="experiments/test"	# acts as the directory path to store related files to
THREADS=1

mkdir -p $EXPERIMENT_NAME

USER_AMOUNT=$(($LOAD_DURATION * $LOAD_INTENSITY))
UTIL_FILE_PATH="${EXPERIMENT_NAME}/util_results.txt"
USER_ID_FILE="${EXPERIMENT_NAME}/userids.txt"
LOAD="${EXPERIMENT_NAME}/load.csv"
LOAD_SCRIPT="${EXPERIMENT_NAME}/load.lua"
LOAD_RESULT="loadgen_result.csv"

chmod +x deploy_gcp_raw.sh
./deploy_gcp_raw.sh
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
printf "frontendIP = \"${FRONTEND_ADDR}\"\nfunction onCycle(id_new_user)\n\tuserId = id_new_user\nend\nfunction frontend_cart_checkout(user_id)\n\treturn \"[POST]{user_id=\"..user_id..\"&email=someone\%40example.com&street_address=1600+Amphitheatre+Parkway&zip_code=94043&city=Mountain+View&state=CA&country=United+States&credit_card_number=4432-8015-6152-0454&credit_card_expiration_month=1&credit_card_expiration_year=2021&credit_card_cvv=672}\"..frontendIP..\"/cart/checkout\"\nend\nfunction onCall(callnum)\n\tif (callnum == 1) then\n\t\treturn frontend_cart_checkout(userId)\n\telse\n\t\treturn nil\n\tend\nend" > $LOAD_SCRIPT
echo "starting load generator..."
pkill -f 'java -jar'
java -jar src/loadgenerator/httploadgenerator.jar loadgenerator --user-id-file $USER_ID_FILE & java -jar src/loadgenerator/httploadgenerator.jar director --ip localhost --load $LOAD -o $LOAD_RESULT --lua $LOAD_SCRIPT -t $THREADS
pkill -f 'java -jar'
echo "saving stackdriver utilization logs to ${UTIL_FILE_PATH}"
cd util/utilization-exporter
go run exporter.go `gcloud config get-value project` $(($LOAD_DURATION + 10)) > ../../$UTIL_FILE_PATH
cd ../..