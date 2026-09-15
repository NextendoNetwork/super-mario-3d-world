package main

import (
	"net"

	nex "github.com/NextendoNetwork/nextendo-nex"
)

func natHandler(sameHost bool) nex.RMCHandler {
	next := nex.NATTraversalHandler()
	return func(c *nex.Connection, request *nex.RMCMessage) *nex.RMCMessage {
		if sameHost && request.Method == nex.MethodRequestProbeInitiationExt {
			in := nex.NewStreamIn(request.Body, c.Settings)
			_ = nex.ReadList(in, func(s *nex.StreamIn) string { return s.String() })
			station := nex.ParseStationURL(in.String())
			if in.Err() != nil {
				return nex.NewRMCError(c.Settings, request.Protocol, request.CallID, nex.ResultCoreInvalidArgument)
			}
			ip := net.ParseIP(station.Get("address"))
			// The LAN candidate reaches the peer; the loopback candidate is identity only.
			if ip != nil && ip.IsLoopback() {
				return nex.NewRMCSuccess(c.Settings, request.Protocol, request.Method, request.CallID, nil)
			}
		}
		return next(c, request)
	}
}
