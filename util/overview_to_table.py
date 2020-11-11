import re
import pandas

RUNS = [1, 2, 3]
LOADS = [5, 10, 15, 20, 25, 30, 35]

PATTERN = re.compile("^(\w+):.+avg\. response time: ([\d\.]+),.*avg\. utilization: ([\d\.]+),.*")

ITEM_AMOUNT_PER_CART = 1
CORE_COUNT = 4

df_util = None
df_rt = None

for load in LOADS:
    service_dict_util = dict()
    service_dict_rt = dict()
    for run in RUNS:
        file = open("checkout-" + str(run) + "-" + str(load) + "/overview.txt")
        for line in file:
            res = PATTERN.match(line)
            if res:
                service_name = res.group(1)
                avg_rt = float(res.group(2))
                avg_util = float(res.group(3)) * 100
                if service_name == "adservice":
                    continue
                if not service_name in service_dict_util:
                    service_dict_util[service_name] = []
                if not service_name in service_dict_rt:
                    service_dict_rt[service_name] = []
                service_dict_util[service_name].append(avg_util)
                service_dict_rt[service_name].append(avg_rt)
    for k, v in service_dict_util.items():
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
        service_dict_util[k] = sdl
    for k, v in service_dict_rt.items():
        avg_rt = sum(v) / len(v)
        service_dict_rt[k] = avg_rt
    if df_util is None:
        df_util = pandas.DataFrame(data=service_dict_util, index=[load])
    else:
        df_util = df_util.append(pandas.DataFrame(service_dict_util, index=[load]))
    if df_rt is None:
        df_rt = pandas.DataFrame(data=service_dict_rt, index=[load])
    else:
        df_rt = df_rt.append(pandas.DataFrame(service_dict_rt, index=[load]))

means = []
for column in df_util:
    means.append(df_util[column].mean() * CORE_COUNT * 10)
df_util = df_util.append(pandas.DataFrame(data=[means], index=["SDL of MEAN"], columns=df_util.columns))

with pandas.ExcelWriter('overview_table.xlsx') as writer:
    df_util.to_excel(writer, sheet_name='SDL Values')
    df_rt.to_excel(writer, sheet_name='Response Times')

print(df_util)
print(df_rt)
