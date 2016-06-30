// Copyright (c) 2014 The gomqtt Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package broker

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gomqtt/client"
	"github.com/gomqtt/packet"
	"github.com/gomqtt/spec"
	"github.com/gomqtt/tools"
	"github.com/gomqtt/transport"
	"github.com/stretchr/testify/assert"
)

func TestBroker(t *testing.T) {
	backend := NewMemoryBackend()
	backend.Logins = map[string]string{
		"allow": "allow",
	}

	port, done := runBroker(NewWithBackend(backend), 999)

	spec.Run(t, spec.FullMatrix, "localhost:"+port.Port())

	close(done)
}

func TestConnectTimeout(t *testing.T) {
	broker := New()
	broker.ConnectTimeout = 10 * time.Millisecond

	port, done := runBroker(broker, 1)

	conn, err := transport.Dial(port.URL())
	assert.NoError(t, err)

	pkt, err := conn.Receive()
	assert.Nil(t, pkt)
	assert.Error(t, err)

	close(done)
}

func TestKeepAlive(t *testing.T) {
	t.Parallel()

	port, done := runBroker(New(), 1)

	opts := client.NewOptions()
	opts.KeepAlive = "1s"

	client := client.New()

	var reqCounter int32
	var respCounter int32

	client.Logger = func(message string) {
		if strings.Contains(message, "Pingreq") {
			atomic.AddInt32(&reqCounter, 1)
		} else if strings.Contains(message, "Pingresp") {
			atomic.AddInt32(&respCounter, 1)
		}
	}

	connectFuture, err := client.Connect(port.URL(), opts)
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode)
	assert.False(t, connectFuture.SessionPresent)

	time.Sleep(2500 * time.Millisecond)

	err = client.Disconnect()
	assert.NoError(t, err)

	assert.Equal(t, int32(2), atomic.LoadInt32(&reqCounter))
	assert.Equal(t, int32(2), atomic.LoadInt32(&respCounter))

	close(done)
}

func TestKeepAliveTimeout(t *testing.T) {
	t.Parallel()

	connect := packet.NewConnectPacket()
	connect.KeepAlive = 1

	connack := packet.NewConnackPacket()

	client := tools.NewFlow().
		Send(connect).
		Receive(connack).
		End()

	port, done := runBroker(New(), 1)

	conn, err := transport.Dial(port.URL())
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	client.Test(t, conn)

	close(done)
}

func runBroker(broker *Broker, num int) (*tools.Port, chan struct{}) {
	port := tools.NewPort()

	server, _ := transport.Launch(port.URL())

	done := make(chan struct{})

	go func() {
		for i := 0; i < num; i++ {
			conn, _ := server.Accept()

			if conn != nil {
				broker.Handle(conn)
			}
		}
	}()

	go func() {
		<-done
		server.Close()
	}()

	return port, done
}
