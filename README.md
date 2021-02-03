# KubeVirt: privilege dropping one capability at a time - Fosdem2021

These short examples were tested on the KubeVirt CI node. Instructions on how
to get one running can be found in this
[section](#how-to-get-kubevirt-ci-node-running).

## CAP_NET_ADMIN demo

This part of the demo will showcase how CAP_NET_ADMIN impacts network
operations; we will create two containers (1 w/ CAP_NET_ADMIN; another without)
and perform a set of operations.

```bash
# creating a container w/ CAP_NET_ADMIN
net_admin_container=$(
    docker run -itd --rm \
        --name with-cap-net-admin \
        --cap-add net_admin \
        -v /dev/net/tun:/dev/net/tun \
    centos:8 bash
)

# w/out CAP_NET_ADMIN
no_caps_container=$(
    docker run -itd --rm \
        --name no-caps \
        -v /dev/net/tun:/dev/net/tun \
    centos:8 bash
)

# get container PID of the capability-less container
no_caps_pid=$(docker inspect no-caps -f '{{ .State.Pid }}')

# create bridge
## works, since we have net_admin
docker exec -it $net_admin_container ip link add br0 type bridge
docker exec -it $net_admin_container ip link set dev br0 up

## fails, since capability is missing
docker exec -it $no_caps_container   ip link add br0 type bridge

# create tap device
## works, since we have net_admin
docker exec -it $net_admin_container ip tuntap add dev tap0 mode tap
## fails, since capability is missing
docker exec -it $no_caps_container   ip tuntap add dev tap0 mode tap

# create the tap & bridge on the container without CAP_NET_ADMIN
# superuser is required for tapping into other namespaces
nsenter -t $no_caps_pid -n ip tuntap add dev tap0 mode tap
nsenter -t $no_caps_pid -n ip link add br0 type bridge

# enslave tap device
## works, since we have net_admin
docker exec -it $net_admin_container \
    ip link set dev tap0 master br0
## fails, since capability is missing
docker exec -it $no_caps_container \
    ip link set dev tap0 master br0

# set MAC
## works, since we have net_admin
docker exec -it $net_admin_container \
    ip l set dev tap0 address 02:00:00:01:02:03
## fails, since capability is missing
docker exec -it $no_caps_container \
    ip l set dev tap0 address 02:00:00:01:02:03

# set MTU
## works, since we have net_admin
docker exec -it $net_admin_container \
    ip l set dev tap0 mtu 9000
## fails, since capability is missing
docker exec -it $no_caps_container \
    ip l set dev tap0 mtu 9000
```

## CAP_NET_RAW demo

This part of the demo will showcase how CAP_NET_RAW impacts network
operations; we will create two containers (1 w/ CAP_NET_RAW; another without)
and perform a set of operations.

```bash
# attempt to create a RAW socket without CAP_NET_RAW
docker run --rm -it --name cap-net-raw-demo-dropped-cap \
    --cap-drop net_raw \
    capabilities-demo \
    /capabilities-demo raw-socket eth0

# attempt to create a RAW socket *with* CAP_NET_RAW
docker run --rm -it --name cap-net-raw-demo \
    capabilities-demo \
    /capabilities-demo raw-socket eth0
```

CAP_NET_RAW is also required when requesting the `SO_BINDTODEVICE` socket
option. For this section, you should ssh into the KubeVirt ci node:
```bash
$KUBEVIRT_REPO/cluster-up/ssh.sh node01

stty rows 28
stty columns 146
sudo groupadd docker
sudo usermod -aG docker $USER

# the password is `vagrant`
su vagrant
```

```bash
# attempt to use the socket option SO_BINDTODEVICE without CAP_NET_RAW
docker run --rm -t \
    --name demo-nocaps \
    --cap-drop net_raw \
    registry:5000/capabilities-demo \
    /capabilities-demo bind-to-device eth0 --port 800

# attempt to use the socket option SO_BINDTODEVICE *with* CAP_NET_RAW
docker run --rm -t \
    --name demo-with-cap-net-raw \
    --cap-add net_raw \
    registry:5000/capabilities-demo \
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
docker run --rm -it \
    --name dhcp-server \
    --cap-add net_admin \
    registry:5000/capabilities-demo \
    /capabilities-demo start-dhcp-server eth0 \
        --cidr 10.10.10.2/24 \
        --ip-server 10.10.10.1 \
        --ip-router 172.17.0.1

# request an address on via the veth
$ dhclient -v sneaky-veth
```

## Dependencies
- Git
- Golang
- container runtime (docker / podman / ...)
- KubeVirt CI node running

## How to get KubeVirt ci node running
Clone the KubeVirt project:
```bash
git clone git@github.com:kubevirt/kubevirt.git <kubevirt-repo-path>
```

Configure the environment:
```bash
export KUBEVIRT_PROVIDER=k8s-1.19
export KUBEVIRT_NUM_NODES=1

make cluster-up && make cluster-sync
```

Push the locally built container to the KubeVirt CI node:
```bash
$ push_registry="localhost:$($KUBEVIRT_REPO/cluster-up/cli.sh ports registry | tr -d '\r')"
# By default, the KubeVirt ci node container runtime is docker.
# This must be aligned
$ CONTAINER_RUNTIME=docker make container-build
$ docker tag capabilities-demo $push_registry/capabilities-demo
$ docker push $push_registry/capabilities-demo
```

