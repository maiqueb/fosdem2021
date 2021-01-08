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

## CAP_NET_RAW demo

This part of the demo will showcase how CAP_NET_RAW impacts network
operations; we will create two containers (1 w/ CAP_NET_RAW; another without)
and perform a set of operations.

```bash
# attempt to create a RAW socket without CAP_NET_RAW
podman run --rm -it --name cap-net-raw-demo-dropped-cap \
    --cap-drop net_raw \
    capabilities-demo \
    /capabilities-demo raw-socket tap0

# attempt to create a RAW socket *with* CAP_NET_RAW
podman run --rm -it --name cap-net-raw-demo \
    capabilities-demo \
    /capabilities-demo raw-socket tap0
```

CAP_NET_RAW is also required when requesting the `SO_BINDTODEVICE` socket
option.

```bash
# attempt to use the socket option SO_BINDTODEVICE without CAP_NET_RAW
podman run --rm -t \
    --name demo-nocaps \
    --cap-drop net_raw \
    capabilities-demo \
    /capabilities-demo bind-to-device eth0 --port 800

# attempt to use the socket option SO_BINDTODEVICE *with* CAP_NET_RAW
podman run --rm -t \
    --name demo-with-cap-net-raw \
    --cap-drop net_raw \
    capabilities-demo \
    /capabilities-demo bind-to-device eth0 --port 800
```

## Abusing CAP_NET_ADMIN ...

A malicious user could for instance run a DHCP server on a pod having
CAP_NET_ADMIN to potentially highjack the network.

Let's see how:
```bash
# create a veth pair
ip l add sneaky-veth type veth peer sneaky-veth-br

# plug one end to the docker bridge
ip l set dev sneaky-veth-br master docker0

# set ifaces up
ip l set dev sneaky-veth up
ip l set dev sneaky-veth-br up

# start the dhcp-server
podman run --rm -it \
    --name dhcp-server \
    --cap-add net_admin \
    capabilities-demo \
    /capabilities-demo start-dhcp-server tap0 \
        --cidr 10.10.10.2/24 \
        --ip-server 10.10.10.1 \
        --ip-router 172.17.0.1

# request an address on via the veth
$ dhclient -v sneaky-veth
```
