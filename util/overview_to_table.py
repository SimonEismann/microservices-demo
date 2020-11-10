import re
import pandas

RUNS = [1, 2, 3]
LOADS = [5, 10, 15, 20, 25, 30, 35]

PATTERN = re.compile("^(\w+):.+avg\. utilization: ([\d\.]+),.*")

ITEM_AMOUNT_PER_CART = 1
CORE_COUNT = 4

df = None

for load in LOADS:
    service_dict = dict()
    for run in RUNS:
        file = open("checkout-" + str(run) + "-" + str(load) + "/overview.txt")
        for line in file:
            res = PATTERN.match(line)
            if res:
                service_name = res.group(1)
                avg_util = float(res.group(2)) * 100
                if service_name == "adservice":
                    continue
                if not service_name in service_dict:
                    service_dict[service_name] = []
                service_dict[service_name].append(avg_util)
    for k, v in service_dict.items():
        print(str(load) + ": " + k + ": " + str(v))
        sdl = sum(v) / len(v) / load
        if k == "cartservice":
            sdl = sdl / 2
            print("cartservice triggered")
        elif k == "currencyservice":
            sdl = sdl / (ITEM_AMOUNT_PER_CART + 1)
            print("currencyservice triggered")
        elif k == "productcatalogservice":
            sdl = sdl / (ITEM_AMOUNT_PER_CART + 1)
            print("productcatalogservice triggered")
        elif k == "shippingservice":
            sdl = sdl / 2
            print("shippingservice triggered")
        service_dict[k] = sdl
    if df is None:
        df = pandas.DataFrame(data=service_dict, index=[load])
    else:
        df = df.append(pandas.DataFrame(service_dict, index=[load]))

means = []
for column in df:
    means.append(df[column].mean() * CORE_COUNT * 10)
df = df.append(pandas.DataFrame(data=[means], index=["SDL of MEAN"], columns=df.columns))

print(df)
