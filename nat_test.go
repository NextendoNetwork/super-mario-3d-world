package main

import (
	"testing"

	nex "github.com/NextendoNetwork/nextendo-nex"
)

func TestNoRelayAdvertised(t *testing.T) {
	s := nex.NewSwitchSettings(accessKey, nexVersion)
	c := nex.NewConnection(nex.NewEndpoint(s), "127.0.0.1:12345", func([]byte) {})
	req := nex.NewRMCRequest(s, nex.ProtocolNATTraversal, nex.MethodGetRelaySignatureKey, 1, nil)
	response := natHandler(false)(c, req)
	in := nex.NewStreamIn(response.Body, s)
	if in.U32() != 0 {
		t.Fatal("unexpected relay mode")
	}
	in.U64()
	if in.String() != "" || in.U16() != 0 {
		t.Fatal("an unimplemented relay was advertised")
	}
	if in.U32() != 0 || in.U32() != 0 || in.Err() != nil {
		t.Fatal("invalid relay response")
	}
}

func TestLocalProbeAcknowledgesLoopbackAndRejectsMalformedInput(t *testing.T) {
	s := nex.NewSwitchSettings(accessKey, nexVersion)
	c := nex.NewConnection(nex.NewEndpoint(s), "127.0.0.1:12345", func([]byte) {})
	body := nex.NewStreamOut(s)
	nex.WriteList(body, []string{}, func(o *nex.StreamOut, value string) { o.String(value) })
	body.String("prudp:/address=127.0.0.1;port=63054")
	req := nex.NewRMCRequest(s, nex.ProtocolNATTraversal, nex.MethodRequestProbeInitiationExt, 2, body.Bytes())
	if response := natHandler(true)(c, req); response == nil || response.IsError {
		t.Fatal("local probe was not acknowledged")
	}
	req.Body = []byte{1}
	if response := natHandler(true)(c, req); response == nil || !response.IsError {
		t.Fatal("malformed probe was accepted")
	}
}
