import csv
import sys

output_file = sys.argv[1]
min = sys.argv[2]
max = sys.argv[3]
length = sys.argv[4]

currentLoad = min
direction = True    #true = load rising, false = load decreasing

writer = csv.writer(open(output_file, "w", newline=''), delimiter=',')
for second in range(length):
    if(direction):
        currentLoad += 1
        if(currentLoad >= max):
            direction = False
    else:
        currentLoad -= 1
        if(currentLoad <= min):
            direction = True
    writer.writerow([second + 0.5, currentLoad])
