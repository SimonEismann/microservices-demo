#!/usr/bin/python
#
# Copyright 2018 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import os
import time
from concurrent import futures

import grpc
import numpy as np
from grpc_health.v1 import health_pb2
from grpc_health.v1 import health_pb2_grpc
from opencensus.ext.grpc import server_interceptor
from opencensus.ext.zipkin.trace_exporter import ZipkinExporter
from opencensus.trace.samplers import AlwaysOnSampler
from opencensus.trace.tracer import Tracer

import demo_pb2
import demo_pb2_grpc
from logger import getJSONLogger

logger = getJSONLogger('emailservice')

# passes time by multiplying matrices t times
def passTime(t):
    if t <= 0:
        return
    for i in range(t):  # create and multiply two random 50x50 matrices (values in [0,1)) until t is reached
        m1 = np.arange(2500).reshape((50, 50))      # np.random.rand(50, 50)
        m2 = np.arange(2500).reshape((50, 50))      # np.random.rand(50, 50)
        np.matmul(m1, m2)

sendConfirmationDelay = int(os.environ.get("DELAY_SEND_CONFIRMATION", '0'))

# Setup Zipkin exporter
try:
    zipkin_service_addr = os.environ.get("ZIPKIN_SERVICE_ADDR", '')
    if zipkin_service_addr == "":
        logger.info(
            "Skipping Zipkin traces initialization. Set environment variable ZIPKIN_SERVICE_ADDR=<host>:<port> to enable.")
        raise KeyError()
    host, port = zipkin_service_addr.split(":")
    ze = ZipkinExporter(service_name="emailservice",
                        host_name=host,
                        port=int(port),
                        endpoint='/api/v2/spans')
    sampler = AlwaysOnSampler()
    tracer = Tracer(exporter=ze, sampler=sampler)
    tracer_interceptor = server_interceptor.OpenCensusServerInterceptor(sampler, ze)
    logger.info("Zipkin traces enabled, sending to " + zipkin_service_addr)
except KeyError:
    tracer_interceptor = server_interceptor.OpenCensusServerInterceptor()

class BaseEmailService(demo_pb2_grpc.EmailServiceServicer):
    def Check(self, request, context):
        return health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.SERVING)

class DummyEmailService(BaseEmailService):
    def SendOrderConfirmation(self, request, context):
        passTime(sendConfirmationDelay)
        logger.info('A request to send order confirmation email to {} has been received.'.format(request.email))
        return demo_pb2.Empty()


class HealthCheck():
    def Check(self, request, context):
        return health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.SERVING)


def start():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10),
                         interceptors=(tracer_interceptor,))
    healthServer = grpc.server(futures.ThreadPoolExecutor(max_workers=10))

    service = DummyEmailService()

    demo_pb2_grpc.add_EmailServiceServicer_to_server(service, server)
    health_pb2_grpc.add_HealthServicer_to_server(HealthCheck(), healthServer)

    port = os.environ.get('PORT', "8080")
    logger.info("listening on port: " + port)
    server.add_insecure_port('[::]:' + port)
    server.start()
    healthPort = os.environ.get('HEALTH_PORT', "8081")
    logger.info("listening on port: " + healthPort)
    healthServer.add_insecure_port('[::]:' + healthPort)
    healthServer.start()
    try:
        while True:
            time.sleep(3600)
    except KeyboardInterrupt:
        server.stop(0)
        healthServer.stop(0)

if __name__ == '__main__':
    logger.info('starting the email service in dummy mode.')
    start()
