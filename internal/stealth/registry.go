package stealth

import (
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/protocol/direct"
	"github.com/sagernet/sing-box/protocol/hysteria2"
	"github.com/sagernet/sing-box/protocol/vless"
)

// Minimal sing-box protocol registries. We deliberately avoid sing-box's
// include.*Registry() helpers because they pull in every protocol (tor,
// shadowsocks, shadowtls, naive, v2ray transports, TUN/gvisor …). The node only
// needs two carrier inbounds and a direct outbound, so registering just those
// keeps the dependency surface and binary size small.

func inboundRegistry() *inbound.Registry {
	r := inbound.NewRegistry()
	vless.RegisterInbound(r)
	hysteria2.RegisterInbound(r)
	return r
}

func outboundRegistry() *outbound.Registry {
	r := outbound.NewRegistry()
	direct.RegisterOutbound(r)
	return r
}

func endpointRegistry() *endpoint.Registry {
	return endpoint.NewRegistry()
}
