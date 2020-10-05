chmod +x measurement_checkout_var.sh
chmod +x measurement_checkout_const.sh
LOADS=(5 10 15 20 25 30)
DURATION=300
./measurement_checkout_var.sh experiments/checkout-var 600 25
for LOAD in "${LOADS[@]}"
do
	./measurement_checkout_const.sh experiments/checkout-$LOAD $DURATION $LOAD
done
