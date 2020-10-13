chmod +x measurement_checkout_var.sh
chmod +x measurement_checkout_const.sh
LOADS=(5 10 15 20 25 30)
DURATION=900
./measurement_checkout_var.sh experiments/checkout-var 900 25
for LOAD in "${LOADS[@]}"
do
	read -n 1 -p Continue?	# just hit enter to continue next experiment
	./measurement_checkout_const.sh experiments/checkout-$LOAD $DURATION $LOAD
done
