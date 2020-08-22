A load profile can be configured in `example_load.csv` -> Format: "passed_time,request_amount_to_send".

`example_load.lua` defines the behavior of the Loadgenerator.

The result log of the Loadgenerator appears in the root folder as `result.csv`.

For more informations about the loadgenerator look [here](https://github.com/SimonEismann/HTTP-Load-Generator).
```shell
# test your lua script for errors
java -jar httpscripttester.jar ./example_load.lua

# run loadgenerator (with load.csv and load.lua present in root folder)
./run.sh
```