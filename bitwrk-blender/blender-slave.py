#!/usr/bin/env python3

#  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
#  Copyright (C) 2013-2014  Jonas Eschenburg <jonas@bitwrk.net>
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

# Blender-slave.py - Offers Blender rendering to the BitWrk service

# Minimum Python version: 3.2 (tempfile.TemporaryDirectory, http.server)
import sys, os
if sys.version_info[:2] < (3,2):
    raise RuntimeError("Python >= 3.2 required. Detected: %s" % sys.version_info)

import http.server, urllib.request, urllib.parse, struct, os, tempfile, subprocess
from select import select

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

print("Blender sees:", xmin, ymin, xmax, ymax, MAX_COST)

scene = bpy.context.scene
render = scene.render
render.image_settings.file_format='OPEN_EXR'
render.image_settings.exr_codec='PIZ'
render.image_settings.use_preview=False

percentage = max(1, min(10000, render.resolution_percentage))
resx = int(render.resolution_x * percentage / 100)
resy = int(render.resolution_y * percentage / 100)

render.use_border = True
render.use_crop_to_border = True
render.tile_x = 32
render.tile_y = 32

render.border_min_x = xmin / float(resx)
render.border_max_x = (xmax+1) / float(resx)
render.border_min_y = ymin / float(resy)
render.border_max_y = (ymax+1) / float(resy)

try:
    if xmax < xmin or ymax < ymin:
        raise RuntimeError("Illegal tile dimensions")
    if scene.cycles.progressive != 'PATH':
        raise RuntimeError("Please use 'Path Tracing' sampling")

    cost_per_pixel = scene.cycles.max_bounces * scene.cycles.samples
    cost_of_tile = cost_per_pixel * (xmax - xmin + 1) * (ymax - ymin + 1)
    if cost_of_tile > MAX_COST:
        raise RuntimeError("Cost limit exceeded")
except:
    print(sys.exc_info)
    sys.exit(1)

"""

class BlenderHandler(http.server.BaseHTTPRequestHandler):
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
            self._work(stream)
        except:
            self.send_error(500)
            raise
        finally:
            register_with_bitwrk_client()
            
    def _work(self, rfile):
        xmin,ymin,xmax,ymax = 0,0,63,63
        frame=1
        seen_tags = {}
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
            
            if tag in seen_tags:
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
            elif tag == b'blen':
                self._callBlender(rfile, length, frame, xmin, xmax, ymin, ymax)
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
        
    def _callBlender(self, rfile, length, frame, xmin, xmax, ymin, ymax):
        with tempfile.TemporaryDirectory() as tmpdir:
            blendfile = os.path.join(tmpdir, 'input.blend')
            pythonfile = os.path.join(tmpdir, 'setup.py')
            with open(pythonfile, 'w') as f:
                f.write(PYTHONSCRIPT.format(xmin=xmin, ymin=ymin, xmax=xmax, ymax=ymax, maxcost=MAX_COST))
            
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

def register_with_bitwrk_client():
    query = urllib.parse.urlencode({
        'id' : 'blender-%d' % addr[1],
        'article' : ARTICLE_ID,
        'pushurl' : 'http://%s:%d/work' % addr
    })
    urllib.request.urlopen("http://%s:%d/registerworker" % (BITWRK_HOST, BITWRK_PORT), query.encode('ascii'), 10)

def serve():
    httpd = http.server.HTTPServer(('127.0.0.1', 0), BlenderHandler)
    
    # Advertise worker to bitwrk
    global addr
    addr = httpd.server_address
    print("Serving on", addr)
    register_with_bitwrk_client()
    try:
        httpd.serve_forever()
    finally:
        # Unregister on exit
        query = urllib.parse.urlencode({
            'id' : 'blender-%d' % addr[1]
        })
        urllib.request.urlopen("http://%s:%d/unregisterworker" % (BITWRK_HOST, BITWRK_PORT), query.encode('ascii'), 10)
    
def get_blender_version():
    proc = subprocess.Popen([BLENDER_BIN, '-v'], stdout=subprocess.PIPE)
    output, _ = proc.communicate()
    if b"Blender 2.69" in output:
        return "2.69"
    else:
        raise RuntimeError("Blender version could not be detected")


def parse_args():
    global BLENDER_BIN, BLENDER_VERSION, BITWRK_HOST, BITWRK_PORT, ARTICLE_ID, MAX_COST
    
    import argparse
    parser = argparse.ArgumentParser(description="Provides Blender rendering to the BitWrk service (http://bitwrk.net)")
    parser.add_argument('--blender', metavar='PATH', help="Blender executable to call", required=True)
    parser.add_argument('--bitwrk-host', metavar='HOST', help="BitWrk client host", default="localhost")
    parser.add_argument('--bitwrk-port', metavar='PORT', help="BitWrk client port", type=int, default=8081)
    parser.add_argument('--max-cost', metavar='CLASS', help="Maximum cost of one task (in giga-rays)",
        choices=["512M", "2G", "8G", "32G"], default="8G")
    args = parser.parse_args()
    
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
        
if __name__ == "__main__":
    try:
        parse_args()
    except Exception as e:
        print(e)
        sys.exit(2)
    
    print("Detected Blender", BLENDER_VERSION)
    print("Serving to BitWrk client on {}:{}".format(BITWRK_HOST, BITWRK_PORT))
    print("Maximum number of rays is", MAX_COST)
    print("Article ID is", ARTICLE_ID)
    print("------------------------------------------------")
    serve()
