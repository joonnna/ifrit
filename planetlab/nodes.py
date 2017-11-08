import xmlrpclib
import subprocess
import socket

api_server = xmlrpclib.ServerProxy('https://www.planet-lab.eu/PLCAPI/', allow_none=True)

def addNodes(nodeList):
    ret = api_server.AddSliceToNodes(auth, slice_name, nodeList)
    if ret == 1:
        print "Nodes were added to slice"
    else:
        print "Failed to add nodes"

def getSliceNodeAddrs(auth):
    slice_return_fields = ['node_ids']

    ids = api_server.GetSlices(auth, slice_name, slice_return_fields)

    node_ids = [int(x) for x in ids[0]['node_ids']]

    return_fields = ['hostname']
    res = api_server.GetNodes(auth, node_ids, return_fields)

    return [x['hostname'] for x in res]


def getBootedNodes(auth):
    boot_state_filter = {'boot_state': 'boot'}
    return_fields = ['hostname']
    return api_server.GetNodes(auth, boot_state_filter, return_fields)

def getAllNodes(auth):
    boot_state_filter = {}
    return_fields = ['hostname']
    res = api_server.GetNodes(auth, boot_state_filter, return_fields)

    return [x['hostname'] for x in res]
    """
    ret = []
    for h in hostNames:
        try:
            ip = socket.gethostbyname(h)
            ret.append((h, ip))
        except socket.gaierror:
            continue
    return ret
    """

def getSliceBootedNodes(auth, slice_name):
    filter = {'boot_state': 'boot'}

    slice_return_fields = ['node_ids']
    ids = api_server.GetSlices(auth, slice_name, slice_return_fields)

    filter['node_id'] = ids[0]['node_ids']

    return_fields = ['hostname']

    nodes = api_server.GetNodes(auth, filter, return_fields)

    hostNames = [x['hostname'] for x in nodes]
    print hostNames
    print len(hostNames)
    """
    ret = []
    for h in hostNames:
        try:
            ip = socket.gethostbyname(h)
            ret.append((h, ip))
        except socket.gaierror:
            continue
    return ret
    """
# Create an empty dictionary (XML-RPC struct)
auth = {}
# Specify password authentication
auth['AuthMethod'] = 'password'
# Username and password
auth['Username'] = 'jmi021@post.uit.no'
auth['AuthString'] = 'feeder123'

slice_name = 'uitple_firechain'

authorized = api_server.AuthCheck(auth)
if authorized:
    print 'We are authorized!'


nodes = getAllNodes(auth)
print len(nodes)

f = open("node_addrs", "w")
for a in nodes:
    f.write("%s\n" % (a))
f.close()

"""
nodes = getBootedNodes(auth)
hostNames = []
for n in nodes:
    hostNames.append(n['hostname'])
addNodes(hostNames)
"""
