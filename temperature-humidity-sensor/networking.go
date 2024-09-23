package main

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/netip"
	"time"

	"github.com/soypat/cyw43439"
	"github.com/soypat/seqs"
	"github.com/soypat/seqs/eth/dhcp"
	"github.com/soypat/seqs/httpx"
	"github.com/soypat/seqs/stacks"
)

//
// Things here are largely based on the "http-client" example from the
// soypat/cyw43439 Github repo, which the work-in-progress repo for supporting
// the Pi Pico W wireless networking on tinygo. Here's the specific version I
// used as reference:
// https://github.com/soypat/cyw43439/tree/a62ee4027d66bc0f92d4f7bc3902627fb8e6ed6b/examples/http-client
//

const (
	// We need one TCP port to make one HTTP request at a time.
	tcpPortsCount = 1

	// We need two UDP ports: one for DNS, one for DHCP.
	udpPortsCount = 2
)

// TODO: At some point I want to have all that as a background thing. Like, keep
// showing things on the display while trying to connect to the Internet.
//
// TODO: BTW, "showing things on the display" includes some tiny text showing
// the connection status while it is not fully working.
func createStack() (*stacks.PortStack, *stacks.DHCPClient, error) {
	logger.Info("Creating the networking stack")

	// Wifi config.
	wifiCfg := cyw43439.DefaultWifiConfig()
	wifiCfg.Logger = logger

	// Initialize Pico W device.
	logger.Info("Initializing Pico W device")
	devInitStartTime := time.Now()
	dev := cyw43439.NewPicoWDevice()
	if dev == nil {
		err := errors.New("got a nil device")
		logger.Error("Creating the Pico W device", slog.String("err", err.Error()))
		return nil, nil, err
	}
	err := dev.Init(wifiCfg)
	if err != nil {
		logger.Error("Initializing the Pico W device", slog.String("err", err.Error()))
		return nil, nil, fmt.Errorf("Pico W device init failed: %w", err)
	}

	macAddress, _ := dev.HardwareAddr6()
	logger.Info("Pico W device initialized",
		slog.Duration("duration", time.Since(devInitStartTime)),
		slog.String("mac", net.HardwareAddr(macAddress[:]).String()),
	)

	// Connect to Wifi.
	logger.Info("Connecting to WiFi", slog.String("ssid", wifiSSID), slog.Int("passwordLen", len(wifiPassword)))
	for {
		err = dev.JoinWPA2(wifiSSID, wifiPassword)
		if err == nil {
			break
		}
		logger.Error("connecting to WiFi", slog.String("err", err.Error()))
		time.Sleep(5 * time.Second)
	}
	logger.Info("Connected to WiFi")

	// Create the "port stack".
	stack := stacks.NewPortStack(stacks.PortStackConfig{
		MAC:             macAddress,
		MaxOpenPortsUDP: udpPortsCount,
		MaxOpenPortsTCP: tcpPortsCount,
		MTU:             mtu,
		Logger:          logger,
	})

	if stack == nil {
		err = errors.New("got a nil PortStack")
		logger.Error("Creating the port stack", slog.String("err", err.Error()))
		return nil, nil, err
	}

	// Handle packets.
	dev.RecvEthHandle(stack.RecvEth)
	go nicLoop(dev, stack)

	// Request important stuff via DHCP.
	dhcpClient := stacks.NewDHCPClient(stack, dhcp.DefaultClientPort)
	err = dhcpClient.BeginRequest(stacks.DHCPRequestConfig{
		// The original code set teo additional fields here: `RequestedAddr` and
		// `Hostname`. I am skipping these intentionally. I am not experienced
		// with DHCP, but from what I saw, `RequestedAddr` is used when we want
		// to ask for a specific IP address; not our case here, any will do. And
		// `Hostname` is our own hostname, which the DHCP server could use for
		// whatever reason, but doesn't make much sense in this case (I intend
		// to have several devices running the same firmware, and I don't intend
		// to make things like the host name configurable).
		Xid: uint32(time.Now().Nanosecond()),
	})
	if err != nil {
		logger.Error("Beginning DHCP request", slog.String("err", err.Error()))
		return stack, dhcpClient, errors.New("DHCP begin request:" + err.Error())
	}
	i := 0
	for dhcpClient.State() != dhcp.StateBound {
		i++
		logger.Info("DHCP ongoing...")
		time.Sleep(time.Second / 2)
		if i > 15 {
			err = errors.New("DHCP did not complete")
			logger.Error("DHCP request", slog.String("err", err.Error()))
			return stack, nil, err
		}
	}
	var primaryDNS netip.Addr
	dnsServers := dhcpClient.DNSServers()
	if len(dnsServers) > 0 {
		primaryDNS = dnsServers[0]
	} else {
		logger.Warn("Failed to get a DNS server via DHCP")
	}
	ip := dhcpClient.Offer()
	logger.Info("DHCP complete",
		slog.Uint64("cidrbits", uint64(dhcpClient.CIDRBits())),
		slog.String("ourIP", ip.String()),
		slog.String("dns", primaryDNS.String()),
		slog.String("broadcast", dhcpClient.BroadcastAddr().String()),
		slog.String("gateway", dhcpClient.Gateway().String()),
		slog.String("router", dhcpClient.Router().String()),
		slog.String("dhcp", dhcpClient.DHCPServer().String()),
		slog.String("hostname", string(dhcpClient.Hostname())),
		slog.Duration("lease", dhcpClient.IPLeaseTime()),
		slog.Duration("renewal", dhcpClient.RenewalTime()),
		slog.Duration("rebinding", dhcpClient.RebindingTime()),
	)

	stack.SetAddr(ip) // It's important to set the IP address after DHCP completes.

	return stack, dhcpClient, nil
}

func nicLoop(dev *cyw43439.Device, stack *stacks.PortStack) {
	// Maximum number of packets to queue before sending them.
	const (
		queueSize                = 3
		maxRetriesBeforeDropping = 3
	)

	var queue [queueSize][mtu]byte
	var lenBuf [queueSize]int
	var retries [queueSize]int

	markSent := func(i int) {
		queue[i] = [mtu]byte{} // Not really necessary.
		lenBuf[i] = 0
		retries[i] = 0
	}

	for {
		stallRx := true

		// Poll for incoming packets.
		for i := 0; i < 1; i++ {
			gotPacket, err := dev.PollOne()
			if err != nil {
				logger.Error("Poll error in NIC loop", slog.String("err", err.Error()))
			}
			if !gotPacket {
				break
			}
			stallRx = false
		}

		// Queue packets to be sent.
		for i := range queue {
			if retries[i] != 0 {
				continue // Packet currently queued for retransmission.
			}
			var err error
			buf := queue[i][:]
			lenBuf[i], err = stack.HandleEth(buf[:])
			if err != nil {
				logger.Error("Ethernet handling error in NIC loop",
					slog.String("err", err.Error()),
					slog.Int("lenBuf[i]", lenBuf[i]),
				)
				lenBuf[i] = 0
				continue
			}
			if lenBuf[i] == 0 {
				break
			}
		}
		stallTx := lenBuf == [queueSize]int{}
		if stallTx {
			if stallRx {
				// Avoid busy waiting when both Rx and Tx stall.
				time.Sleep(51 * time.Millisecond)
			}
			continue
		}

		// Send queued packets.
		for i := range queue {
			n := lenBuf[i]
			if n <= 0 {
				continue
			}
			err := dev.SendEth(queue[i][:n])
			if err != nil {
				// Queue packet for retransmission.
				retries[i]++
				if retries[i] > maxRetriesBeforeDropping {
					markSent(i)
					logger.Error("Dropped outgoing packet in NIC loop", slog.String("err", err.Error()))
				}
			} else {
				markSent(i)
			}
		}
	}
}

// resolveHardwareAddr obtains the hardware address of the given IP address.
func resolveHardwareAddr(stack *stacks.PortStack, ip netip.Addr) ([6]byte, error) {
	if !ip.IsValid() {
		return [6]byte{}, errors.New("invalid ip")
	}
	arpClient := stack.ARP()
	arpClient.Abort() // Remove any previous ARP requests.
	err := arpClient.BeginResolve(ip)
	if err != nil {
		return [6]byte{}, err
	}
	time.Sleep(4 * time.Millisecond)

	// ARP exchanges should be fast, don't wait too long for them.
	const timeout = time.Second
	const maxRetries = 20
	retries := maxRetries
	for !arpClient.IsDone() && retries > 0 {
		retries--
		if retries == 0 {
			return [6]byte{}, errors.New("arp timed out")
		}
		time.Sleep(timeout / maxRetries)
	}
	_, hw, err := arpClient.ResultAs6()
	return hw, err
}

// TODO: Too much hardcoded stuff here!
func makeRequest(stack *stacks.PortStack, dhcpClient *stacks.DHCPClient) {
	start := time.Now()

	svAddr, err := netip.ParseAddrPort(serverAddrStr)
	if err != nil {
		panic("parsing server address:" + err.Error())
	}

	// Resolver router's hardware address to dial outside our network to internet.
	routerMAC, err := resolveHardwareAddr(stack, dhcpClient.Router())
	if err != nil {
		panic("router hwaddr resolving:" + err.Error()) // xxxxxxxxxxxxx TODO: don't panic!
	}
	logger.Info("Got the router MAC address", slog.String("mac", net.HardwareAddr(routerMAC[:]).String()))

	rng := rand.New(rand.NewSource(int64(time.Now().Sub(start))))

	// Start TCP server.
	clientAddr := netip.AddrPortFrom(stack.Addr(), uint16(rng.Intn(65535-1024)+1024))
	conn, err := stacks.NewTCPConn(stack, stacks.TCPConnConfig{
		TxBufSize: tcpBufSize,
		RxBufSize: tcpBufSize,
	})

	if err != nil {
		panic("conn create:" + err.Error())
	}

	closeConn := func(err string) {
		slog.Error("tcpconn:closing", slog.String("err", err))
		conn.Close()
		for !conn.State().IsClosed() {
			slog.Info("tcpconn:waiting", slog.String("state", conn.State().String()))
			time.Sleep(1000 * time.Millisecond)
		}
	}

	// Here we create the HTTP request and generate the bytes. The Header method
	// returns the raw header bytes as should be sent over the wire.
	var req httpx.RequestHeader
	req.SetRequestURI("/")
	// If you need a Post request change "GET" to "POST" and then add the
	// post data to reqBytes: `postReq := append(reqBytes, postData...)` and send postReq over TCP.
	req.SetMethod("GET")
	req.SetHost(svAddr.Addr().String())
	req.SetHost("example.com")
	// req.SetHost("pudim.com.br")
	reqBytes := req.Header()

	logger.Info("tcp:ready",
		slog.String("clientAddr", clientAddr.String()),
		slog.String("serverAddr", serverAddrStr),
	)
	rxBuf := make([]byte, 1024*10)
	for {
		time.Sleep(5 * time.Second)
		slog.Info("dialing", slog.String("serverAddr", serverAddrStr))

		// Make sure to timeout the connection if it takes too long.
		conn.SetDeadline(time.Now().Add(connTimeout))
		err = conn.OpenDialTCP(clientAddr.Port(), routerMAC, svAddr, seqs.Value(rng.Intn(65535-1024)+1024))
		if err != nil {
			closeConn("opening TCP: " + err.Error())
			continue
		}
		slog.Info("LMB: Opened connection!")
		retries := 50
		for conn.State() != seqs.StateEstablished && retries > 0 {
			time.Sleep(100 * time.Millisecond)
			retries--
		}
		slog.Info("LMB: Disabling deadline!")
		conn.SetDeadline(time.Time{}) // Disable the deadline.
		if retries == 0 {
			closeConn("tcp establish retry limit exceeded")
			continue
		}

		// Send the request.
		slog.Info("LMB: Sending the request!")
		_, err = conn.Write(reqBytes)
		if err != nil {
			closeConn("writing request: " + err.Error())
			continue
		}
		slog.Info("LMB: Sleep 1111111!")
		time.Sleep(500 * time.Millisecond)
		conn.SetDeadline(time.Now().Add(connTimeout))
		slog.Info("LMB: Reading response")
		n, err := conn.Read(rxBuf)
		if n == 0 && err != nil {
			closeConn("reading response: " + err.Error())
			continue
		} else if n == 0 {
			closeConn("no response")
			continue
		}
		println("got HTTP response!")
		println(string(rxBuf[:n]))
		closeConn("done")
		return // exit program.
	}
}
