package transport

import "testing"

func TestSameHostDefaults(t *testing.T) {
	t.Setenv("TRANSPORT_PROFILE", ProfileSameHost)
	t.Setenv("BIND_IP", "")
	for _, name := range []string{"NEXTENDO_HOST", "NNCS1_BIND_IP", "NNCS2_BIND_IP", "NNCS1_PUBLIC_IP", "NNCS2_PUBLIC_IP"} {
		t.Setenv(name, "")
	}
	c, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if c.NEXBindIP != "127.0.0.1" || c.NEXAdvertisedIP != "127.0.0.1" || c.NNCS2BindIP != "127.0.0.2" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestPublicProfileAllowsPrivateBindsAndPublicAdvertisements(t *testing.T) {
	c := Config{Profile: ProfilePublic, NEXBindIP: "0.0.0.0", NEXAdvertisedIP: "9.9.9.9", NNCS1BindIP: "10.0.0.2", NNCS2BindIP: "10.0.0.3", NNCS1PublicIP: "8.8.8.8", NNCS2PublicIP: "1.1.1.1", NATFile: "/data/nat_endpoints.txt"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestPublicProfileRejectsPrivateCGNATAndDocumentationAddresses(t *testing.T) {
	for _, address := range []string{"192.168.1.2", "100.109.209.90", "203.0.113.10"} {
		c := Config{Profile: ProfilePublic, NEXBindIP: "0.0.0.0", NEXAdvertisedIP: address, NNCS1BindIP: "10.0.0.2", NNCS2BindIP: "10.0.0.3", NNCS1PublicIP: "8.8.8.8", NNCS2PublicIP: "1.1.1.1", NATFile: "/data/nat_endpoints.txt"}
		if err := c.Validate(); err == nil {
			t.Fatalf("expected %s to be rejected", address)
		}
	}
}
