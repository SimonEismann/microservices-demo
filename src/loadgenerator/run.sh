# start the actual loadgenerator
nohup java -jar httploadgenerator.jar loadgenerator > /dev/null 2>&1 &
# start the loadgen director with a specified .lua load script. wd=warmup duration (seconds), wp=warmup pause (after duration, seconds), wr=warmup rate (requests per second)
java -cp load.jar:httploadgenerator.jar tools.descartes.dlim.httploadgenerator.runner.Main director --ip localhost --load load.csv -o result.csv --lua load.lua -t 256 -c measurment.ProcListener --wd $1 --wp $3 --wr $2