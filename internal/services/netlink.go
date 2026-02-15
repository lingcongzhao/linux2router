package services

import (
	"fmt"
	"net"

	"linuxtorouter/internal/models"

	"github.com/vishvananda/netlink"
)

type NetlinkService struct{}

func NewNetlinkService() *NetlinkService {
	return &NetlinkService{}
}

func (s *NetlinkService) ListInterfaces() ([]models.NetworkInterface, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list links: %w", err)
	}

	var interfaces []models.NetworkInterface
	for _, link := range links {
		attrs := link.Attrs()

		iface := models.NetworkInterface{
			Index: attrs.Index,
			Name:  attrs.Name,
			MTU:   attrs.MTU,
			Type:  link.Type(),
		}

		if attrs.HardwareAddr != nil {
			iface.MAC = attrs.HardwareAddr.String()
		}

		if attrs.OperState == netlink.OperUp {
			iface.State = "UP"
		} else if attrs.OperState == netlink.OperDown {
			iface.State = "DOWN"
		} else {
			// Fall back to checking flags
			if attrs.Flags&net.FlagUp != 0 {
				iface.State = "UP"
			} else {
				iface.State = "DOWN"
			}
		}

		// Get flags
		if attrs.Flags&net.FlagUp != 0 {
			iface.Flags = append(iface.Flags, "UP")
		}
		if attrs.Flags&net.FlagBroadcast != 0 {
			iface.Flags = append(iface.Flags, "BROADCAST")
		}
		if attrs.Flags&net.FlagLoopback != 0 {
			iface.Flags = append(iface.Flags, "LOOPBACK")
		}
		if attrs.Flags&net.FlagPointToPoint != 0 {
			iface.Flags = append(iface.Flags, "POINTTOPOINT")
		}
		if attrs.Flags&net.FlagMulticast != 0 {
			iface.Flags = append(iface.Flags, "MULTICAST")
		}

		// Get addresses
		addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
		if err == nil {
			for _, addr := range addrs {
				if addr.IP.To4() != nil {
					iface.IPv4Addrs = append(iface.IPv4Addrs, addr.IPNet.String())
				} else {
					iface.IPv6Addrs = append(iface.IPv6Addrs, addr.IPNet.String())
				}
			}
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

func (s *NetlinkService) GetInterface(name string) (*models.NetworkInterface, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("interface not found: %w", err)
	}

	attrs := link.Attrs()

	iface := &models.NetworkInterface{
		Index: attrs.Index,
		Name:  attrs.Name,
		MTU:   attrs.MTU,
		Type:  link.Type(),
	}

	if attrs.HardwareAddr != nil {
		iface.MAC = attrs.HardwareAddr.String()
	}

	if attrs.OperState == netlink.OperUp {
		iface.State = "UP"
	} else if attrs.OperState == netlink.OperDown {
		iface.State = "DOWN"
	} else {
		if attrs.Flags&net.FlagUp != 0 {
			iface.State = "UP"
		} else {
			iface.State = "DOWN"
		}
	}

	// Get flags
	if attrs.Flags&net.FlagUp != 0 {
		iface.Flags = append(iface.Flags, "UP")
	}
	if attrs.Flags&net.FlagBroadcast != 0 {
		iface.Flags = append(iface.Flags, "BROADCAST")
	}
	if attrs.Flags&net.FlagLoopback != 0 {
		iface.Flags = append(iface.Flags, "LOOPBACK")
	}
	if attrs.Flags&net.FlagPointToPoint != 0 {
		iface.Flags = append(iface.Flags, "POINTTOPOINT")
	}
	if attrs.Flags&net.FlagMulticast != 0 {
		iface.Flags = append(iface.Flags, "MULTICAST")
	}

	// Get addresses
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err == nil {
		for _, addr := range addrs {
			if addr.IP.To4() != nil {
				iface.IPv4Addrs = append(iface.IPv4Addrs, addr.IPNet.String())
			} else {
				iface.IPv6Addrs = append(iface.IPv6Addrs, addr.IPNet.String())
			}
		}
	}

	return iface, nil
}

func (s *NetlinkService) SetInterfaceUp(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("interface not found: %w", err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	return nil
}

func (s *NetlinkService) SetInterfaceDown(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("interface not found: %w", err)
	}

	if err := netlink.LinkSetDown(link); err != nil {
		return fmt.Errorf("failed to bring interface down: %w", err)
	}

	return nil
}

func (s *NetlinkService) SetMTU(name string, mtu int) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("interface not found: %w", err)
	}

	if err := netlink.LinkSetMTU(link, mtu); err != nil {
		return fmt.Errorf("failed to set MTU: %w", err)
	}

	return nil
}

func (s *NetlinkService) AddAddress(name string, cidr string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("interface not found: %w", err)
	}

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("failed to add address: %w", err)
	}

	return nil
}

func (s *NetlinkService) RemoveAddress(name string, cidr string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("interface not found: %w", err)
	}

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	if err := netlink.AddrDel(link, addr); err != nil {
		return fmt.Errorf("failed to remove address: %w", err)
	}

	return nil
}

func (s *NetlinkService) GetStats(name string) (*models.InterfaceStats, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("interface not found: %w", err)
	}

	attrs := link.Attrs()
	if attrs.Statistics == nil {
		return &models.InterfaceStats{}, nil
	}

	stats := attrs.Statistics
	return &models.InterfaceStats{
		RxBytes:   stats.RxBytes,
		TxBytes:   stats.TxBytes,
		RxPackets: stats.RxPackets,
		TxPackets: stats.TxPackets,
		RxErrors:  stats.RxErrors,
		TxErrors:  stats.TxErrors,
		RxDropped: stats.RxDropped,
		TxDropped: stats.TxDropped,
	}, nil
}
