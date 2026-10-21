package node

import "testing"

func clientHandshake() []byte { return append([]byte("RFB 003.008\n"), 1, 1) }

func TestDesktopActivityIgnoresProtocolTraffic(t *testing.T) {
	traffic := clientHandshake()
	traffic = append(traffic, make([]byte, 20)...)                                        // SetPixelFormat
	traffic = append(traffic, []byte{2, 0, 0, 1, 0, 0, 0, 4}...)                          // encoding resembling a key type
	traffic = append(traffic, []byte{3, 0, 0, 0, 0, 0, 0, 1, 0, 1}...)                    // framebuffer request
	traffic = append(traffic, []byte{6, 0, 0, 0, 0, 0, 0, 8, 4, 1, 0, 0, 0, 0, 0, 65}...) // clipboard containing a fake KeyEvent
	for split := 0; split <= len(traffic); split++ {
		a := &rfbActivity{}
		if a.Observe(traffic[:split]) || a.Observe(traffic[split:]) {
			t.Fatalf("protocol renewed lease at split %d", split)
		}
		if !a.Observe([]byte{4, 1, 0, 0, 0, 0, 0, 65}) {
			t.Fatalf("lost subsequent key at split %d", split)
		}
	}
}

func TestDesktopInputAcrossWebSocketFrames(t *testing.T) {
	for _, msg := range [][]byte{
		{4, 1, 0, 0, 0, 0, 0, 65},                // standard keyboard
		{5, 0, 0, 10, 0, 10},                     // mouse
		{255, 0, 0, 1, 0, 0, 0, 65, 0, 0, 0, 30}, // noVNC extended keyboard
	} {
		for split := 1; split < len(msg); split++ {
			a := &rfbActivity{}
			a.Observe(clientHandshake())
			if a.Observe(msg[:split]) {
				t.Fatal("partial input renewed lease")
			}
			if !a.Observe(msg[split:]) {
				t.Fatalf("fragmented input lost: %v at %d", msg, split)
			}
		}
	}
	a := &rfbActivity{}
	stream := append(clientHandshake(), []byte{5, 0, 0, 10, 0, 10}...)
	active := false
	for _, b := range stream {
		active = a.Observe([]byte{b}) || active
	}
	if !active {
		t.Fatal("bytewise stream lost mouse input")
	}
}
