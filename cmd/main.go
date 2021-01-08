package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"golang.org/x/sys/unix"
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

	rootCmd.AddCommand(createRawSocket)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
