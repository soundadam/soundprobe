package network

import (
	"reflect"
	"testing"
)

func TestParseDarwinRoute(t *testing.T) {
	interfaceName, gateway := parseDarwinRoute(`
   route to: default
 destination: default
       gateway: 192.168.1.1
     interface: en0
`)
	if interfaceName != "en0" || gateway != "192.168.1.1" {
		t.Fatalf("route = %q/%q", interfaceName, gateway)
	}
}

func TestParseLinuxRoute(t *testing.T) {
	interfaceName, gateway := parseLinuxRoute("default via 10.0.0.1 dev eth0 proto dhcp src 10.0.0.5\n")
	if interfaceName != "eth0" || gateway != "10.0.0.1" {
		t.Fatalf("route = %q/%q", interfaceName, gateway)
	}
}

func TestParseWindowsRoute(t *testing.T) {
	interfaceAddress, gateway := parseWindowsRoute(`
IPv4 Route Table
===========================================================================
Active Routes:
Network Destination        Netmask          Gateway       Interface  Metric
          0.0.0.0          0.0.0.0     192.168.1.1     192.168.1.23     25
`)
	if interfaceAddress != "192.168.1.23" || gateway != "192.168.1.1" {
		t.Fatalf("route = %q/%q", interfaceAddress, gateway)
	}
}

func TestParseDNS(t *testing.T) {
	darwin := parseDarwinDNS(`
resolver #1
  nameserver[0] : 1.1.1.1
  nameserver[1] : 2606:4700:4700::1111
resolver #2
  nameserver[0] : 1.1.1.1
`)
	wantDarwin := []string{"1.1.1.1", "2606:4700:4700::1111"}
	if !reflect.DeepEqual(darwin, wantDarwin) {
		t.Fatalf("Darwin DNS = %#v", darwin)
	}

	linux := parseResolvConf("nameserver 8.8.8.8\nnameserver invalid\nnameserver 1.1.1.1\n")
	wantLinux := []string{"1.1.1.1", "8.8.8.8"}
	if !reflect.DeepEqual(linux, wantLinux) {
		t.Fatalf("resolv.conf DNS = %#v", linux)
	}

	windows := parseWindowsDNS(`DNS Servers . . . . . . . . . . . : 192.168.1.1
                                       1.1.1.1
`)
	if !reflect.DeepEqual(windows, []string{"1.1.1.1", "192.168.1.1"}) {
		t.Fatalf("Windows DNS = %#v", windows)
	}
}

func TestParseMacHardwarePort(t *testing.T) {
	output := `Hardware Port: Ethernet
Device: en5
Ethernet Address: 00:00:00:00:00:01

Hardware Port: Wi-Fi
Device: en0
Ethernet Address: 00:00:00:00:00:02
`
	if got := parseMacHardwarePort(output, "en0"); got != "Wi-Fi" {
		t.Fatalf("hardware port = %q", got)
	}
}

func TestClassifyInterface(t *testing.T) {
	tests := map[string]string{
		"utun4": "tunnel",
		"eth0":  "ethernet",
		"wlan0": "wifi",
		"en0":   "other",
		"":      "",
	}
	for input, want := range tests {
		if got := classifyInterface(input); got != want {
			t.Fatalf("classifyInterface(%q) = %q, want %q", input, got, want)
		}
	}
}
