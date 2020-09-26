import csv
import sys

output_file = sys.argv[1]
min = int(sys.argv[2])
max = int(sys.argv[3])
length = int(sys.argv[4])
step = 1

currentLoad = min
direction = True    #true = load rising, false = load decreasing

writer = csv.writer(open(output_file, "w", newline=''), delimiter=',')
for second in range(length):
    if(direction):
        currentLoad += step
        if(currentLoad >= max):
            direction = False
    else:
        currentLoad -= step
        if(currentLoad <= min):
            direction = True
    writer.writerow([second + 0.5, currentLoad])
