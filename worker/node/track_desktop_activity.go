package node

import "encoding/binary"

// rfbActivity follows client message boundaries, not WebSocket frame boundaries.
// It only observes the None-authentication RFB stream used by our Unix-socket
// VNC server. Framebuffer requests/handshakes never renew the idle lease. Payloads
// (including large clipboards) are skipped without buffering. Unknown extensions
// disable observation rather than changing or blocking the forwarded stream.
type rfbActivity struct {
	phase    int
	header   [20]byte
	used     int
	skip     uint64
	disabled bool
}

func (a *rfbActivity) Observe(data []byte) (active bool) {
	for len(data) > 0 && !a.disabled {
		if a.skip > 0 {
			n := min(uint64(len(data)), a.skip)
			data = data[n:]
			a.skip -= n
			continue
		}
		switch a.phase {
		case 0:
			n := copy(a.header[a.used:12], data)
			a.used += n
			data = data[n:]
			if a.used < 12 {
				continue
			}
			version := string(a.header[:12])
			a.used = 0
			switch version {
			case "RFB 003.003\n":
				a.phase = 2
			case "RFB 003.007\n", "RFB 003.008\n":
				a.phase = 1
			default:
				a.disabled = true
			}
			continue
		case 1:
			if data[0] != 1 {
				a.disabled = true
			}
			data = data[1:]
			a.phase = 2
			continue
		case 2:
			data = data[1:] // ClientInit (shared flag)
			a.phase = 3
			continue
		}
		if a.used == 0 {
			a.header[0] = data[0]
			a.used = 1
			data = data[1:]
		}
		size := 0
		switch a.header[0] {
		case 0:
			size = 20 // SetPixelFormat
		case 2:
			size = 4 // SetEncodings + variable payload
		case 3:
			size = 10 // FramebufferUpdateRequest
		case 4:
			size = 8 // KeyEvent
		case 5:
			size = 6 // PointerEvent
		case 6:
			size = 8 // ClientCutText + variable payload
		case 150:
			size = 10 // EnableContinuousUpdates
		case 248:
			size = 9 // Fence + variable payload
		case 250:
			size = 4 // XVP
		case 251:
			size = 8 // SetDesktopSize + screen records
		case 255:
			size = 12 // QEMU ExtendedKeyEvent
		default:
			a.disabled = true
			continue
		}
		n := copy(a.header[a.used:size], data)
		a.used += n
		data = data[n:]
		if a.used < size {
			continue
		}
		switch a.header[0] {
		case 2:
			a.skip = uint64(binary.BigEndian.Uint16(a.header[2:4])) * 4
		case 6:
			length := int64(int32(binary.BigEndian.Uint32(a.header[4:8])))
			if length < 0 {
				length = -length
			} // extended clipboard
			a.skip = uint64(length)
		case 248:
			a.skip = uint64(a.header[8])
		case 251:
			a.skip = uint64(a.header[6]) * 16
		case 4, 5:
			active = true
		case 255:
			if a.header[1] == 0 {
				active = true
			} else {
				a.disabled = true
			}
		}
		a.used = 0
	}
	return active
}
