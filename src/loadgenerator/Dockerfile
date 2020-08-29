FROM openjdk:8-jdk-alpine

ARG LUA_FILE=example_load.lua
ARG LOAD_FILE=example_load.csv

WORKDIR /usr/local/loadgen

COPY ${LUA_FILE} load.lua
COPY httploadgenerator.jar httploadgenerator.jar
COPY ${LOAD_FILE} load.csv
COPY load.jar load.jar
COPY run.sh run.sh
COPY userids.txt userids.txt

RUN chmod +x ./run.sh

ENTRYPOINT ["/usr/local/loadgen/run.sh"]