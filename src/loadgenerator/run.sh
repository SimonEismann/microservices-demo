#!/bin/sh

# delay start to let system boot up
sleep 120
# start the actual loadgenerator
nohup java -jar httploadgenerator.jar loadgenerator > /dev/null 2>&1 &
# start the loadgen director with a specified .lua load script.
java -cp load.jar:httploadgenerator.jar tools.descartes.dlim.httploadgenerator.runner.Main director --ip localhost --load load.csv -o result.csv --lua load.lua -t 256 -c measurment.ProcListener
while true; do sleep 10; done
