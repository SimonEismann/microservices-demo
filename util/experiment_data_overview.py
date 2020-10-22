import pandas as pd
import re
import sys


class Node:
    def __init__(self, name, instance_name):
        self.name = name
        self.instance_name = instance_name
        self.avg_resptime = 0.0
        self.avg_utilization = "not defined"
        self.max_utilization = "not defined"
        self.top3_utilization = "not defined"
        self.response_times = dict()
        # self.related_nodes = set()

    def toString(self):
        return self.name + ": " + self.instance_name + ", avg. response time: " + str(
            self.avg_resptime) + ", top3 utilization: " + self.top3_utilization + ", avg. utilization: " + self.avg_utilization + ", max utilization: " + self.max_utilization + ", response times: " + str(
            self.response_times)  # + ", " + str(self.related_nodes)

    def setAvgUtil(self, util):
        self.avg_utilization = util

    def setMaxUtil(self, util):
        self.max_utilization = util

    def setTop3Util(self, util):
        self.top3_utilization = util


EXPERIMENT_PATH = sys.argv[1]
DO_EXPORT = sys.argv[2].startswith("export=true")  # tells if script should export training data from spans
UTIL_TS_PATTERN = re.compile('^Timeseries:.*key:\"instance_name\"\s+value:\"([\w\-]+)\"\}.*')
UTIL_AVG_PATTERN = re.compile('^Average Utilization: ([\d\.]+)$')
UTIL_MAX_PATTERN = re.compile('^Max Utilization: ([\d\.]+)$')
UTIL_TOP3_PATTERN = re.compile('^Top3 Utilization: ([\d\.]+)$')
CLIENT_POSTFIX = " CLIENT"

# read files
nodemap = pd.read_csv(EXPERIMENT_PATH + "/nodemap.txt", header=None, names=["service", "instance", "IP"])
loadgen_file = pd.read_csv(EXPERIMENT_PATH + "/loadgen_result.csv")

nodes = []
columns = list(loadgen_file.columns.values)
for index, row in nodemap.iterrows():
    server_node = Node(row["service"], row["instance"])
    client_node = Node(row["service"] + CLIENT_POSTFIX, row["instance"])
    utils = None
    for i in range(len(columns)):
        if row["IP"] in columns[i]:
            utils = loadgen_file[columns[i]].astype(float)
            break
    utils = sorted(utils)
    server_node.avg_utilization = str(sum(utils) / len(utils))
    server_node.max_utilization = str(utils[-1])
    server_node.top3_utilization = str(sum(utils[-3:-1]) / 3)
    nodes.append(server_node)
    nodes.append(client_node)

zipkin_spans = pd.read_csv(EXPERIMENT_PATH + "/zipkin_spans.csv")
zipkin_annotations = pd.read_csv(EXPERIMENT_PATH + "/zipkin_annotations.csv")
client_anns = zipkin_annotations[zipkin_annotations['a_key'].str.startswith("Client")]
client_anns = client_anns[client_anns['a_value'].str.startswith("true")]


def getClient(spanID):
    if spanID in client_anns['span_id'].values:
        return True
    else:
        return False


nodes.sort(key=lambda node: node.name)
for node in nodes:
    spans = None
    if node.name.startswith("zipkin"):
        continue
    if node.name == "frontend":
        spans = zipkin_spans[zipkin_spans['name'].str.startswith('/')].copy()
        spans["name"].replace(to_replace='/cart/\d+.*', value='/cart/{user_id}', regex=True, inplace=True)
        spans["name"].replace(to_replace='/product/.*', value='/product/{product_id}', regex=True, inplace=True)
    elif node.name.endswith(CLIENT_POSTFIX):
        node_name = re.sub(CLIENT_POSTFIX + "$", '', node.name)
        spans = zipkin_spans[zipkin_spans['name'].str.contains(node_name)]
        spans = spans.loc[spans['id'].apply(lambda id: getClient(id))]
    else:
        spans = zipkin_spans[zipkin_spans['name'].str.contains(node.name)]
        spans = spans.loc[spans['id'].apply(lambda id: not getClient(id))]
    unique_workloads = set(spans["name"])
    node.avg_resptime = spans["duration"].mean() / 1000
    for wl in unique_workloads:
        tmp = spans[(spans.name == wl)]
        rst = tmp["duration"].astype(int) / 1000  # microseconds to milliseconds
        if DO_EXPORT and not node.name.endswith(CLIENT_POSTFIX):
            f = open(EXPERIMENT_PATH + "/training_data/" + wl.replace("/", "") + ".csv", "w")
            for index, span in tmp.iterrows():
                start = int(span["start_ts"])
                end = start + int(span["duration"])
                f.write(str(start) + "," + str(end) + "\n")
            f.close()
        node.response_times[wl] = str(sum(rst) / len(rst)) + "ms"
    if not len(node.response_times) == 0:
        print(node.toString())
