import re
import pandas

RUNS = [1, 2, 3]
LOADS = [5, 10, 15, 20, 25, 30]

PATTERN = re.compile("^(\w+):.+avg\. response time: ([\d\.]+),.*avg\. utilization: ([\d\.]+),.*")

def getClientPattern(service_name):
    return re.compile("^" + service_name + " CLIENT:.+avg\. response time: ([\d\.]+),.*")

def getClientAvgResponseTime(service_name, load):
    rts = []
    ptrn = getClientPattern(service_name)
    for run in RUNS:
        file = open("checkout-" + str(run) + "-" + str(load) + "/overview.txt")
        for line in file:
            res = ptrn.match(line)
            if res:
                rts.append(float(res.group(1)))
                break
        file.close()
    return sum(rts) / len(rts)

# config for specific setup
ITEM_AMOUNT_PER_CART = 1    # how many items are in cart? (constant)
RECOMMENDATION_AMOUNT = 1   # how many recommendations does frontend show on page? (constant)
CORE_COUNT = 4

df_util = None
df_rt = None
df_sdl = None

for load in LOADS:
    service_dict_util = dict()
    service_dict_rt = dict()
    service_dict_sdl = dict()
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
        avg_util = sum(v) / len(v)
        sdl =  avg_util / load
        if k == "cartservice":
            sdl = sdl / 2
            print("cartservice triggered")
        elif k == "currencyservice":
            sdl = sdl / (ITEM_AMOUNT_PER_CART + 1)
            print("currencyservice triggered")
        elif k == "productcatalogservice":
            sdl = sdl / (ITEM_AMOUNT_PER_CART + RECOMMENDATION_AMOUNT + 1)
            print("productcatalogservice triggered")
        elif k == "shippingservice":
            sdl = sdl / 2
            print("shippingservice triggered")
        service_dict_util[k] = avg_util
        service_dict_sdl[k] = sdl
    for k, v in service_dict_rt.items():
        avg_rt = sum(v) / len(v)
        service_dict_rt[k] = avg_rt
    if df_util is None:
        df_util = pandas.DataFrame(data=service_dict_util, index=[load])
    else:
        df_util = df_util.append(pandas.DataFrame(service_dict_util, index=[load]))
    if df_sdl is None:
        df_sdl = pandas.DataFrame(data=service_dict_sdl, index=[load])
    else:
        df_sdl = df_sdl.append(pandas.DataFrame(service_dict_sdl, index=[load]))
    if df_rt is None:
        df_rt = pandas.DataFrame(data=service_dict_rt, index=[load])
    else:
        df_rt = df_rt.append(pandas.DataFrame(service_dict_rt, index=[load]))

means = []
for column in df_sdl:
    means.append(df_sdl[column].mean() * CORE_COUNT * 10)
df_sdl = df_sdl.append(pandas.DataFrame(data=[means], index=["SDL of MEAN"], columns=df_sdl.columns))

# RTA calculation
df_rta = df_rt.copy(deep=True)

def getRTA(service_name, avg_rt, load):
    rta = avg_rt
    if service_name == "recommendationservice":
        rta -= getClientAvgResponseTime("productcatalogservice", load)
    elif service_name == "checkoutservice":
        rta -= 2*getClientAvgResponseTime("cartservice", load) + 2*getClientAvgResponseTime("shippingservice", load) + (ITEM_AMOUNT_PER_CART + 1)*getClientAvgResponseTime("currencyservice", load) + ITEM_AMOUNT_PER_CART*getClientAvgResponseTime("productcatalogservice", load) + getClientAvgResponseTime("emailservice", load) + getClientAvgResponseTime("paymentservice", load)
    elif service_name == "frontend":
        rta -= getClientAvgResponseTime("recommendationservice", load) + getClientAvgResponseTime("checkoutservice", load) + RECOMMENDATION_AMOUNT*getClientAvgResponseTime("productcatalogservice", load)
    return rta

for column in df_rta:
    for i in range(len(LOADS)):
        rt_avg = df_rta.at[LOADS[i], column]
        rta = getRTA(column, rt_avg, LOADS[i])
        df_rta.at[LOADS[i], column] = rta

# final result save
with pandas.ExcelWriter('overview_table.xlsx') as writer:
    df_util.to_excel(writer, sheet_name='Utilizations')
    df_sdl.to_excel(writer, sheet_name='SDL Values')
    df_rt.to_excel(writer, sheet_name='Response Times')
    df_rta.to_excel(writer, sheet_name='RTA Values')

print(df_util)
print(df_sdl)
print(df_rt)
print(df_rta)
