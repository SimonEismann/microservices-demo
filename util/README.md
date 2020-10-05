## cart populator
Fills the redis cart database with carts without using the microservice infrastructure. The "-1" version fills every cart with one item.

## modelgen_weka.jar
Generates serialized WEKA 3.8.4 regression models from our parsed CSVs (experiment_data_overview.py --> parse_training_data.py --> training data). Can be configured to generate M5P or random forest models. Usage:
```shell
java -jar modelgen_weka.jar [M5P|RandomForest] /path/to/training_data.csv /weka/model/output.model
```