package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/soypat/cyw43439"
	"github.com/soypat/seqs"
	"github.com/soypat/seqs/eth/dhcp"
	"github.com/soypat/seqs/eth/dns"
	"github.com/soypat/seqs/httpx"
	"github.com/soypat/seqs/stacks"
)

//
// Things on this file are largely based on the "http-client" example from the
// soypat/cyw43439 Github repo, which the work-in-progress repo for supporting
// the Pi Pico W wireless networking on tinygo. Here's the specific version I
// used as reference:
// https://github.com/soypat/cyw43439/tree/a62ee4027d66bc0f92d4f7bc3902627fb8e6ed6b/examples/http-client
//

//
// Public interface
//

var (
	// WiFiNotReadyError is returned to indicate that an operation cannot be
	// completed because the WiFi device or connection isn't ready yet.
	WiFiNotReadyError = errors.New("WiFi not ready")
)

// PicoNetStatus represents the status of a PicoNet.
//
// Things on PicoNet are initialized sequentially. It goes from status to status
// in the order they are declared below. So, knowing the current status allows
// to know where in the initialization sequence we are. And if we spend too much
// time on the same state, it probably means that some error is happening in the
// next step of the initialization process.
type PicoNetStatus int

const (
	StatusUninitialized PicoNetStatus = iota
	StatusCreatingDevice
	StatusConnectingToWiFi
	StatusCreatingStack
	StatusObtainingIP
	StatusConfiguringDNS
	StatusObtainingRouterMAC
	StatusReadyToGo
)

func (s PicoNetStatus) String() string {
	switch s {
	case StatusUninitialized:
		return "Uninitialized"
	case StatusCreatingDevice:
		return "CreatingDevice"
	case StatusConnectingToWiFi:
		return "ConnectingToWiFi"
	case StatusCreatingStack:
		return "CreatingStack"
	case StatusObtainingIP:
		return "ObtainingIP"
	case StatusConfiguringDNS:
		return "ConfiguringDNS"
	case StatusObtainingRouterMAC:
		return "ObtainingRouterMAC"
	case StatusReadyToGo:
		return "ReadyToGo"
	default:
		return "Invalid"
	}
}

// PicoNet is *the* interface to do networking stuff on a Raspberry Pi Pico W --
// at least on this humble program! :-)
//
// You should create just one of those. I mean, the code doesn't really check
// how many instances do exist, and it may even work with multiple instances,
// but that's not tested and there's no reason to have more than one!
type PicoNet struct {
	// mutex protects all relevant operations performed on a PicoNet.
	mutex sync.Mutex

	// logger is used internally for all the logging.
	logger *slog.Logger

	// status tells how things are.
	status PicoNetStatus

	// device is the Raspberry Pi Pico W WiFi device.
	device *cyw43439.Device

	// stack is the network stack used internally.
	stack *stacks.PortStack

	// dhcpClient is the DHCP client used internally.
	dhcpClient *stacks.DHCPClient

	// dnsClient is used to resolve names.
	dnsClient *stacks.DNSClient

	// dnsIP is the IP address of the primary DNS server. We currently don't try
	// to use any DNS server other than the primary one.
	dnsIP netip.Addr

	// picoMAC is the MAC address of the Pico W.
	picoMAC [6]byte

	// routerMAC is the MAC address of the router the Pico W is connected to.
	// We'll send our packets to it.
	routerMAC [6]byte
}

// NewPicoNet creates a new PicoNet and starts the background initialization
// process.
//
// The background initialization process will keep retrying any failing
// operations, even if some of them are pretty much guaranteed to fail again.
// You should check the initialization progress with PicoNet.Status() and handle
// long-running initialization errors as desired.
func NewPicoNet(logger *slog.Logger) *PicoNet {
	pn := &PicoNet{
		logger: logger,
	}

	go func() {
		pn.setStatus(StatusCreatingDevice)
		pn.createDevice()

		pn.setStatus(StatusConnectingToWiFi)
		pn.connectToWifi()

		pn.setStatus(StatusCreatingStack)
		pn.createStack()

		pn.setStatus(StatusObtainingIP)
		pn.obtainIPAddress()

		pn.setStatus(StatusConfiguringDNS)
		pn.configureDNS()

		pn.setStatus(StatusObtainingRouterMAC)
		pn.obtainRouterMAC()

		pn.setStatus(StatusReadyToGo)
	}()
	return pn
}

func (pn *PicoNet) Status() PicoNetStatus {
	pn.mutex.Lock()
	defer pn.mutex.Unlock()
	return pn.status
}

// Get does an HTTP GET request.
func (pn *PicoNet) Get(urlStr string) (resp *Response, err error) {
	rawRes, body, err := pn.doRequest("GET", urlStr, []byte{})
	if err != nil {
		return nil, err
	}

	// These mappings between `rawRes` and `res` look completely nuts, I know.
	// It turns out that the low-level networking code I am using (the `seqs`
	// library) seems to be in a very early stage of development, and therefore
	// it can't properly parse HTTP responses. What I am doing in `doRequest()`
	// is effectively to parse the HTTP response as if it were an HTTP request,
	// and then reading the information I want from the request fields that by
	// coincidence match the wanted response fields.
	statusCode, err := strconv.ParseInt(string(rawRes.Hdr.RequestURI()), 10, 32)
	if err != nil {
		pn.logger.Warn("Parsing HTTP status code", slogError(err))
		statusCode = 0
	}

	res := &Response{
		Status:        string(rawRes.Hdr.RequestURI()) + " " + string(rawRes.Hdr.Protocol()),
		StatusCode:    int(statusCode),
		Proto:         string(rawRes.Hdr.Method()),
		Headers:       rawRes.Hdr.GetAll(),
		ContentLength: rawRes.Hdr.ContentLength(),
		Body:          body,
	}

	return res, nil
}

// xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
func (pn *PicoNet) Post() (resp *Response, err error) {
	return nil, nil
}

// Response is the response from an HTTP request. This ain't no standard http
// package, so don't expect super standard-respecting parsing of a response.
// Just to give one example: this will not handle duplicate headers nicely.
type Response struct {
	// Status contains the response status, like "200 OK".
	Status string

	// StatusCode contains the status code, like 200.
	StatusCode int

	// Proto contains the protocol version, like "HTTP/1.0".
	Proto string

	// Headers maps header keys to their values.
	Headers map[string]string

	// Body contains the response body.
	Body []byte

	// ContentLength contains the length of the associated content.
	ContentLength int
}

//
// Initialization
//

const (
	// We need one TCP port to make one HTTP request at a time.
	tcpPortsCount = 1

	// We need two UDP ports: one for DNS, one for DHCP.
	udpPortsCount = 2

	// Use the MTU for the WiFi device.
	mtu = cyw43439.MTU
)

func (pn *PicoNet) createDevice() {
	for {
		startTime := time.Now()

		// Create the Pico W device.
		pn.logger.Info("Creating the WiFi device")
		pn.device = cyw43439.NewPicoWDevice()
		if pn.device == nil {
			pn.logger.Error("Got a nil WiFi device")

			// I think that retrying here unlikely to succeed, but I also don't
			// see much else we could do. Rebooting the device would not be a
			// bad idea, but this is better done by the caller.
			time.Sleep(5 * time.Second)
			continue
		}

		pn.logger.Info("WiFi device created successfully", slogTook(startTime))
		break
	}

	for {
		startTime := time.Now()

		// Initialize the Pico W device.
		pn.logger.Info("Initializing the WiFi device")
		wifiCfg := cyw43439.DefaultWifiConfig()
		wifiCfg.Logger = pn.logger

		err := pn.device.Init(wifiCfg)
		if err != nil {
			pn.logger.Error("Initializing the WiFi device", slogError(err))
			time.Sleep(5 * time.Second)
			continue
		}

		pn.picoMAC, err = pn.device.HardwareAddr6()
		if err != nil {
			pn.logger.Error("Obtaining the WiFi device MAC address", slogError(err))
			time.Sleep(5 * time.Second)
			continue
		}

		pn.logger.Info("Pico W device successfully initialized", slogTook(startTime), slogMAC(pn.picoMAC))

		break
	}
}

func (pn *PicoNet) connectToWifi() {
	for {
		startTime := time.Now()
		pn.logger.Info("Connecting to WiFi", slog.String("ssid", wifiSSID), slog.Int("passwordLen", len(wifiPassword)))
		err := pn.device.JoinWPA2(wifiSSID, wifiPassword)
		if err != nil {
			pn.logger.Error("Connecting to WiFi", slogError(err))
			time.Sleep(5 * time.Second)
			continue
		}
		pn.logger.Info("Successfully Connected to WiFi", slogTook(startTime))
		break
	}
}

func (pn *PicoNet) createStack() {
	for {
		startTime := time.Now()

		pn.logger.Info("Creating the port stack")
		pn.stack = stacks.NewPortStack(stacks.PortStackConfig{
			MAC:             pn.picoMAC,
			MaxOpenPortsUDP: udpPortsCount,
			MaxOpenPortsTCP: tcpPortsCount,
			MTU:             mtu,
			Logger:          pn.logger,
		})

		if pn.stack == nil {
			pn.logger.Error("Got a nil port stack")
			time.Sleep(5 * time.Second)
			continue
		}

		pn.device.RecvEthHandle(pn.stack.RecvEth)
		go pn.nicLoop()

		pn.logger.Info("Successfully created port stack", slogTook(startTime))
		break
	}
}

// TODO: This assumes that we never need to renewal the IP address we received
// from DHCP. I think this is not correct.
func (pn *PicoNet) obtainIPAddress() {
	for {
		startTime := time.Now()
		pn.logger.Info("Creating DHCP client")
		pn.dhcpClient = stacks.NewDHCPClient(pn.stack, dhcp.DefaultClientPort)
		if pn.dhcpClient == nil {
			pn.logger.Error("Got a nil DHCP client")
			time.Sleep(5 * time.Second)
			continue
		}

		pn.logger.Info("Successfully created DHCP client", slogTook(startTime))
		break
	}

	for {
		startTime := time.Now()
		pn.logger.Info("Starting DHCP request")

		err := pn.dhcpClient.BeginRequest(stacks.DHCPRequestConfig{
			// The original code set two additional fields here: `RequestedAddr`
			// and `Hostname`. I am skipping these intentionally. I am not
			// experienced with DHCP, but from what I saw, `RequestedAddr` is
			// used when we want to ask for a specific IP address; not our case
			// here, any will do. And `Hostname` is our own hostname, which the
			// DHCP server could use for whatever reason, but doesn't make much
			// sense in this case (I intend to have several devices running the
			// same firmware, and I don't intend to make things like the host
			// name configurable).
			Xid: uint32(time.Now().Nanosecond()),
		})

		if err != nil {
			pn.logger.Error("Starting DHCP request", slogError(err))
			time.Sleep(5 * time.Second)
			continue
		}

		pn.logger.Info("Successfully started DHCP request", slogTook(startTime))
		break
	}

	for {
		startTime := time.Now()

		const maxRetries = 15
		retries := maxRetries
		for pn.dhcpClient.State() != dhcp.StateBound {
			retries--
			pn.logger.Info("DHCP ongoing...")
			if retries == 0 {
				pn.logger.Error("DHCP did not complete")
				time.Sleep(5 * time.Second)
				continue
			}
			time.Sleep(time.Second / 2)
		}

		var primaryDNS netip.Addr
		dnsServers := pn.dhcpClient.DNSServers()
		if len(dnsServers) > 0 {
			primaryDNS = dnsServers[0]
		}

		// We've got an IP address!
		ip := pn.dhcpClient.Offer()
		pn.stack.SetAddr(ip)

		pn.logger.Info("Successfully completed the DHCP request",
			slog.Uint64("cidrBits", uint64(pn.dhcpClient.CIDRBits())),
			slog.String("ourIP", ip.String()),
			slog.String("dns", primaryDNS.String()),
			slog.String("broadcast", pn.dhcpClient.BroadcastAddr().String()),
			slog.String("gateway", pn.dhcpClient.Gateway().String()),
			slog.String("router", pn.dhcpClient.Router().String()),
			slog.String("dhcp", pn.dhcpClient.DHCPServer().String()),
			slog.String("hostname", string(pn.dhcpClient.Hostname())),
			slog.Duration("lease", pn.dhcpClient.IPLeaseTime()),
			slog.Duration("renewal", pn.dhcpClient.RenewalTime()),
			slog.Duration("rebinding", pn.dhcpClient.RebindingTime()),
			slogTook(startTime),
		)
		break
	}
}

func (pn *PicoNet) configureDNS() {
	for {
		startTime := time.Now()
		pn.logger.Info("Configuring DNS")

		dnsServers := pn.dhcpClient.DNSServers()

		if len(dnsServers) == 0 || !dnsServers[0].IsValid() {
			// This is one case in which retrying is pointless. We do follow the
			// same pattern, nevertheless, to make error handling consistent.
			pn.logger.Error("Didn't get any DNS server via DHCP")
			time.Sleep(5 * time.Second)
			continue
		}

		pn.dnsClient = stacks.NewDNSClient(pn.stack, dns.ClientPort)
		pn.dnsIP = dnsServers[0]

		pn.logger.Info("Successfully configured DNS", slogTook(startTime))
		break
	}
}

func (pn *PicoNet) obtainRouterMAC() {
	for {
		startTime := time.Now()
		pn.logger.Info("Obtaining router MAC address")

		var err error
		pn.routerMAC, err = resolveHardwareAddr(pn.stack, pn.dhcpClient.Router())
		if err != nil {
			pn.logger.Error("Obtaining router MAC address", slogError(err))
			time.Sleep(5 * time.Second)
			continue
		}

		pn.logger.Info("Successfully obtained the router MAC address", slogMAC(pn.routerMAC), slogTook(startTime))
		break
	}
}

//
// Helpers
//

func (pn *PicoNet) setStatus(s PicoNetStatus) {
	pn.mutex.Lock()
	defer pn.mutex.Unlock()
	pn.status = s
}

func (pn *PicoNet) lookupNetIP(host string) ([]netip.Addr, error) {
	name, err := dns.NewName(host)
	if err != nil {
		return nil, err
	}

	err = pn.dnsClient.StartResolve(pn.dnsConfig(name))
	if err != nil {
		return nil, err
	}
	defer pn.stack.CloseUDP(dns.ClientPort)

	time.Sleep(5 * time.Millisecond)

	// 100 retries with 50ms delays gives us 5s to resolve. Should be more than
	// enough even with really bad networking.
	retries := 100
	for retries > 0 {
		done, _ := pn.dnsClient.IsDone()
		if done {
			break
		}
		retries--
		time.Sleep(50 * time.Millisecond)
	}
	done, retCode := pn.dnsClient.IsDone()
	if !done && retries == 0 {
		return nil, errors.New("DNS lookup timed out")
	} else if retCode != dns.RCodeSuccess {
		return nil, errors.New("DNS lookup failed:" + retCode.String())
	}
	answers := pn.dnsClient.Answers()
	if len(answers) == 0 {
		return nil, errors.New("no DNS answers")
	}
	var addrs []netip.Addr
	for i := range answers {
		data := answers[i].RawData()
		if len(data) == 4 {
			addrs = append(addrs, netip.AddrFrom4([4]byte(data)))
		}
	}
	if len(addrs) == 0 {
		return nil, errors.New("no IPv4 DNS answers")
	}
	return addrs, nil
}

func (pn *PicoNet) dnsConfig(name dns.Name) stacks.DNSResolveConfig {
	return stacks.DNSResolveConfig{
		Questions: []dns.Question{
			{
				Name:  name,
				Type:  dns.TypeA,
				Class: dns.ClassINET,
			},
		},
		DNSAddr:         pn.dnsIP,     // Send DNS the request to this server...
		DNSHWAddr:       pn.routerMAC, // ...through our router.
		EnableRecursion: true,
	}
}

func (pn *PicoNet) nicLoop() {
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
			gotPacket, err := pn.device.PollOne()
			if err != nil {
				pn.logger.Error("Poll error in NIC loop", slogError(err))
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
			lenBuf[i], err = pn.stack.HandleEth(buf[:])
			if err != nil {
				pn.logger.Error("Ethernet handling error in NIC loop",
					slogError(err),
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
			err := pn.device.SendEth(queue[i][:n])
			if err != nil {
				// Queue packet for retransmission.
				retries[i]++
				if retries[i] > maxRetriesBeforeDropping {
					markSent(i)
					pn.logger.Error("Dropped outgoing packet in NIC loop", slogError(err))
				}
			} else {
				markSent(i)
			}
		}
	}
}

// Pretty bad naming here, and possibly because this function is trying to do
// too much. Anyway, takes something like "http://example.com/whatever" or
// "http://192.168.171.171:8080" and returns the important bits in data types we
// can actually use. And, yes, this includes making a DNS request if necessary.
func (pn *PicoNet) getUsableAddress(urlStr string) (addrPort netip.AddrPort, host, path string, err error) {

	if !strings.HasPrefix(urlStr, "http://") {
		err = errors.New("URL must use the http scheme")
		return
	}

	u, err := url.Parse(urlStr)

	path = u.Path
	host = u.Hostname()
	strPort := u.Port()
	if strPort == "" {
		strPort = "80"
	}

	hostPort := host + ":" + strPort

	uint64Port, err := strconv.ParseUint(strPort, 10, 16)
	if err != nil {
		err = fmt.Errorf("resolving %q: %w", host, err)
		return
	}

	uint16Port := uint16(uint64Port)

	isIP := net.ParseIP(host) != nil
	if isIP {
		addrPort, err = netip.ParseAddrPort(hostPort)
		return
	}

	// The passed URL does not use an IP directly, so we need to make a DNS
	// request.
	addrs, err := pn.lookupNetIP(host)
	if err != nil {
		err = fmt.Errorf("resolving %q: %w", host, err)
		return
	}

	// lookupNetIP will return an error if it can't get any IPv4 addresses, so
	// it's guaranteed that addrs[0] will contain something!
	addrPort = netip.AddrPortFrom(addrs[0], uint16Port)

	return
}

func (pn *PicoNet) doRequest(method, urlStr string, reqBody []byte) (resHeader *httpx.ResponseHeader, resBody []byte, err error) {
	const connTimeout = 5 * time.Second
	const tcpBufSize = 2030 // MTU - ethhdr - iphdr - tcphdr

	addrPort, host, path, err := pn.getUsableAddress(urlStr)
	if err != nil {
		pn.logger.Error("Preparing request", slogError(err))
		return nil, nil, err
	}

	// Create the TCP connection, set this up so it gets closed eventually.
	clientAddr := netip.AddrPortFrom(pn.stack.Addr(), uint16(rand.Intn(65535-1024)+1024))
	conn, err := stacks.NewTCPConn(pn.stack, stacks.TCPConnConfig{
		TxBufSize: tcpBufSize,
		RxBufSize: tcpBufSize,
	})

	if err != nil {
		panic("conn create:" + err.Error())
	}

	defer func() {
		err := conn.Close()
		if err != nil {
			pn.logger.Error("Closing TCP connection", slogError(err))
			return
		}
		for !conn.State().IsClosed() {
			pn.logger.Info("Waiting for TCP connection to close", slog.String("state", conn.State().String()))
			time.Sleep(1000 * time.Millisecond)
		}
		pn.logger.Info("TCP connection closed")
	}()

	// Here we create the HTTP request and generate the bytes. The Header method
	// returns the raw header bytes as should be sent over the wire.
	var req httpx.RequestHeader
	req.SetRequestURI(path)

	// xxxxxxxxxxxxxxxxxxx Handle body!
	// If you need a Post request change "GET" to "POST" and then add the post
	// data to reqBytes: `postReq := append(reqBytes, postData...)` and send
	// postReq over TCP.
	req.SetMethod(method)
	req.SetHost(host)
	reqBytes := req.Header()

	pn.logger.Info("TCP connection ready, now dialing",
		slog.String("clientAddr", clientAddr.String()),
		slog.String("serverAddr", urlStr),
		slog.String("serverIPPort", addrPort.String()),
	)

	// Make sure to timeout the connection if it takes too long.
	conn.SetDeadline(time.Now().Add(connTimeout))
	err = conn.OpenDialTCP(clientAddr.Port(), pn.routerMAC, addrPort, seqs.Value(rand.Intn(65535-1024)+1024))
	if err != nil {
		pn.logger.Error("Opening TCP connection", slogError(err))
		return nil, nil, fmt.Errorf("opening TCP connection: %w", err)
	}

	retries := 50
	for conn.State() != seqs.StateEstablished && retries > 0 {
		time.Sleep(100 * time.Millisecond)
		retries--
	}

	// xxxxxxxxxxxxx Disable the deadline when sending data?! I don't think I want to do that!
	conn.SetDeadline(time.Time{}) // Disable the deadline.
	if retries == 0 {
		pn.logger.Error("Retry limit exceeded opening TCP connection")
		return nil, nil, errors.New("retry limit exceeded opening TCP connection")
	}

	// Send the request.
	_, err = conn.Write(reqBytes)
	if err != nil {
		pn.logger.Error("Writing request", slogError(err))
		return nil, nil, fmt.Errorf("writing request: %w", err)
	}

	// xxxxxxxxxxxxxxx TODO: This Sleep() is fishy, right?
	time.Sleep(500 * time.Millisecond)
	conn.SetDeadline(time.Now().Add(connTimeout))

	br := bufio.NewReader(conn)

	resHeader = &httpx.ResponseHeader{}
	err = resHeader.Hdr.Read(br)
	if err != nil {
		pn.logger.Error("Reading response", slogError(err))
		return nil, nil, fmt.Errorf("reading response: %w", err)
	}

	resBody = make([]byte, resHeader.Hdr.ContentLength())
	_, err = io.ReadFull(br, resBody) // xxxxxxxxxx check n?
	if err != nil {
		pn.logger.Error("Reading response body", slogError(err))
		return nil, nil, err
	}

	return resHeader, resBody, nil
}

func (pn *PicoNet) translateHeaders(urlStr string) (resp *Response, err error) {
	rawRes, body, err := pn.doRequest("GET", urlStr, []byte{})
	if err != nil {
		return nil, err
	}

	// These mappings between `rawRes` and `res` look completely nuts, I know.
	// It turns out that the low-level networking code I am using (the `seqs`
	// library) seems to be in a very early stage of development, and therefore
	// it can't properly parse HTTP responses. What I am doing in `doRequest()`
	// is effectively to parse the HTTP response as if it were an HTTP request,
	// and then reading the information I want from the request fields that by
	// coincidence match the wanted response fields.
	statusCode, err := strconv.ParseInt(string(rawRes.Hdr.RequestURI()), 10, 32)
	if err != nil {
		pn.logger.Warn("Parsing HTTP status code", slogError(err))
		statusCode = 0
	}

	res := &Response{
		Status:        string(rawRes.Hdr.RequestURI()) + " " + string(rawRes.Hdr.Protocol()),
		StatusCode:    int(statusCode),
		Proto:         string(rawRes.Hdr.Method()),
		Headers:       rawRes.Hdr.GetAll(),
		ContentLength: rawRes.Hdr.ContentLength(),
		Body:          body,
	}

	return res, nil
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

//
// Logging helpers
//

func slogTook(start time.Time) slog.Attr {
	return slog.Duration("took", time.Since(start))
}

func slogMAC(mac [6]byte) slog.Attr {
	return slog.String("mac", net.HardwareAddr(mac[:]).String())
}

func slogError(err error) slog.Attr {
	errString := "<nil>"
	if err != nil {
		errString = err.Error()
	}
	return slog.String("err", errString)
}
