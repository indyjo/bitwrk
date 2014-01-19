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
    "version": (0, 0, 0),
    "blender": (2, 69, 0),
    "category": "Render",
}

# Minimum Python version: 3.2 (tempfile.TemporaryDirectory)

import bpy, os, http.client, struct, tempfile, urllib.request
from bpy.props import StringProperty, IntProperty, PointerProperty

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
            default="foobar")
        
        bpy.types.Scene.bitwrk_settings = PointerProperty(type=BitWrkSettings, name="BitWrk Settings", description="Settings for using the BitWrk service")

    @classmethod
    def unregister(cls):
        del bpy.types.Scene.network_render


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
        
        row = self.layout.row()
        row.prop(settings, "article_id")
        row.enabled = False

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
        

class BitWrkRenderEngine(bpy.types.RenderEngine):
    """BitWrk Rendering Engine"""
    bl_idname = "BITWRK_RENDER"
    bl_label = "BitWrk distributed rendering"
    bl_description = "Performs distributed rendering using the BitWrk marketplace for compute power"
    
    def render(self, scene):
        # Make sure the .blend file has been saved
        filename = bpy.data.filepath
        if scene.cycles.progressive != 'PATH':
            raise RuntimeError("Please use 'Path Tracing' sampling")
        if not os.path.exists(filename):
            raise RuntimeError("Current file path not defined\nSave your file before sending a job")
        bpy.ops.wm.save_mainfile(filepath=filename, check_existing=False)
        
        percentage = max(1, min(100, scene.render.resolution_percentage))
        resx = int(scene.render.resolution_x * percentage / 100)
        resy = int(scene.render.resolution_y * percentage / 100)
        result = self.begin_result(0,0,resx,resy)
        settings = scene.bitwrk_settings
        
        conn = http.client.HTTPConnection(settings.bitwrk_client_host, settings.bitwrk_client_port, strict=True, timeout=600)
        try:
            conn.putrequest("POST", "/buy/" + settings.article_id)
            conn.putheader('Transfer-Encoding', 'chunked')
            conn.endheaders()
            chunked = Chunked(conn)
            try:
                tagged = Tagged(chunked)
                tagged.writeInt('xmin', 0)
                tagged.writeInt('ymin', 0)
                tagged.writeInt('xmax', resx-1)
                tagged.writeInt('ymax', resy-1)
                tagged.writeInt('fram', scene.frame_current)
                with open(filename, "rb") as file:
                    tagged.writeFile('blen', file)
            finally:
                chunked.close()
            
            print("Request sent")
            resp = conn.getresponse()
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
                    result.layers[0].load_from_file(filename)
        finally:
            conn.close()
           
        self.end_result(result)
    

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