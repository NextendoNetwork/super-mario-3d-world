package main

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestMakeNNCSResponseSeparatesObservationAndAdvertisedIdentity(t *testing.T) {
	response := makeNNCSResponse(102, &net.UDPAddr{IP: net.ParseIP("203.0.113.9"), Port: 54321}, "198.51.100.7")
	if binary.BigEndian.Uint32(response[0:4]) != 102 || binary.BigEndian.Uint32(response[4:8]) != 54321 {
		t.Fatal("wrong response header")
	}
	if net.IP(response[8:12]).String() != "203.0.113.9" {
		t.Fatal("wrong observed address")
	}
	if net.IP(response[12:16]).String() != "198.51.100.7" {
		t.Fatal("wrong advertised identity")
	}
}

func TestReplySourceMatchesProbeType(t *testing.T) {
	regular, _ := listenNNCS("127.0.0.1", 0)
	defer regular.Close()
	same, _ := listenNNCS("127.0.0.1", 0)
	defer same.Close()
	other, _ := listenNNCS("127.0.0.2", 0)
	defer other.Close()
	client, _ := listenNNCS("127.0.0.1", 0)
	defer client.Close()
	go serveNNCS(regular, "127.0.0.1", "127.0.0.2", same, other, nil)
	for _, kind := range []uint32{1, 2, 3, 4, 5, 101, 102, 103} {
		request := make([]byte, 16)
		binary.BigEndian.PutUint32(request, kind)
		if _, err := client.WriteToUDP(request, regular.LocalAddr().(*net.UDPAddr)); err != nil {
			t.Fatal(err)
		}
		_ = client.SetReadDeadline(time.Now().Add(time.Second))
		response := make([]byte, 16)
		n, source, err := client.ReadFromUDP(response)
		if err != nil || n != 16 {
			t.Fatalf("type %d: size=%d error=%v", kind, n, err)
		}
		expected := regular
		expectedIdentity := "127.0.0.1"
		if kind == 2 {
			expected = other
			expectedIdentity = "127.0.0.2"
		} else if kind == 3 || kind == 102 {
			expected = same
		}
		if source.String() != expected.LocalAddr().String() {
			t.Fatalf("type %d used the wrong source", kind)
		}
		if net.IP(response[12:16]).String() != expectedIdentity {
			t.Fatalf("type %d advertised the wrong source identity", kind)
		}
	}
}
