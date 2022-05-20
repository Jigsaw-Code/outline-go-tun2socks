package dns

import (
	"errors"
	"log"
	"sync"

	"golang.org/x/net/dns/dnsmessage"
)

type Rule interface {
	Match(name []byte) bool
}

type TransportWithRule interface {
	Transport
	SetRule(Rule)
}

type splitDNS struct {
	mu                     sync.RWMutex
	rule                   Rule
	specialDNS, defaultDNS Transport
}

func MakeSplitDNS(rule Rule, specialDNS, defaultDNS Transport) TransportWithRule {
	return &splitDNS{
		rule:       rule,
		specialDNS: specialDNS,
		defaultDNS: defaultDNS,
	}
}

func (d *splitDNS) Query(q []byte) ([]byte, error) {
	qname, err := getQName(q)
	if err != nil {
		log.Printf("Couldn't get qname")
		return nil, err
	}
	d.mu.RLock()
	rule := d.rule
	d.mu.RUnlock()
	log.Printf("Query for %s", string(qname))
	if rule != nil && rule.Match(qname) {
		log.Printf("Using special resolution for %s", string(qname))
		return d.specialDNS.Query(q)
	}
	return d.defaultDNS.Query(q)
}

func (d *splitDNS) GetURL() string {
	return "FIXME"
}

func (d *splitDNS) SetRule(rule Rule) {
	d.mu.Lock()
	d.rule = rule
	d.mu.Unlock()
}

func getQName(q []byte) ([]byte, error) {
	var msg dnsmessage.Message
	if err := msg.Unpack(q); err != nil {
		return nil, err
	}
	if len(msg.Questions) == 0 {
		return nil, errors.New("no question in query")
	}
	name := msg.Questions[0].Name
	return name.Data[:name.Length], nil
}
