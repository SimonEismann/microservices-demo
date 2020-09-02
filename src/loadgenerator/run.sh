#!/bin/sh

# delay start to let system boot up
sleep 180
# start the actual loadgenerator and the loadgen director with a specified .lua load script.
java -jar httploadgenerator.jar loadgenerator --user-id-file userids.txt & java -jar httploadgenerator.jar director --ip localhost --load load.csv -o result.csv --lua load.lua -t 1
cat result.csv
while true; do sleep 10; done	# keep the container from rebooting
