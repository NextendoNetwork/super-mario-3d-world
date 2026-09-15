package transport

import (
	"fmt"
	"net"
	"os"
	"strings"
)

const (
	ProfileSameHost = "same-host"
	ProfileLAN      = "lan"
	ProfilePublic   = "public-internet"
)

// Config separates socket bind addresses from the addresses advertised to
// clients. They match in a same-host test but commonly differ behind DNAT.
type Config struct {
	Profile         string
	NEXBindIP       string
	NEXAdvertisedIP string
	NNCS1BindIP     string
	NNCS2BindIP     string
	NNCS1PublicIP   string
	NNCS2PublicIP   string
	NATFile         string
}

func LoadFromEnv() (Config, error) {
	profile := value("TRANSPORT_PROFILE", ProfileSameHost)
	defaultBind := "0.0.0.0"
	if profile == ProfileSameHost {
		defaultBind = "127.0.0.1"
	}
	c := Config{
		Profile:         profile,
		NEXBindIP:       value("BIND_IP", defaultBind),
		NEXAdvertisedIP: os.Getenv("NEXTENDO_HOST"),
		NNCS1BindIP:     os.Getenv("NNCS1_BIND_IP"),
		NNCS2BindIP:     os.Getenv("NNCS2_BIND_IP"),
		NNCS1PublicIP:   os.Getenv("NNCS1_PUBLIC_IP"),
		NNCS2PublicIP:   os.Getenv("NNCS2_PUBLIC_IP"),
		NATFile:         strings.TrimSpace(os.Getenv("NNCS_NAT_FILE")),
	}
	if c.Profile == ProfileSameHost {
		c.NEXAdvertisedIP = fallback(c.NEXAdvertisedIP, "127.0.0.1")
		c.NNCS1BindIP = fallback(c.NNCS1BindIP, "127.0.0.1")
		c.NNCS2BindIP = fallback(c.NNCS2BindIP, "127.0.0.2")
		c.NNCS1PublicIP = fallback(c.NNCS1PublicIP, c.NNCS1BindIP)
		c.NNCS2PublicIP = fallback(c.NNCS2PublicIP, c.NNCS2BindIP)
	}
	return c, c.Validate()
}

func (c Config) SameHost() bool { return c.Profile == ProfileSameHost }

func (c Config) Validate() error {
	if c.Profile != ProfileSameHost && c.Profile != ProfileLAN && c.Profile != ProfilePublic {
		return fmt.Errorf("TRANSPORT_PROFILE must be same-host, lan, or public-internet")
	}
	if err := ipv4(c.NEXBindIP, "BIND_IP", true); err != nil {
		return err
	}
	for name, address := range map[string]string{
		"NEXTENDO_HOST": c.NEXAdvertisedIP, "NNCS1_BIND_IP": c.NNCS1BindIP,
		"NNCS2_BIND_IP": c.NNCS2BindIP, "NNCS1_PUBLIC_IP": c.NNCS1PublicIP,
		"NNCS2_PUBLIC_IP": c.NNCS2PublicIP,
	} {
		if err := ipv4(address, name, false); err != nil {
			return err
		}
	}
	if c.NNCS1BindIP == c.NNCS2BindIP {
		return fmt.Errorf("NNCS bind addresses must be distinct")
	}
	if c.NNCS1PublicIP == c.NNCS2PublicIP {
		return fmt.Errorf("NNCS advertised addresses must be distinct")
	}
	if c.Profile == ProfileSameHost {
		for _, address := range []string{c.NEXBindIP, c.NEXAdvertisedIP, c.NNCS1BindIP, c.NNCS2BindIP, c.NNCS1PublicIP, c.NNCS2PublicIP} {
			if !net.ParseIP(address).IsLoopback() {
				return fmt.Errorf("same-host addresses must be loopback addresses")
			}
		}
	}
	if c.Profile == ProfilePublic {
		for name, address := range map[string]string{"NEXTENDO_HOST": c.NEXAdvertisedIP, "NNCS1_PUBLIC_IP": c.NNCS1PublicIP, "NNCS2_PUBLIC_IP": c.NNCS2PublicIP} {
			if !globallyRoutable(net.ParseIP(address)) {
				return fmt.Errorf("%s must be a globally routable IPv4 address", name)
			}
		}
	}
	if c.Profile != ProfileSameHost && c.NATFile == "" {
		return fmt.Errorf("NNCS_NAT_FILE is required outside same-host mode")
	}
	return nil
}

func value(name, defaultValue string) string {
	if result := strings.TrimSpace(os.Getenv(name)); result != "" {
		return result
	}
	return defaultValue
}

func fallback(input, defaultValue string) string {
	if strings.TrimSpace(input) == "" {
		return defaultValue
	}
	return input
}

func ipv4(address, name string, allowUnspecified bool) error {
	ip := net.ParseIP(strings.TrimSpace(address))
	if ip == nil || ip.To4() == nil || ip.IsMulticast() || (!allowUnspecified && ip.IsUnspecified()) {
		return fmt.Errorf("%s must be a unicast IPv4 address", name)
	}
	return nil
}

func globallyRoutable(ip net.IP) bool {
	if ip == nil || ip.To4() == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, block := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4"} {
		_, network, _ := net.ParseCIDR(block)
		if network.Contains(ip) {
			return false
		}
	}
	return true
}
