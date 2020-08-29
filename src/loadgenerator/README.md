A load profile can be configured in `example_load.csv` -> Format: "passed_time,request_amount_to_send".

`example_load.lua` defines the behavior of the Loadgenerator. We deploy the loadgenerator with only 1 thread so the example behavior is not executed multiple times at once. Therefore, the behavior of the example behavior can be observed better.

The result log of the Loadgenerator appears in the root folder as `result.csv`.

For more informations about the loadgenerator look [here](https://github.com/SimonEismann/HTTP-Load-Generator).

We use an adapted version which supports `application/x-www-form-urlencoded` HTTP content-type for POST requests. Further, custom user id lists (userids.txt) are supported
```shell
# test your lua script for errors
java -jar httpscripttester.jar ./example_load.lua

# run loadgenerator (with load.csv and load.lua present in root folder -> see Dockerfile)
./run.sh
```