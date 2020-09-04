import pandas as pd
import re
import sys

class Node:
    def __init__(self, name, instance_name):
        self.name = name
        self.instance_name = instance_name
        self.avg_utilization = "not defined"
        self.response_times = dict()
        # self.related_nodes = set()

    def toString(self):
        return self.name + ": " + self.instance_name + ", avg. utilization: " + self.avg_utilization + ", response times: " + str(self.response_times) #+ ", " + str(self.related_nodes)

    def setUtil(self, util):
        self.avg_utilization = util


EXPERIMENT_PATH = sys.argv[1]
UTIL_TS_PATTERN = re.compile('^Timeseries:.*key:\"instance_name\" value:\"([\w\-]+)\"\}.*$')
UTIL_U_PATTERN = re.compile('^.*Utilization: ([\d\.]+)$')

# read files
nodemap = pd.read_csv(EXPERIMENT_PATH + "/nodemap.txt", header=None, names=["service", "instance"])
utilfile = open(EXPERIMENT_PATH + "/util_results.txt")

nodes = []
service_name = ""
for line in utilfile:
    res = UTIL_TS_PATTERN.match(line)
    if res:
        for index, row in nodemap.iterrows():
            if row["instance"] == res.group(1):
                service_name = row["service"]
                break
        nodes.append(Node(service_name, res.group(1)))
    else:
        res2 = UTIL_U_PATTERN.match(line)
        if res2:
            for node in nodes:
                if node.name == service_name:
                    node.setUtil(res2.group(1))
                    break

zipkin_spans = pd.read_csv(EXPERIMENT_PATH + "/zipkin_spans.csv")
# zipkin_annotations = pd.read_csv(EXPERIMENT_PATH + "/zipkin_annotations.csv")

for node in nodes:
    spans = None
    if node.name == "zipkin":
        continue
    if node.name == "frontend":
        spans = zipkin_spans[zipkin_spans['name'].str.startswith('/')]
    else:
        spans = zipkin_spans[zipkin_spans['name'].str.contains(node.name)]
    unique_workloads = set(spans["name"])
    for wl in unique_workloads:
        rst = spans[(spans.name == wl)]["duration"].astype(int) / 1000 # microseconds to milliseconds
        node.response_times[wl] = str(sum(rst) / len(rst)) + "ms"
    print(node.toString())