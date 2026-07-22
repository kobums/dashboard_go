package setting

import (
	"errors"
	"math"
	"net"
	"regexp"
	"strings"

	"dashboard/global"

	"github.com/c-robinson/iplib"
)

type IP struct {
	Address  string `json:"address"`
	Start    net.IP `json:"-"`
	End      net.IP `json:"-"`
	StartInt int64  `json:"-"`
	EndInt   int64  `json:"-"`
	Subnet   int    `json:"-"`
}

func NewIP(str string) (*IP, error) {
	r, _ := regexp.Compile(`^((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})((/([1-3]?[0-9]))|(-(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)))?$`)
	m := r.FindStringSubmatch(str)
	if len(m) == 0 {
		return nil, errors.New("[2001] wrong address")
	}

	ip := IP{}

	ip.Address = str
	addr := net.ParseIP(m[1])

	if m[7] != "" {
		subnet := global.Atoi(m[7])
		n := iplib.NewNet4(addr, subnet)
		ip.Start = n.FirstAddress()
		ip.End = n.LastAddress()
		ip.Subnet = subnet
	} else if m[9] != "" {
		iend := global.Atoi(m[9])
		istart := global.Atoi(m[3])
		ip.Subnet = 24 + int(math.Abs(float64(iend-istart)))/32

		ip.Start = addr
		n := strings.Split(str, ".")
		n[3] = m[9]
		ip.End = net.ParseIP(strings.Join(n, "."))

	} else {
		ip.Start = addr
		ip.End = addr
		ip.Subnet = 24
	}

	ip.StartInt = int64(iplib.IP4ToUint32(ip.Start))
	ip.EndInt = int64(iplib.IP4ToUint32(ip.End))

	return &ip, nil
}

func (c *IP) Match(dest *IP) bool {
	if c.StartInt <= dest.StartInt && c.EndInt >= dest.StartInt {
		return true
	}

	if dest.StartInt <= c.StartInt && dest.EndInt >= c.StartInt {
		return true
	}

	return false
}

func (c *IP) Contains(dest *IP) bool {
	if c.StartInt <= dest.StartInt && c.EndInt >= dest.StartInt {
		return true
	}

	return false
}

func (c *IP) Equal(dest *IP) bool {
	return c.Address == dest.Address
}

func MatchIP(src IP, dest IP) bool {
	if src.StartInt <= dest.StartInt && src.EndInt >= dest.StartInt {
		return true
	}

	if dest.StartInt <= src.StartInt && dest.EndInt >= src.StartInt {
		return true
	}

	return false
}
