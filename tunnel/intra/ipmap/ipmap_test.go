package ipmap

import (
	"net"
	"testing"
)

func TestGetTwice(t *testing.T) {
	m := NewIPMap()
	a := m.Get("example")
	b := m.Get("example")
	if a != b {
		t.Error("Matched Get returned different objects")
	}
}

func TestGetInvalid(t *testing.T) {
	m := NewIPMap()
	s := m.Get("example")
	if !s.Empty() {
		t.Error("Invalid name should result in an empty set")
	}
	if len(s.GetAll()) != 0 {
		t.Error("Empty set should be empty")
	}
}

func TestGetDomain(t *testing.T) {
	m := NewIPMap()
	s := m.Get("www.google.com")
	if s.Empty() {
		t.Error("Google lookup failed")
	}
	ips := s.GetAll()
	if len(ips) == 0 {
		t.Fatal("IP set is empty")
	}
	if ips[0] == nil {
		t.Error("nil IP in set")
	}
}

func TestGetIP(t *testing.T) {
	m := NewIPMap()
	s := m.Get("192.0.2.1")
	if s.Empty() {
		t.Error("IP parsing failed")
	}
	ips := s.GetAll()
	if len(ips) != 1 {
		t.Errorf("Wrong IP set size %d", len(ips))
	}
	if ips[0].String() != "192.0.2.1" {
		t.Error("Wrong IP")
	}
}

func TestAddDomain(t *testing.T) {
	m := NewIPMap()
	s := m.Get("example")
	s.Add("www.google.com")
	if s.Empty() {
		t.Error("Google lookup failed")
	}
	ips := s.GetAll()
	if len(ips) == 0 {
		t.Fatal("IP set is empty")
	}
	if ips[0] == nil {
		t.Error("nil IP in set")
	}
}
func TestAddIP(t *testing.T) {
	m := NewIPMap()
	s := m.Get("example")
	s.Add("192.0.2.1")
	ips := s.GetAll()
	if len(ips) != 1 {
		t.Errorf("Wrong IP set size %d", len(ips))
	}
	if ips[0].String() != "192.0.2.1" {
		t.Error("Wrong IP")
	}
}

func TestConfirmed(t *testing.T) {
	m := NewIPMap()
	s := m.Get("www.google.com")
	if s.Confirmed() != nil {
		t.Error("Confirmed should start out nil")
	}

	ips := s.GetAll()
	s.Confirm(ips[0].String())
	if !ips[0].Equal(s.Confirmed()) {
		t.Error("Confirmation failed")
	}

	s.Disconfirm(ips[0])
	if s.Confirmed() != nil {
		t.Error("Confirmed should now be nil")
	}
}

func TestConfirmNew(t *testing.T) {
	m := NewIPMap()
	s := m.Get("example")
	s.Add("192.0.2.1")
	// Confirm a new address.
	s.Confirm("192.0.2.2")
	if s.Confirmed() == nil || s.Confirmed().String() != "192.0.2.2" {
		t.Error("Confirmation failed")
	}
	ips := s.GetAll()
	if len(ips) != 2 {
		t.Error("New address not added to the set")
	}
}

func TestDisconfirmMismatch(t *testing.T) {
	m := NewIPMap()
	s := m.Get("www.google.com")
	ips := s.GetAll()
	s.Confirm(ips[0].String())

	// Make a copy
	otherIP := net.ParseIP(ips[0].String())
	// Alter it
	otherIP[0]++
	// Disconfirm.  This should have no effect because otherIP
	// is not the confirmed IP.
	s.Disconfirm(otherIP)

	if !ips[0].Equal(s.Confirmed()) {
		t.Error("Mismatched disconfirmation")
	}
}
