// Copyright (C) 2026 mieru authors.
// Adapted for V2bX from github.com/enfein/mbox under GPL-3.0.
//
// Package mieru adapts the official Mieru server API to a sing-box inbound.
package mieru

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/uot"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	mierucommon "github.com/enfein/mieru/v3/apis/common"
	mieruconstant "github.com/enfein/mieru/v3/apis/constant"
	mierumodel "github.com/enfein/mieru/v3/apis/model"
	mieruserver "github.com/enfein/mieru/v3/apis/server"
	mierutp "github.com/enfein/mieru/v3/apis/trafficpattern"
	mierupb "github.com/enfein/mieru/v3/pkg/appctl/appctlpb"
	"google.golang.org/protobuf/proto"
)

const Type = "mieru"

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[Options](registry, Type, NewInbound)
}

type Inbound struct {
	inbound.Adapter
	ctx      context.Context
	router   adapter.ConnectionRouterEx
	logger   log.ContextLogger
	listener *listener.Listener
	server   mieruserver.Server
	mu       sync.Mutex
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options Options) (adapter.Inbound, error) {
	config, err := buildServerConfig(options)
	if err != nil {
		return nil, fmt.Errorf("failed to build mieru server config: %w", err)
	}
	server := mieruserver.NewServer()
	if err = server.Store(config); err != nil {
		return nil, fmt.Errorf("failed to store mieru server config: %w", err)
	}
	instance := &Inbound{
		Adapter: inbound.NewAdapter(Type, tag),
		ctx:     ctx,
		router:  uot.NewRouter(router, logger),
		logger:  logger,
		server:  server,
	}
	instance.listener = listener.New(listener.Options{
		Context: ctx,
		Logger:  logger,
		Network: []string{N.NetworkTCP, N.NetworkUDP},
		Listen:  options.ListenOptions,
	})
	return instance, nil
}

func (h *Inbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.server.Start(); err != nil {
		return fmt.Errorf("failed to start mieru server: %w", err)
	}
	go h.acceptLoop()
	return nil
}

func (h *Inbound) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.server.IsRunning() {
		return h.server.Stop()
	}
	return nil
}

func (h *Inbound) acceptLoop() {
	for {
		conn, request, err := h.server.Accept()
		if err != nil {
			if !h.server.IsRunning() {
				return
			}
			h.logger.Debug("failed to accept mieru connection: ", err)
			continue
		}
		go h.handleConnection(conn, request)
	}
}

func (h *Inbound) handleConnection(conn net.Conn, request *mierumodel.Request) {
	ctx := log.ContextWithNewID(h.ctx)
	response := &mierumodel.Response{
		Reply: mieruconstant.Socks5ReplySuccess,
		BindAddr: mierumodel.AddrSpec{
			IP: net.IPv4zero,
		},
	}
	if err := response.WriteToSocks5(conn); err != nil {
		_ = conn.Close()
		return
	}
	var metadata adapter.InboundContext
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	metadata.InboundDetour = h.listener.ListenOptions().Detour
	metadata.UDPDisableDomainUnmapping = h.listener.ListenOptions().UDPDisableDomainUnmapping
	if remoteAddr := conn.RemoteAddr(); remoteAddr != nil {
		metadata.Source = M.SocksaddrFromNet(remoteAddr)
	}
	if request.DstAddr.FQDN != "" {
		metadata.Destination = M.Socksaddr{Fqdn: request.DstAddr.FQDN, Port: uint16(request.DstAddr.Port)}
	} else if request.DstAddr.IP != nil {
		addr, _ := netip.AddrFromSlice(request.DstAddr.IP)
		metadata.Destination = M.Socksaddr{Addr: addr.Unmap(), Port: uint16(request.DstAddr.Port)}
	}
	if userContext, ok := conn.(mierucommon.UserContext); ok {
		metadata.User = userContext.UserName()
	}
	switch request.Command {
	case mieruconstant.Socks5ConnectCmd:
		h.router.RouteConnectionEx(ctx, conn, metadata, nil)
	case mieruconstant.Socks5UDPAssociateCmd:
		h.handleUDP(ctx, conn, metadata)
	default:
		_ = conn.Close()
	}
}

func (h *Inbound) handleUDP(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) {
	packetConn := &packetConn{
		PacketConn:  mierucommon.NewPacketOverStreamTunnel(conn),
		destination: metadata.Destination,
	}
	h.router.RoutePacketConnectionEx(ctx, packetConn, metadata, nil)
}

type packetConn struct {
	net.PacketConn
	destination M.Socksaddr
}

var _ N.PacketConn = (*packetConn)(nil)

func (c *packetConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	n, _, err := c.PacketConn.ReadFrom(buffer.FreeBytes())
	if err != nil {
		return M.Socksaddr{}, err
	}
	buffer.Truncate(n)
	if buffer.Len() < 3 {
		return M.Socksaddr{}, io.ErrShortBuffer
	}
	buffer.Advance(3)
	var addr mierumodel.AddrSpec
	if err = addr.ReadFromSocks5(buffer); err != nil {
		return M.Socksaddr{}, err
	}
	if addr.FQDN != "" {
		return M.Socksaddr{Fqdn: addr.FQDN, Port: uint16(addr.Port)}, nil
	}
	ip, _ := netip.AddrFromSlice(addr.IP)
	return M.Socksaddr{Addr: ip.Unmap(), Port: uint16(addr.Port)}, nil
}

func (c *packetConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	header := buf.NewSize(3 + M.MaxSocksaddrLength)
	defer header.Release()
	common.Must(header.WriteZeroN(3))
	addr := mierumodel.AddrSpec{Port: int(destination.Port)}
	if destination.IsFqdn() {
		addr.FQDN = destination.Fqdn
	} else {
		addr.IP = destination.Addr.AsSlice()
	}
	if err := addr.WriteToSocks5(header); err != nil {
		return err
	}
	packet := buf.NewSize(header.Len() + buffer.Len())
	defer packet.Release()
	common.Must1(packet.Write(header.Bytes()))
	common.Must1(packet.Write(buffer.Bytes()))
	_, err := c.PacketConn.WriteTo(packet.Bytes(), nil)
	return err
}

func buildServerConfig(options Options) (*mieruserver.ServerConfig, error) {
	if err := validateOptions(options); err != nil {
		return nil, err
	}
	var protocol *mierupb.TransportProtocol
	if options.Transport == "TCP" {
		protocol = mierupb.TransportProtocol_TCP.Enum()
	} else {
		protocol = mierupb.TransportProtocol_UDP.Enum()
	}
	users := make([]*mierupb.User, 0, len(options.Users))
	for _, user := range options.Users {
		users = append(users, &mierupb.User{
			Name:     proto.String(user.Name),
			Password: proto.String(user.Password),
		})
	}
	trafficPattern, _ := mierutp.Decode(options.TrafficPattern)
	var advanced *mierupb.ServerAdvancedSettings
	if options.UserHintIsMandatory {
		advanced = &mierupb.ServerAdvancedSettings{UserHintIsMandatory: proto.Bool(true)}
	}
	return &mieruserver.ServerConfig{Config: &mierupb.ServerConfig{
		PortBindings: []*mierupb.PortBinding{{
			Port:     proto.Int32(int32(options.ListenPort)),
			Protocol: protocol,
		}},
		Users:            users,
		TrafficPattern:   trafficPattern,
		Mtu:              proto.Int32(options.MTU),
		AdvancedSettings: advanced,
	}}, nil
}

func validateOptions(options Options) error {
	if options.Transport != "TCP" && options.Transport != "UDP" {
		return E.New("transport must be TCP or UDP")
	}
	if options.ListenPort == 0 {
		return E.New("listen_port must be set")
	}
	if len(options.Users) == 0 {
		return E.New("users is empty")
	}
	if options.MTU != 0 && (options.MTU < 1280 || options.MTU > 1500) {
		return E.New("MTU must be between 1280 and 1500")
	}
	for _, user := range options.Users {
		if user.Name == "" || user.Password == "" {
			return E.New("mieru username and password must not be empty")
		}
	}
	if options.TrafficPattern != "" {
		trafficPattern, err := mierutp.Decode(options.TrafficPattern)
		if err != nil {
			return fmt.Errorf("failed to decode traffic pattern: %w", err)
		}
		if err = mierutp.Validate(trafficPattern); err != nil {
			return fmt.Errorf("invalid traffic pattern: %w", err)
		}
	}
	return nil
}
