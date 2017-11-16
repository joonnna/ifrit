import matplotlib.pyplot as plt


res = open("exp_res", "r")

vals = []
x_vals = []
y_vals = []

for line in res:
    values = line.strip("\n").split("\t")
    if len(values) < 2:
        continue
    vals.append((int(values[1]), int(values[2])))
    x_vals.append(int(values[1]))

res.close()
x_vals = list(set(x_vals))


for val in x_vals:
    v = [x[1] for x in vals if x[0] == val]
    y_vals.append((sum(v)/len(v)))

x_vals.sort()
print x_vals
print y_vals

plt.plot(x_vals, y_vals)
plt.show()
