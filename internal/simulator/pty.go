// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package simulator

import (
	"fmt"
	"os"

	"github.com/creack/pty"
)

// PtyPair represents a pseudo-terminal pair with master and slave sides.
type PtyPair struct {
	Master     *os.File
	Slave      *os.File
	MasterPath string
	SlavePath  string
}

// Close closes both master and slave file descriptors.
func (p *PtyPair) Close() error {
	var err error
	if p.Master != nil {
		if e := p.Master.Close(); e != nil && err == nil {
			err = e
		}
		p.Master = nil
	}
	if p.Slave != nil {
		if e := p.Slave.Close(); e != nil && err == nil {
			err = e
		}
		p.Slave = nil
	}
	return err
}

// CreatePtyPair creates a new pseudo-terminal pair natively.
// The master is used by the simulator to read/write, and the slave path
// is provided to the client for communication.
func CreatePtyPair() (*PtyPair, error) {
	// Open a new pty master/slave pair
	master, slave, err := pty.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open pty: %w", err)
	}

	// The slave.Name() gives us the device path
	slaveName := slave.Name()

	return &PtyPair{
		Master:     master,
		Slave:      slave,
		MasterPath: master.Name(),
		SlavePath:  slaveName,
	}, nil
}
