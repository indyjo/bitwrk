#!/usr/bin/env python3

# ##### BEGIN GPL LICENSE BLOCK #####
#
#  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
#  Copyright (C) 2013-2018  Jonas Eschenburg <jonas@bitwrk.net>
#
#  This program is free software: you can redistribute it and/or modify
#  it under the terms of the GNU General Public License as published by
#  the Free Software Foundation, either version 3 of the License, or
#  (at your option) any later version.
#
#  This program is distributed in the hope that it will be useful,
#  but WITHOUT ANY WARRANTY; without even the implied warranty of
#  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#  GNU General Public License for more details.
#
#  You should have received a copy of the GNU General Public License
#  along with this program.  If not, see <http:#www.gnu.org/licenses/>.
#
# ##### END GPL LICENSE BLOCK #####

# Blender-slave.py - Offers Blender rendering to the BitWrk service

# Minimum Python version: 3.2 (tempfile.TemporaryDirectory, http.server)
import sys, signal
if sys.version_info[:2] < (3,2):
    raise RuntimeError("Python >= 3.2 required. Detected: %s" % sys.version_info)

import urllib.request, urllib.parse, urllib.error
import http.server, socket, struct, os, tempfile, subprocess
from select import select
from threading import Thread
        
# decode http chunked encoding
class Unchunked:
    def __init__(self, stream):
        self.stream = stream
        self.bytesLeft = 0
        self.bof = True
        self.eof = False
        
    def expect(self, pattern):
        data = self.stream.read(len(pattern))
        if data != pattern:
            raise ValueError("Data %s doesn't match expectation %s" % (repr(data), repr(pattern)))
            
    def readLength(self):
        length = 0
        while True:
            data = self.stream.read(1)
            if len(data) != 1:
                raise RuntimeError("Premature end of chunked data (parsing chunk length)")
            c = data[0]
            if (c < ord('0') or c > ord('9')) and (c < ord('a') or c > ord('f')) and c != ord('\r'):
                raise ValueError("Unexpected character")
            if c == ord('\r'):
                break
            elif c <= ord('9'):
                digit = c - ord('0')
            else:
                digit = 10 + (c - ord('a'))
            length = 16*length + digit
            if length > 0x8fffffff:
                raise RuntimeError("Length too big: %x" % length)
        self.expect(b'\n')
        return length
        
    def read(self, num):
        if num < 0:
            raise ValueError()
        result = bytearray()
        while num > 0:
            if self.eof:
                break
            elif self.bytesLeft == 0:
                if not self.bof:
                    self.expect(b'\r\n')
                self.bytesLeft = self.readLength()
                if self.bytesLeft == 0:
                    self.eof = True
                self.bof = False
            else:
                chunkNum = min(num, self.bytesLeft)
                data = self.stream.read(chunkNum)
                if len(data) != chunkNum:
                    raise RuntimeError("Premature end of chunked data")
                result.extend(data)
                self.bytesLeft -= chunkNum
                num -= chunkNum
        return bytes(result)
        
PYTHONSCRIPT = """
import bpy, sys

xmin={xmin}
ymin={ymin}
xmax={xmax}
ymax={ymax}
MAX_COST={maxcost}
device='{device}'

print("Blender sees:", xmin, ymin, xmax, ymax, MAX_COST, device)

scene = bpy.context.scene
render = scene.render
num_layers = 0
for layer in render.layers:
    if layer.use:
        num_layers += 1
if render.use_single_layer:
    num_layers = 1
render.image_settings.file_format='OPEN_EXR'
# Multi-layer rendering support in BitWrk was introduced with support for
# Blender 2.70, so only enable multilayer starting with that version.
if len(render.layers) > 1 and not render.use_single_layer and bpy.app.version >= (2,70,0):
    print("Multilayer enabled")
    render.image_settings.file_format='OPEN_EXR_MULTILAYER'
render.image_settings.color_mode='RGBA'
render.image_settings.exr_codec='PIZ'
render.image_settings.use_preview=False
render.use_compositing=False
render.use_sequencer=False
render.use_save_buffers=False
render.use_persistent_data=False

scene.cycles.use_cache=False
scene.cycles.debug_use_spatial_splits=False
scene.cycles.use_progressive_refine=False
scene.cycles.device=device

percentage = max(1, min(10000, render.resolution_percentage))
resx = int(render.resolution_x * percentage / 100)
resy = int(render.resolution_y * percentage / 100)

render.use_border = True
render.use_crop_to_border = True
render.tile_x = max(4, min(64, (xmax - xmin + 1) // 8))
render.tile_y = max(4, min(64, (ymax - ymin + 1) // 8))
print("Using tiles of size", render.tile_x, render.tile_y)
render.threads_mode='AUTO'

# Add 0.5 to integer coords to avoid errors caused by rounding towards zero
render.border_min_x = (xmin+0.5) / resx
render.border_max_x = (xmax+1.5) / resx
render.border_min_y = (ymin+0.5) / resy
render.border_max_y = (ymax+1.5) / resy

try:
    if xmax < xmin or ymax < ymin:
        raise RuntimeError("Illegal tile dimensions")
        
    f = lambda x: x*x if scene.cycles.use_square_samples else x
    if scene.cycles.progressive == 'PATH':
        cost_per_bounce = f(scene.cycles.samples)
    elif scene.cycles.progressive == 'BRANCHED_PATH':
        cost_per_bounce = f(scene.cycles.aa_samples) * (
            f(scene.cycles.diffuse_samples) +
            f(scene.cycles.glossy_samples) +
            f(scene.cycles.transmission_samples) +
            f(scene.cycles.ao_samples) +
            f(scene.cycles.mesh_light_samples) +
            f(scene.cycles.subsurface_samples))
    else:
        raise RuntimeError("Unknown sampling")

    cost_per_pixel = scene.cycles.max_bounces * cost_per_bounce
    cost_of_tile = num_layers * cost_per_pixel * (xmax - xmin + 1) * (ymax - ymin + 1)
    if cost_of_tile > MAX_COST:
        raise RuntimeError("Cost limit exceeded")
except:
    print(sys.exc_info())
    sys.exit(1)

"""

def checkRange(val, min, max):
    """Checks whether a value is between min and max"""
    if val < min or val > max:
        raise RuntimeError("Value %d not in range [%d, %d]", val, min, max)

class BlenderHandler(http.server.BaseHTTPRequestHandler):
    """HTTP handler that handles incoming work requests by dispatching them to Blender"""
    def do_POST(self):
        if self.path != "/work":
            self.send_error(404)
            return
        #print(self.headers)
        if self.headers["Transfer-Encoding"] == "chunked":
            stream = Unchunked(self.rfile)
        elif self.headers["Content-Length"] != "0":
            stream = self.rfile
        try:
            with tempfile.TemporaryDirectory() as tmpdir:
                self._work(stream, tmpdir)
        except:
            self.send_error(500)
            raise
        finally:
            reregister_with_bitwrk_client()
            
    def _work(self, rfile, tmpdir):
        xmin,ymin,xmax,ymax = 0,0,63,63
        frame=1
        seen_tags = {}
        resourceCount = 0
        done = False
        while True:
            tag = rfile.read(4)
            if len(tag) == 0:
                break
            if len(tag) != 4:
                raise RuntimeError("Premature EOF reading tag: %s" % tag)
            if type(tag) != bytes:
                raise RuntimeError("Illegal tag type: %s (%s)" % (tag, type(tag)))
            if done:
                raise RuntimeError("Done rendering but tag %s seen", tag)
            
            if tag not in [b'rsrc'] and tag in seen_tags:
                raise RuntimeError("Tag already seen: %s" % tag)
            seen_tags[tag] = tag
            
            lenBytes = self._read(rfile, 4)
            
            length = struct.unpack(">I", lenBytes)[0]
            if tag == b'xmin':
                xmin = self._readInt(rfile, tag, length)
            elif tag == b'xmax':
                xmax = self._readInt(rfile, tag, length)
            elif tag == b'ymin':
                ymin = self._readInt(rfile, tag, length)
            elif tag == b'ymax':
                ymax = self._readInt(rfile, tag, length)
            elif tag == b'fram':
                frame = self._readInt(rfile, tag, length)
            elif tag == b'rsrc':
                if resourceCount==1000:
                    raise RuntimeError("Too many resource files")
                resourceCount += 1
                self._readResource(rfile, length, tmpdir)
            elif tag == b'blen':
                self._callBlender(rfile, length, tmpdir, frame, xmin, xmax, ymin, ymax)
                done = True
            else:
                raise RuntimeError("Unknown tag: %s of length %d" % (tag, length))
        
    def _read(self, rfile, length):
        data = rfile.read(length)
        if len(data) != length:
            raise RuntimeError("Premature end of file: %d bytes expected, %d bytes received" % (length, len(data)))
        return data

        
    def _readInt(self, rfile, tag, length):
        if length != 4:
            raise RuntimeError("Illegal length %d for tag %s" % (length, tag))
        data = self._read(rfile, length)
        return struct.unpack(">i", data)[0]
    
    def _readResource(self, rfile, length, tmpdir):
        checkRange(length, 12, 0x8fffffff)
        
        aliasLength = struct.unpack('>I', self._read(rfile, 4))[0]
        checkRange(aliasLength, 1, 127)
        length -= 4 + aliasLength
        if length < 0:
            raise RuntimeError("Inconsistent lengths in resource chunk")
        
        alias = self._read(rfile, aliasLength)
        for c in alias:
            if ((c < ord('a') or c > ord('z')) and
                (c < ord('0') or c > ord('9'))):
                raise RuntimeError("Illegal characters in alias name")
        alias = alias.decode('ascii')
        
        origNameLength = struct.unpack('>I', self._read(rfile, 4))[0]
        checkRange(origNameLength, 1, 1023)
        length -= 4 + origNameLength
        if length < 0:
            raise RuntimeError("Inconsistent lengths in resource chunk")
        
        origName = self._read(rfile, origNameLength).decode('utf-8')
        
        print("Mapping", alias, "to", origName)
        
        resourceLength = struct.unpack('>I', self._read(rfile, 4))[0]
        checkRange(resourceLength, 0, 0x8fffffff)
        length -= 4 + resourceLength
        if length < 0:
            raise RuntimeError("Inconsistent lengths in resource chunk")
        
        with open(os.path.join(tmpdir, "rsrc." + alias + ".name"), "w") as f:
            f.write(origName)
        
        with open(os.path.join(tmpdir, "rsrc." + alias + ".data"), 'wb') as f:
            f.write(self._read(rfile, resourceLength))
            
        checkRange(length, 0, 0)
            
        
    def _callBlender(self, rfile, length, tmpdir, frame, xmin, xmax, ymin, ymax):
        blendfile = os.path.join(tmpdir, 'input.blend')
        pythonfile = os.path.join(tmpdir, 'setup.py')
        with open(pythonfile, 'w') as f:
            f.write(PYTHONSCRIPT.format(xmin=xmin, ymin=ymin, xmax=xmax, ymax=ymax, maxcost=MAX_COST, device=DEVICE))
        
        with open(blendfile, 'wb') as f:
            f.write(self._read(rfile, length))
        args = [BLENDER_BIN,
            '--background', blendfile,
            '-F', 'EXR',
            '--render-output', os.path.join(tmpdir, 'output#'),
            '-Y',
            '-noaudio',
            '-E', 'CYCLES',
            '-P', pythonfile,
            '--render-frame', '%d' % frame,
            ]
        print("Calling", args)
        #subprocess.check_call(args)
        with subprocess.Popen(args) as proc:
            while True:
                retcode = proc.poll()
                if retcode == 0:
                    break
                elif retcode is not None:
                    self.send_response(500)
                    return
                rl, _, _ = select([self.rfile], [], [], .1)
                if self.rfile in rl:
                    print("ERROR request cancelled")
                    proc.kill()
                    return
            
        #subprocess.check_call(['/bin/sleep','120'])
        
        self.send_response(200)
        with open(os.path.join(tmpdir, 'output%d.exr' % frame), 'rb') as f:
            f.seek(0, os.SEEK_END)
            self.send_header("Content-Length", "%d" % f.tell())
            self.end_headers()
            
            f.seek(0, os.SEEK_SET)
            data = f.read(32768)
            while len(data) > 0:
                self.wfile.write(data)
                data = f.read(32768)

def get_bitwrk_url():
    """Returns the defined BitWrk client URL"""
    if ":" in BITWRK_HOST:
        return "http://[%s]:%d" % (BITWRK_HOST, BITWRK_PORT)
    else:
        return "http://%s:%d" % (BITWRK_HOST, BITWRK_PORT)

def probe_bitwrk_client():
    """Tries to find out whether there is a BitWrk client at the defined URL"""
    bitwrkurl = get_bitwrk_url()
    try:
        id_resp = urllib.request.urlopen(bitwrkurl + "/id", None, 10)
        vrs_resp = urllib.request.urlopen(bitwrkurl + "/version", None, 10)
        print(" > Connected to '{}' version {} on {}"
              .format(id_resp.read(80).decode('ascii'),
                      vrs_resp.read(80).decode('ascii'),
                      bitwrkurl))
    except urllib.error.HTTPError as ex:
        print(" > Got a {} ({}) error when trying to probe BitWrk client on {}"
              .format(ex.code, ex.reason, bitwrkurl))
        if ex.code == 404:
            print("   This usually means that another application is listening on port ({}).".format(BITWRK_PORT))
        return False
    except urllib.error.URLError as ex:
        print(" > Couldn't connect to BitWrk client on", bitwrkurl)
        print("   Reason:", ex.reason)
        print("   This usually means that the BitWrk client is not running.")
        print("   It could also be listening on another port.")
        return False
    return True
    
def get_push_url():
    """Returns the URL that is used by the BitWrk client to push work to this worker."""
    if ":" in OWN_ADDRESS[0]:
        return 'http://[%s]:%d/work' % OWN_ADDRESS
    else:
        return 'http://%s:%d/work' % OWN_ADDRESS

def get_worker_id():
    """Returns the ID this worker is registered under at the BitWrk client."""
    return 'blender-%s-%d' % OWN_ADDRESS

def register_with_bitwrk_client():
    """Connects to the BitWrk client to advertise this worker"""
    query = urllib.parse.urlencode({
        'id' : get_worker_id(),
        'article' : ARTICLE_ID,
        'pushurl' : get_push_url()
    })
    
    bitwrkurl = get_bitwrk_url()
    try:
        urllib.request.urlopen(bitwrkurl + "/registerworker", query.encode('ascii'), 10)
    except urllib.error.HTTPError as ex:
        print(" > Got a {} ({}) error when trying to register on {}"
              .format(ex.code, ex.reason, bitwrkurl + "/registerworker"))
        if ex.code == 404:
            print("   This usually means that the BitWrk client does not accept workers.")
            print("   Please start it with the -extport argument!")
        return False
    return True

def detect_own_address():
    """Connects to the BitWrk client to find out which address this worker is reachable at"""
    bitwrkurl = get_bitwrk_url()
    try:
        myip = urllib.request.urlopen(bitwrkurl + "/myip", None, 10).read(200).decode('ascii')
        if myip.startswith("["):
            return myip[1:myip.index(']')]
        elif ":" in myip:
            return myip[0:myip.index(":")]
        else:
            return myip
    except urllib.error.HTTPError as ex:
        print(" > Got a {} ({}) error when trying to probe own network address on {}"
              .format(ex.code, ex.reason, bitwrkurl + "/myip"))
        if ex.code == 404:
            print("   This usually means that the BitWrk client is older than 0.6.2.")
        return None

def get_blender_version():
    """Starts the Blender executable to find out its version"""
    proc = subprocess.Popen([BLENDER_BIN, '-v'], stdout=subprocess.PIPE)
    output, _ = proc.communicate()
    if b"Blender 2.76 (sub 0)" in output:
        return "2.76"
    elif b"Blender 2.77 (sub 0)" in output:
        return "2.77"
    elif b"Blender 2.78 (sub 0)" in output:
        return "2.78"
    elif b"Blender 2.79 (sub" in output:
        return "2.79"
    else:
        raise RuntimeError("Blender version could not be detected.\n"
                           + "This version of " + __file__
                           + " will detect Blender versions "
                           + "2.76 up to 2.79.")

def parse_args():
    import argparse
    parser = argparse.ArgumentParser(description="Provides Blender rendering to the BitWrk service (http://bitwrk.net)")
    parser.add_argument('--blender', metavar='PATH', help="Blender executable to call", required=True)
    parser.add_argument('--bitwrk-host', metavar='HOST', help="BitWrk client host [localhost]", default="localhost")
    parser.add_argument('--bitwrk-port', metavar='PORT', help="BitWrk client port [8081]", type=int, default=8081)
    parser.add_argument('--max-cost', metavar='CLASS', help="Maximum cost of one task (in mega- and giga-rays) [512M]",
        choices=["512M", "2G", "8G", "32G"], default="512M")
    parser.add_argument('--listen-port', metavar='PORT',
        help="TCP port on which to listen for jobs (0=any) [0]", type=int, default=0)
    parser.add_argument('--listen-iface', metavar='IP',
        help="Network interface on which to listen for jobs [auto]", default="auto")
    parser.add_argument("--own-address", metavar='IP',
        help="Network address of this worker [auto]", default="auto")
    parser.add_argument('--device', metavar='DEVICE', help="Device to use for rendering [CPU]",
        choices=["CPU", "GPU"], default="CPU")
    return parser.parse_args()
        
if __name__ == "__main__":
    try:
        args = parse_args()
    except Exception as e:
        print(e)
        sys.exit(2)
    
    BLENDER_BIN=args.blender
    BLENDER_VERSION=get_blender_version()
    
    BITWRK_HOST=args.bitwrk_host
    BITWRK_PORT=args.bitwrk_port
    ARTICLE_ID="net.bitwrk/blender/0/{}/{}".format(BLENDER_VERSION, args.max_cost)
    if args.max_cost=='512M':
        MAX_COST=512*1024*1024
    elif args.max_cost=='2G':
        MAX_COST=2*1024*1024*1024
    elif args.max_cost=='8G':
        MAX_COST=8*1024*1024*1024
    elif args.max_cost=='32G':
        MAX_COST=32*1024*1024*1024
    else:
        raise RuntimeError()
    DEVICE=args.device
    
    print(" > Detected Blender", BLENDER_VERSION)
    print(" > Maximum number of rays is", MAX_COST)
    print(" > Article ID is", ARTICLE_ID)
    print(" > Rendering on", DEVICE)
    print()
    
    if not probe_bitwrk_client():
        sys.exit(3)
        
    # Detect this worker's public IP if required
    if args.own_address == "auto" or args.listen_iface == "auto":
        myip = detect_own_address()
        if myip is None:
            sys.exit(4)
            
    # Listen on network interface with that (or the configured) IP
    listen_iface = myip if args.listen_iface == "auto" else args.listen_iface
    addr_infos = socket.getaddrinfo(host=listen_iface, port=args.listen_port, type=socket.SOCK_STREAM)
    if addr_infos is None or len(addr_infos) == 0:
        print("Couldn't find out address family for address", listen_iface, "getsocketaddr returned", addr_infos)
    
    print(" > Listening on {}".format(addr_infos[0][4]))
    class Server(http.server.HTTPServer):
        address_family = addr_infos[0][0]
    httpd = Server(addr_infos[0][4], BlenderHandler)
    
    # Advertise auto-detected or manually given IP
    if args.own_address == "auto":
        OWN_ADDRESS = (myip, httpd.server_address[1])
    else:
        OWN_ADDRESS = httpd.server_address

    print(" > Registering worker URL", get_push_url())
    if not register_with_bitwrk_client():
        sys.exit(4)
    
    global reregister_with_bitwrk_client
    def reregister_with_bitwrk_client():
        return register_with_bitwrk_client()
    
    def handler(sig, stack):
        t = Thread(target=httpd.shutdown)
        t.start()
    signal.signal(signal.SIGINT, handler)
    signal.signal(signal.SIGTERM, handler)
    
    httpd.serve_forever()

    # Unregister on exit
    print(" > Shutdown. Unregistering from BitWrk client.")
    addr = httpd.server_address
    query = urllib.parse.urlencode({
        'id' : get_worker_id()
    })
    urllib.request.urlopen("http://%s:%d/unregisterworker" % (BITWRK_HOST, BITWRK_PORT), query.encode('ascii'), 10)
    print(" > Worker stopped")
