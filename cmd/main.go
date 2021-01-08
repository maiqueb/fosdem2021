package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/golang/glog"
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

	bindToDevice.Flags().Uint("port", 547, "specify the port to bind to")
	rootCmd.AddCommand(createRawSocket, bindToDevice)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
