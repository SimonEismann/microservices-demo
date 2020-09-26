chmod +x measurement_checkout_var.sh
chmod +x measurement_checkout_const.sh
LOADS=(10 20 30 40 50 60)
DURATION=300
./measurement_checkout_var.sh experiments/checkout-var $DURATION 50
for LOAD in "${LOADS[@]}"
do
	./measurement_checkout_const.sh experiments/checkout-$LOAD $DURATION $LOAD
done
