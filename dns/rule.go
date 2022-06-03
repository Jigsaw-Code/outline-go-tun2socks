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
	qname, _, err := getQuery(q)
	if err != nil {
		log.Printf("Couldn't get qname")
		return nil, err
	}
	d.mu.RLock()
	rule := d.rule
	d.mu.RUnlock()
	log.Printf("Query for %s", qname.String())
	if rule != nil && rule.Match(qname.Data[:qname.Length]) {
		log.Printf("Using special resolution for %s", qname.String())
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

func getQuery(q []byte) (qname dnsmessage.Name, qtype dnsmessage.Type, err error) {
	qtype = dnsmessage.Type(0xFFFF)
	var msg dnsmessage.Message
	if err = msg.Unpack(q); err != nil {
		return
	}
	if len(msg.Questions) == 0 {
		err = errors.New("no question in query")
		return
	}
	qname = msg.Questions[0].Name
	qtype = msg.Questions[0].Type
	return
}
