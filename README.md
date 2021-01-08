# KubeVirt: privilege dropping one capability at a time - Fosdem2021

## CAP_NET_ADMIN demo

This part of the demo will showcase how CAP_NET_ADMIN impacts network
operations; we will create two containers (1 w/ CAP_NET_ADMIN; another without)
and perform a set of operations.

```bash
# creating a container w/ CAP_NET_ADMIN
podman run -itd --rm --name with-cap-net-admin --cap-add net_admin centos:8 bash

# w/out CAP_NET_ADMIN
podman run -itd --rm --name no-caps centos:8 bash

# get container PIDs
no_caps_pid=$(podman inspect no-caps -f '{{ .State.Pid }}')
net_admin_pid=$(podman inspect with-cap-net-admin -f '{{ .State.Pid }}')

# create bridge
podman exec -it $no_caps_pid   ip link add br0 type bridge
podman exec -it $net_admin_pid ip link add br0 type bridge

# create tap device - requires `privileged`, must be run from sudo
nsenter -t $no_caps_pid -n   ip tuntap add dev tap0 mode tap user root
nsenter -t $net_admin_pid -n ip tuntap add dev tap0 mode tap user root

# enslave tap device
podman exec -it $no_caps_pid   ip link set dev tap0 master br0
podman exec -it $net_admin_pid ip link set dev tap0 master br0

# set MAC
podman exec -it $no_caps_pid   ip l set dev tap0 address 02:00:00:01:02:03
podman exec -it $net_admin_pid ip l set dev tap0 address 02:00:00:01:02:03

# set MTU
podman exec -it $no_caps_pid   ip l set dev tap0 mtu 9000
podman exec -it $net_admin_pid ip l set dev tap0 mtu 9000
```
