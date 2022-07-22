package utils

import (
	"net"
	"strings"
)

func GetMacAddress() ([]string, error) {
	ifas, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var as []string
	for _, ifa := range ifas {
		a := ifa.HardwareAddr.String()
		if a != "" {
			as = append(as, a)
		}
	}
	return as, nil
}

func GetMacAddressStr() string {
	addrs, err := GetMacAddress()
	if err != nil {
		return ""
	}

	return strings.Join(addrs, "|")
}
