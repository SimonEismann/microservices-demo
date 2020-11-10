chmod +x measurement_checkout_var.sh
chmod +x measurement_checkout_const.sh
RUNS=(1 2 3)
LOADS=(5 10 15 20 25 30 35)
DURATION=600
./measurement_checkout_var.sh experiments/checkout-var $DURATION 35
for RUN in "${RUNS[@]}"
do
	for LOAD in "${LOADS[@]}"
	do
		# read -n 1 -p "Continue with run ${RUN}-${LOAD}?"	# just hit enter to continue next experiment
		./measurement_checkout_const.sh experiments/checkout-${RUN}-${LOAD} $DURATION $LOAD
	done
done
cd experiments
python3 util/overview_to_table.py > table.txt
cd ..