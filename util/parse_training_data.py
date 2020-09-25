import csv
import sys
import pandas as pd

TRAINING_DATA_FOLDER_PATH = sys.argv[1]
OUTPUT_FILE_NAME = sys.argv[2]
FILES = sys.argv[3].split(",")     # all files to include for this operation separated by comma

dataframe = pd.concat([pd.read_csv(TRAINING_DATA_FOLDER_PATH + "/" + f, sep=',', header=None, names=["start", "end"]) for f in FILES], ignore_index=True)
dataframe.sort_values('start')
open_requests = []
writer = csv.writer(open(TRAINING_DATA_FOLDER_PATH + "/" + OUTPUT_FILE_NAME, "w", newline=''), delimiter=',')
writer.writerow(["Response Time", "Concurrency"])

for index, row in dataframe.iterrows():
    to_remove = []
    for open_request in open_requests:
        if open_request['end'] < row['start']:
            to_remove.append(open_request)
    for req in to_remove:
        open_requests.remove(req)
    writer.writerow([(int(row['end']) - int(row['start'])) / 1000, len(open_requests) + 1])     # mikro to milliseconds
    open_requests.append(row)
