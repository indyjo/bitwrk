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

bl_info = {
    "name": "BitWrk Distributed Rendering",
    "description": "Support for distributed rendering using BitWrk, a marketplace for computing power",
    "author": "Jonas Eschenburg",
    "version": (0, 0, 1),
    "blender": (2, 69, 0),
    "category": "Render",
}

# Minimum Python version: 3.2 (tempfile.TemporaryDirectory)

import bpy, os, sys, http.client, select, struct, tempfile, urllib.request, colorsys, math
from bpy.props import StringProperty, IntProperty, PointerProperty, EnumProperty

# used by BitWrkSettings PropertyGroup
def set_complexity(self, value):
    self['complexity'] = value
    self['article_id'] = "net.bitwrk/blender/0/2.69/{}".format(['2G','8G','32G'][value])
    
def get_max_cost(settings):
    if settings.complexity == '2G':
        return  2*1024*1024*1024
    elif settings.complexity == '8G':
        return  8*1024*1024*1024
    elif settings.complexity == '32G':
        return  32*1024*1024*1024
    else:
        print(dir(settings), settings)
        print(settings.complexity)
        raise RuntimeError()


class BitWrkSettings(bpy.types.PropertyGroup):
    
    @classmethod
    def register(settings):
        settings.bitwrk_client_host = StringProperty(
            name="BitWrk client host",
            description="IP or name of host running local BitWrk client",
            maxlen=180,
            default="localhost")
        settings.bitwrk_client_port = IntProperty(
            name="BitWrk client port",
            description="TCP port the local BitWrk client listens on",
            default=8081,
            min=1,
            max=65535)
        settings.article_id = StringProperty(
            name="Article Id",
            description="Identifies Blender jobs on the BitWrk service",
            default="net.bitwrk/blender/0/2.69/8G")
        settings.complexity = EnumProperty(
            name="Complexity",
            description="Defines the maximum allowed computation complexity for each rendered tile",
            items=[
                ('2G',  " 2 Giga-rays", ""),
                ('8G',  " 8 Giga-rays", ""),
                ('32G', "32 Giga-rays", "")],
            default='8G',
            set=set_complexity,
            get=lambda value: value['complexity'])
        settings.concurrency = IntProperty(
            name="Concurrency",
            description="Maximum number of BitWrk trades active in parallel",
            default=4,
            min=1,
            max=256)
        
        bpy.types.Scene.bitwrk_settings = PointerProperty(type=BitWrkSettings, name="BitWrk Settings", description="Settings for using the BitWrk service")

    @classmethod
    def unregister(cls):
        del bpy.types.Scene.bitwrk_settings


class RENDER_PT_bitwrk_settings(bpy.types.Panel):
    bl_label = "BitWrk Settings Panel"
    bl_space_type = "PROPERTIES"
    bl_region_type = "WINDOW"
    bl_context = "render"
    COMPAT_ENGINES = {"BITWRK_RENDER"}
    
    @classmethod
    def poll(cls, context):
        rd = context.scene.render
        return rd.engine == 'BITWRK_RENDER' and not rd.use_game_engine
    
    def draw(self, context):
        settings=context.scene.bitwrk_settings
        self.layout.label("Local BitWrk client host and port:")
        row = self.layout.row()
        row.prop(settings, "bitwrk_client_host", text="")
        row.prop(settings, "bitwrk_client_port", text="")
        
        self.layout.prop(settings, "complexity")
        row = self.layout.row()
        row.prop(settings, "article_id")
        row.enabled = False
        
        self.layout.prop(settings, "concurrency")

class Chunked:
    """Wraps individual write()s into http chunked encoding."""
    def __init__(self, conn):
        self.conn = conn
    
    def write(self, data):
        if type(data) != bytes:
            data = data.encode('utf-8')
        if len(data) == 0:
            return
        self.conn.send(("%x" % len(data)).encode('ascii'))
        self.conn.send(b'\r\n')
        self.conn.send(data)
        self.conn.send(b'\r\n')
        
    def close(self):
        self.conn.send(b'0\r\n\r\n') # An empty chunk terminates the transmission
        
class Tagged:
    """Produces an IFF-like stream"""
    def __init__(self, out):
        self.out = out
        
    def writeFile(self, tag, f):
        """Writes the contents of a file into the stream"""
        if type(tag) != bytes:
            tag = tag.encode('utf-8')
        if len(tag) != 4:
            raise RuntimeError('Tag must be 4 byte long (was: %x)' % tag)
        f.seek(0, os.SEEK_END)
        length = f.tell()
        if length > 0x8fffffff:
            raise RuntimeError('File is too big to be written: %d bytes' % length)
        f.seek(0, os.SEEK_SET)
        
        self.out.write(struct.pack('>4sI', tag, length))
        while length > 0:
            data = f.read(min(length, 4096))
            length = length - len(data)
            self.out.write(data)
            
    def writeData(self, tag, value):
        if type(tag) != bytes:
            tag = tag.encode('utf-8')
        if type(value) != bytes:
            value = value.encode('utf-8')
        if len(tag) != 4:
            raise RuntimeError('Tag must be 4 byte long (was: %x)' % tag)
        if len(value) > 0x8fffffff:
            raise RuntimeError('Values too long: len(value)=%d' % len(value))
        self.out.write(struct.pack('>4sI', tag, len(value)))
        self.out.write(value)
        
    def writeInt(self, tag, value):
        self.writeData(tag, struct.pack(">i", value))
        
class Tile:
    def __init__(self, frame, minx, miny, resx, resy, color):
        self.conn = None
        self.result = None
        self.frame = frame
        self.minx = minx
        self.miny = miny
        self.resx = resx
        self.resy = resy
        self.color = color
        self.success = False
        
    def dispatch(self, settings, filename, engine):
        # draw rect in preview color
        tile = engine.begin_result(self.minx, self.miny, self.resx, self.resy)
        tile.layers[0].rect = [self.color] * (self.resx*self.resy)
        engine.end_result(tile)
        
        self.result = engine.begin_result(self.minx, self.miny, self.resx, self.resy)
        self.conn = http.client.HTTPConnection(
            settings.bitwrk_client_host, settings.bitwrk_client_port,
            strict=True, timeout=600)
        try:
            self.conn.putrequest("POST", "/buy/" + settings.article_id)
            self.conn.putheader('Transfer-Encoding', 'chunked')
            self.conn.endheaders()
            chunked = Chunked(self.conn)
            try:
                tagged = Tagged(chunked)
                tagged.writeInt('xmin', self.minx)
                tagged.writeInt('ymin', self.miny)
                tagged.writeInt('xmax', self.minx+self.resx-1)
                tagged.writeInt('ymax', self.miny+self.resy-1)
                tagged.writeInt('fram', self.frame)
                with open(filename, "rb") as file:
                    tagged.writeFile('blen', file)
            finally:
                chunked.close()
        except:
            print("Exception in dispatch:", sys.exc_info())
            engine.report({'ERROR'}, "Exception in dispatch: {}".format(sys.exc_info()))
            self.conn.close()
            self.conn = None
            self.result.layers[0].rect = [[1,0,0,1]] * (self.resx*self.resy)
            engine.end_result(self.result)
            self.result = None
            return False
        else:
            return True
        
    def collect(self, settings, engine):
        if self.conn is None:
            return
        try:
            resp = self.conn.getresponse()
            try:
                if resp.status == 303:
                    print("Fetching result from", resp.getheader("Location"))
                    location = resp.getheader("Location")
                    with tempfile.TemporaryDirectory() as tmpdir:
                        filename = os.path.join(tmpdir, "result.exr")
                        with open(filename, "wb") as tmpfile,\
                            urllib.request.urlopen("http://{}:{}{}".format(
                                settings.bitwrk_client_host,
                                settings.bitwrk_client_port,
                                location)) as response:
                            data = response.read(32768)
                            while len(data) > 0:
                                tmpfile.write(data)
                                data = response.read(32768)
                        self.result.layers[0].load_from_file(filename)
                        self.success = True
                else:
                    message = resp.read(1024).decode('ascii')
                    raise RuntimeError("Response status is {}, message was: {}".format(resp.status, message))
            finally:
                resp.close()
            engine.end_result(self.result)
        except:
            print("Exception in collect:", sys.exc_info())
            engine.report({'WARNING'}, "Exception in collect: {}".format(sys.exc_info()))
            self.result.layers[0].rect = [[1,0,0,1]] * (self.resx*self.resy)
            engine.end_result(self.result)
            self.result = None
        finally:
            self.conn.close()
            self.conn = None
            
    def fileno(self):
        return self.conn.sock.fileno()
        
    def cancel(self):
        try:
            if self.conn is not None:
                self.conn.close()
        except:
            pass
        finally:
            self.conn = None


class BitWrkRenderEngine(bpy.types.RenderEngine):
    """BitWrk Rendering Engine"""
    bl_idname = "BITWRK_RENDER"
    bl_label = "BitWrk distributed rendering"
    bl_description = "Performs distributed rendering using the BitWrk marketplace for compute power"
    
    def render(self, scene):
        try:
            self._doRender(scene)
        except:
            self.report({'ERROR'}, "Exception while rendering: {}".format(sys.exc_info()))
            
    def _doRender(self, scene):
        # Make sure the .blend file has been saved
        filename = bpy.data.filepath
        if scene.cycles.progressive != 'PATH':
            raise RuntimeError("Please use 'Path Tracing' sampling")
        if not os.path.exists(filename):
            raise RuntimeError("Current file path not defined\nSave your file before sending a job")
        bpy.ops.wm.save_mainfile(filepath=filename, check_existing=False)
        
        settings = scene.bitwrk_settings
        percentage = max(1, min(100, scene.render.resolution_percentage))
        resx = int(scene.render.resolution_x * percentage / 100)
        resy = int(scene.render.resolution_y * percentage / 100)
        cost_per_pixel = scene.cycles.max_bounces * scene.cycles.samples
        
        max_pixels_per_tile = int(math.floor(get_max_cost(settings) / cost_per_pixel))
        tiles = self._makeTiles(settings, scene.frame_current, 0, 0, resx, resy, max_pixels_per_tile)
        
        num_active = 0
        while not self.test_break():        
        
            remaining = [t for t in tiles if not t.success]
            if not remaining:
                self.report({'INFO'}, "Successfully rendered {} tiles on BitWrk.".format(len(tiles)))
                break
                
            # Dispatch some unfinished tiless
            for tile in remaining:
                if tile.conn is None and num_active < settings.concurrency:
                    if tile.dispatch(settings, filename, self):
                        num_active += 1
            
            # Poll from all tiles currently active
            active = filter(lambda tile: tile.conn is not None, tiles)
            rlist, wlist, xlist = select.select(active, [], active, 2.0)
            
            # Collect from all tiles where data has arrived
            for list in rlist, xlist:
                for tile in list:
                    if tile.conn is not None:
                        tile.collect(settings, self)
                        # collect has either failed or not. In any case, the tile is
                        # no longer active.
                        num_active -= 1
            
            # Report status
            successful = 0
            for tile in tiles:
                if tile.success:
                    successful += 1
            self.update_progress(successful / len(tiles))
        if self.test_break():
            for tile in filter(lambda tile: tile.conn is not None, tiles):
                tile.cancel()
    
    angle = 0.0
    @classmethod
    def _getcolor(cls):
        cls.angle += 0.61803399
        if cls.angle >= 1:
            cls.angle -= 1
        return colorsys.hsv_to_rgb(cls.angle, 0.5, 0.2)
        
        
    
    def _makeTiles(self, settings, frame, minx, miny, resx, resy, max_pixels):
        #print("make tiles:", minx, miny, resx, resy, max_pixels)
        pixels = resx*resy
        if pixels <= max_pixels:
            c = BitWrkRenderEngine._getcolor()
            tile = Tile(frame, minx, miny, resx, resy, [c[0], c[1], c[2], 1])
            return [tile]
        elif resx >= resy:
            left = resx // 2
            result = self._makeTiles(settings, frame, minx, miny, left, resy, max_pixels)
            result.extend(self._makeTiles(settings, frame, minx+left, miny, resx-left, resy, max_pixels))
            return result
        else:
            top = resy // 2
            result = self._makeTiles(settings, frame, minx, miny, resx, top, max_pixels)
            result.extend(self._makeTiles(settings, frame, minx, miny+top, resx, resy-top, max_pixels))
            return result

def register():
    print("Registered BitWrk renderer")
    bpy.utils.register_class(BitWrkRenderEngine)
    bpy.utils.register_class(RENDER_PT_bitwrk_settings)
    bpy.utils.register_class(BitWrkSettings)
    from bl_ui import properties_render as pr
    from bl_ui import properties_material as pm
    panels = [
        pr.RENDER_PT_antialiasing,
        pr.RENDER_PT_dimensions,
        pr.RENDER_PT_performance,
        pr.RENDER_PT_post_processing,
        pr.RENDER_PT_render,
        pr.RENDER_PT_shading,
        pr.RENDER_PT_stamp,
        pm.MATERIAL_PT_preview,
        ]
    for panel in panels:
        panel.COMPAT_ENGINES.add('BITWRK_RENDER')
    
def unregister():
    bpy.utils.unregister_class(BitWrkSettings)
    bpy.utils.unregister_class(RENDER_PT_bitwrk_settings)
    bpy.utils.unregister_class(BitWrkRenderEngine)


# This allows you to run the script directly from blenders text editor
# to test the addon without having to install it.
if __name__ == "__main__":
    register()