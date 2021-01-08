package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/golang/glog"
	dhcp "github.com/krolaw/dhcp4"
	dhcpConn "github.com/krolaw/dhcp4/conn"
	"github.com/spf13/cobra"

	"golang.org/x/sys/unix"
)

const (
	infiniteLease = 999 * 24 * time.Hour
)

func main() {
	flag.Parse()
	if err := flag.Set("alsologtostderr", "true"); err != nil {
		os.Exit(1)
	}

	rootCmd := &cobra.Command{
		Use: "capabilities-demo",
	}

	createRawSocket := &cobra.Command{
		Use:  "raw-socket",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ifaceName := args[0]

			glog.Infof("Will create a RAW socket on interface: %s", ifaceName)
			fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_RAW, unix.IPPROTO_ICMPV6)
			if err != nil {
				return fmt.Errorf("cannot get a RAW socket: %v", err)
			}
			f := os.NewFile(uintptr(fd), "")
			// net.FilePacketConn dups the FD, so we have to close this in any case.
			defer f.Close()

			listenAddr := &net.IPAddr{
				IP:   net.IPv6unspecified,
				Zone: ifaceName,
			}
			// Bind to the port.
			saddr := &unix.SockaddrInet6{}
			copy(saddr.Addr[:], listenAddr.IP)
			if err := unix.Bind(fd, saddr); err != nil {
				return fmt.Errorf("cannot bind to address %v: %v", saddr, err)
			}

			glog.Infof("Successfully created a RAW socket on iface: %s w/ fd number: %d", ifaceName, fd)
			return nil
		},
	}

	bindToDevice := &cobra.Command{
		Use:  "bind-to-device",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ifaceName := args[0]
			port, err := cmd.Flags().GetUint("port")
			if err != nil {
				return fmt.Errorf("could not parse the port number: %v", err)
			}

			glog.Infof("Will create a DGRAM socket on interface: %s", ifaceName)
			fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
			if err != nil {
				return fmt.Errorf("cannot get a DGRAM socket: %v", err)
			}
			f := os.NewFile(uintptr(fd), "")
			// net.FilePacketConn dups the FD, so we have to close this in any case.
			defer f.Close()

			// Bind directly to the interface.
			if err := unix.BindToDevice(fd, ifaceName); err != nil {
				if errno, ok := err.(unix.Errno); ok && errno == unix.EPERM {
					// Return a more helpful error message in this (fairly common) case
					return fmt.Errorf("cannot bind to interface without CAP_NET_RAW or root permissions")
				}
				return fmt.Errorf("cannot bind to interface %s: %v", ifaceName, err)
			}
			glog.Infof("Succeeded in binding to device %s, port %d", ifaceName, port)

			saddr := unix.SockaddrInet6{Port: int(port)}
			copy(saddr.Addr[:], net.IPv6unspecified)
			if err := unix.Bind(fd, &saddr); err != nil {
				return fmt.Errorf("cannot bind to address %v: %v", saddr, err)
			}

			glog.Infof("Created a UDP socket bound to device: %s", ifaceName)
			return nil
		},
	}

	startDhcpServer := &cobra.Command{
		Use:  "start-dhcp-server",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ifaceName := args[0]
			cidr := cmd.Flag("cidr").Value.String()
			ip, ipNet, err := net.ParseCIDR(cidr)
			if err != nil {
				return fmt.Errorf("error parsing CIDR: %v", err)
			}

			defaultGwIP, err := cmd.Flags().GetIP("ip-router")
			if err != nil {
				return fmt.Errorf("error parsing default GW IP: %v", err)
			}

			serverIP, err := cmd.Flags().GetIP("ip-server")
			if err != nil {
				return fmt.Errorf("error parsing server IP: %v", err)
			}

			mtu, err := cmd.Flags().GetUint16("mtu")
			if err != nil {
				return fmt.Errorf("error parsing MTU: %v", err)
			}

			err = SingleClientDHCPServer(ip, ipNet.Mask, ifaceName, serverIP, defaultGwIP, mtu)
			if err != nil {
				return fmt.Errorf("woop: %v", err)
			}

			return nil
		},
	}

	bindToDevice.Flags().Uint("port", 547, "specify the port to bind to")
	startDhcpServer.Flags().String("mac-addr", "", "the MAC address of the DHCP server")
	startDhcpServer.Flags().String("cidr", "", "the IP address to advertise")
	startDhcpServer.Flags().IP("ip-server", net.IP{}, "the IP address of the advertising server")
	startDhcpServer.Flags().IP("ip-router", net.IP{}, "the IP address of the router")
	startDhcpServer.Flags().Uint16("mtu", 1280, "the MTU to advertise")
	rootCmd.AddCommand(createRawSocket, bindToDevice, startDhcpServer)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func SingleClientDHCPServer(
	clientIP net.IP,
	clientMask net.IPMask,
	serverIface string,
	serverIP net.IP,
	routerIP net.IP,
	mtu uint16) error {

	glog.Info("Starting SingleClientDHCPServer")

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("reading the pods hostname failed: %v", err)
	}

	options, err := prepareDHCPOptions(clientMask, routerIP, mtu, hostname)
	if err != nil {
		return err
	}

	handler := &DHCPHandler{
		clientIP:      clientIP,
		serverIP:      serverIP.To4(),
		leaseDuration: infiniteLease,
		options:       options,
	}

	l, err := dhcpConn.NewUDP4BoundListener(serverIface, ":67")
	if err != nil {
		return err
	}
	defer l.Close()
	err = dhcp.Serve(l, handler)
	if err != nil {
		return err
	}
	return nil
}

func prepareDHCPOptions(
	clientMask net.IPMask,
	routerIP net.IP,
	mtu uint16,
	hostname string) (dhcp.Options, error) {

	mtuArray := make([]byte, 2)
	binary.BigEndian.PutUint16(mtuArray, mtu)

	dhcpOptions := dhcp.Options{
		dhcp.OptionSubnetMask:   []byte(clientMask),
		dhcp.OptionRouter:       []byte(routerIP),
		dhcp.OptionInterfaceMTU: mtuArray,
	}

	dhcpOptions[dhcp.OptionHostName] = []byte(hostname)

	return dhcpOptions, nil
}

type DHCPHandler struct {
	serverIP      net.IP
	clientIP      net.IP
	leaseDuration time.Duration
	options       dhcp.Options
}

func (h *DHCPHandler) ServeDHCP(p dhcp.Packet, msgType dhcp.MessageType, options dhcp.Options) (d dhcp.Packet) {
	glog.Info("Serving a new request")
	switch msgType {
	case dhcp.Discover:
		glog.Info("The request has message type DISCOVER")
		return dhcp.ReplyPacket(p, dhcp.Offer, h.serverIP, h.clientIP, h.leaseDuration,
			h.options.SelectOrderOrAll(nil))

	case dhcp.Request:
		glog.Info("The request has message type REQUEST")
		return dhcp.ReplyPacket(p, dhcp.ACK, h.serverIP, h.clientIP, h.leaseDuration,
			h.options.SelectOrderOrAll(nil))

	default:
		glog.Info("The request has unhandled message type")
		return nil // Ignored message type

	}
}
