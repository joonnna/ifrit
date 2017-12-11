import matplotlib.pyplot as plt


res = open("res", "r")

vals = []
x_vals = []
y_vals = []

for line in res:
    values = line.strip("\n").split("\t")
    if len(values) < 2:
        continue


    #vals.append((int(values[2]), int(values[3])))
    x_vals.append(float(values[2])*100.0)
    y_vals.append(int(values[3]))

res.close()
"""
x_vals = list(set(x_vals))


for val in x_vals:
    v = [x[1] for x in vals if x[0] == val]
    y_vals.append((sum(v)/len(v)))
x_vals.sort()

"""

axes = plt.gca()
#axes.set_xlim([xmin,xmax])
axes.set_ylim([200,1000])

plt.ylabel("Legitimate requests processed")
plt.xlabel("Percentage of attacking nodes")
print x_vals
print y_vals

plt.plot(x_vals, y_vals)
plt.savefig("fig2.pdf")
