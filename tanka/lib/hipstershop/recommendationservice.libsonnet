(import "ksonnet-util/kausal.libsonnet") +
{

  local deploy = $.apps.v1.deployment,
  local container = $.core.v1.container,
  local port = $.core.v1.containerPort,
  local service = $.core.v1.service,

  _config+:: {
    recommendationservice: {
      app: "recommendationservice",
      namespace: $._config.namespace, //set a default namespace if not overrided in the main file
      port: 8080,
      portName: "grpc",
      image: {
        repo: $._config.image.repo,
        name: "recommendationservice",
        tag: $._config.image.tag,
      },
      labels: {app: "recommendationservice"},
      env: {
        PORT: "%s" % $._config.recommendationservice.port,
        ENABLE_PROFILER: "0",
        PRODUCT_CATALOG_SERVICE_ADDR: $._config.productcatalogservice.URL,
    },
      readinessProbe: container.mixin.readinessProbe.exec.withCommand(["/bin/grpc_health_probe", "-addr=:%s" % self.port,]),
      livenessProbe: container.mixin.livenessProbe.exec.withCommand(["/bin/grpc_health_probe", "-addr=:%s" % self.port,]),
      limits: container.mixin.resources.withLimits({cpu: "200m", memory: "450Mi"}),
      requests: container.mixin.resources.withRequests({cpu: "100m", memory: "220Mi"}),
      deploymentExtra: {},
      serviceExtra: {},
    },
  },
}