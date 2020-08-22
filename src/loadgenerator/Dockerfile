FROM openjdk:8-jdk-alpine

ARG LUA_FILE=example_load.lua
ARG LOAD_FILE=example_load.csv
ARG WARMUP_DURATION=60
ARG WARMUP_RATE=5
ARG WARMUP_PAUSE=10

WORKDIR /usr/local/loadgen

COPY ${LUA_FILE} load.lua
COPY httploadgenerator.jar httploadgenerator.jar
COPY ${LOAD_FILE} load.csv
COPY load.jar load.jar
COPY run.sh run.sh

RUN chmod +x ./run.sh

ENTRYPOINT ["./run.sh", ${WARMUP_DURATION}, ${WARMUP_RATE}, ${WARMUP_PAUSE}]