chmod +x measurement_checkout_var.sh
chmod +x measurement_checkout_const.sh
RUNS=(1 2 3)
LOADS=(5 10 15 20 25)
DURATION=900
./measurement_checkout_var.sh experiments/checkout-var 900 25
for RUN in "${RUNS[@]}"
do
	for LOAD in "${LOADS[@]}"
	do
		# read -n 1 -p "Continue with run ${RUN}-${LOAD}?"	# just hit enter to continue next experiment
		./measurement_checkout_const.sh experiments/checkout-${RUN}-${LOAD} $DURATION $LOAD
	done
done
