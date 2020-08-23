#!/bin/sh

# delay start to let system boot up
sleep 120
# start the actual loadgenerator and the loadgen director with a specified .lua load script.
java -jar httploadgenerator.jar loadgenerator & java -cp load.jar:httploadgenerator.jar tools.descartes.dlim.httploadgenerator.runner.Main director --ip localhost --load load.csv -o result.csv --lua load.lua -t 256 -c measurment.ProcListener
cat result.csv
while true; do sleep 10; done	# keep the container from rebooting
