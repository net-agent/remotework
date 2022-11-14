package utils

import (
	"golang.org/x/sys/windows/registry"
)

const defaultRdpPortNumber = 3389

func GetRDPPort() uint16 {
	// print rdp port
	rkey, found, err := registry.CreateKey(registry.LOCAL_MACHINE,
		"SYSTEM\\CurrentControlSet\\Control\\Terminal Server\\WinStations\\RDP-Tcp",
		registry.READ)
	if err != nil {
		return defaultRdpPortNumber
	}
	if !found {
		return defaultRdpPortNumber
	}
	defer rkey.Close()
	kv, _, err := rkey.GetIntegerValue("PortNumber")
	if err != nil {
		return defaultRdpPortNumber
	}

	return uint16(kv)
}
