package nat

import (
	"sort"
	"strings"
)

type portSorter struct {
	ports []Port
	by    func(i, j Port) bool
}

func (s *portSorter) Len() int {
	return len(s.ports)
}

func (s *portSorter) Swap(i, j int) {
	s.ports[i], s.ports[j] = s.ports[j], s.ports[i]
}

func (s *portSorter) Less(i, j int) bool {
	ip := s.ports[i]
	jp := s.ports[j]

	return s.by(ip, jp)
}

// Sort sorts a list of ports using the provided predicate
// This function should compare `i` and `j`, returning true if `i` is
// considered to be less than `j`
func Sort(ports []Port, predicate func(i, j Port) bool) {
	s := &portSorter{ports, predicate}
	sort.Sort(s)
}

type portMapEntry struct {
	port      Port
	binding   *PortBinding
	portInt   int
	portProto string
}

type portMapSorter []portMapEntry

func (s portMapSorter) Len() int      { return len(s) }
func (s portMapSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// Less sorts the port so that the order is:
// 1. port with larger specified bindings
// 2. larger port
// 3. port with tcp protocol
func (s portMapSorter) Less(i, j int) bool {
	pi, pj := s[i].portInt, s[j].portInt
	var hpi, hpj int
	if s[i].binding != nil {
		hpi = toInt(s[i].binding.HostPort)
	}
	if s[j].binding != nil {
		hpj = toInt(s[j].binding.HostPort)
	}
	return hpi > hpj || pi > pj || (pi == pj && strings.EqualFold(s[i].portProto, "tcp"))
}

// SortPortMap sorts the list of ports and their respected mapping. The ports
// will explicit HostPort will be placed first.
func SortPortMap(ports []Port, bindings map[Port][]PortBinding) {
	s := portMapSorter{}
	for _, p := range ports {
		portInt, portProto := p.Int(), p.Proto()
		if binding, ok := bindings[p]; ok && len(binding) > 0 {
			for _, b := range binding {
				s = append(s, portMapEntry{
					port: p, binding: &b,
					portInt: portInt, portProto: portProto,
				})
			}
			bindings[p] = []PortBinding{}
		} else {
			s = append(s, portMapEntry{
				port:    p,
				portInt: portInt, portProto: portProto,
			})
		}
	}

	sort.Sort(s)
	var (
		i  int
		pm = make(map[Port]struct{})
	)
	// reorder ports
	for _, entry := range s {
		if _, ok := pm[entry.port]; !ok {
			ports[i] = entry.port
			pm[entry.port] = struct{}{}
			i++
		}
		// reorder bindings for this port
		if entry.binding != nil {
			bindings[entry.port] = append(bindings[entry.port], *entry.binding)
		}
	}
}

func toInt(s string) int {
	i, _, _ := parsePortRange(s)
	return i
}
