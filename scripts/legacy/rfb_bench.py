#!/usr/bin/env python3
"""Force one full-screen FramebufferUpdate on a TigerVNC unix socket and time
how long the complete first frame takes to arrive. That is the smoothness
number: it is dominated by the server encoder CPU, which is exactly what
concurrent users contend for."""
import socket, struct, sys, time

path = sys.argv[1]
try:
    s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    s.settimeout(15)
    s.connect(path)
except Exception as e:
    print("CONNECT_FAIL %s" % e)
    sys.exit(1)


def rx(n):
    b = b""
    while len(b) < n:
        c = s.recv(n - len(b))
        if not c:
            raise EOFError("short read")
        b += c
    return b


rx(12)
s.sendall(b"RFB 003.008\n")
ntypes = rx(1)[0]
types = rx(ntypes)
if 1 not in types:
    print("NO_NONE_AUTH %s" % list(types))
    sys.exit(1)
s.sendall(bytes([1]))
if struct.unpack(">I", rx(4))[0] != 0:
    print("AUTH_FAIL")
    sys.exit(1)
s.sendall(b"\x01")

head = rx(24)
w, h = struct.unpack(">HH", head[:4])
namelen = struct.unpack(">I", head[20:24])[0]
name = rx(namelen).decode("utf-8", "replace")

pf = struct.pack(">BBBB", 32, 24, 0, 1) + struct.pack(">HHH", 255, 255, 255)
pf += struct.pack(">BBB", 16, 8, 0) + b"\x00\x00\x00"
s.sendall(b"\x00" + b"\x00\x00\x00" + pf)

encs = struct.pack(">H", 2) + struct.pack(">i", 5) + struct.pack(">i", 0)
s.sendall(b"\x02\x00" + encs)

req = struct.pack(">BBHHHH", 3, 0, 0, 0, w, h)

s.settimeout(0.25)
try:
    while s.recv(65536):
        pass
except Exception:
    pass

t0 = time.time()
s.sendall(req)
total = 0
last = t0
s.settimeout(0.35)
while True:
    try:
        d = s.recv(65536)
        if not d:
            break
        total += len(d)
        last = time.time()
    except socket.timeout:
        break
    except Exception:
        break

print("%.0fms %dKB %dx%d %s" % ((last - t0) * 1000, total // 1024, w, h, name))
