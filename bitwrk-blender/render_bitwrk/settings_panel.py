# ##### BEGIN GPL LICENSE BLOCK #####
#
#  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
#  Copyright (C) 2013-2016  Jonas Eschenburg <jonas@bitwrk.net>
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
#  along with this program.  If not, see <http://www.gnu.org/licenses/>.
#
# ##### END GPL LICENSE BLOCK #####

import bpy, time, http, re, webbrowser
from bpy.props import StringProperty, IntProperty, PointerProperty, EnumProperty, FloatProperty
from render_bitwrk.common import get_article_id, max_tilesize, render_resolution
from render_bitwrk.tiling import optimal_tiling

# Functions for probing host:port settings for a running BitWrk client
LAST_PROBE_TIME = time.time()
LAST_PROBE_RESULT = False
LAST_PROBE_SETTINGS = None
def probe_bitwrk_client(settings):
    global LAST_PROBE_TIME, LAST_PROBE_RESULT, LAST_PROBE_SETTINGS
    if settings_string(settings) == LAST_PROBE_SETTINGS and time.time() - LAST_PROBE_TIME < 5.0:
        return LAST_PROBE_RESULT
        
    LAST_PROBE_RESULT=do_probe_bitwrk_client(settings)
    LAST_PROBE_TIME=time.time()
    LAST_PROBE_SETTINGS=settings_string(settings)
    return LAST_PROBE_RESULT
    
def settings_string(settings):
    return "{}:{}".format(settings.bitwrk_client_host, settings.bitwrk_client_port)
    
def do_probe_bitwrk_client(settings):
    conn = http.client.HTTPConnection(
        host=settings.bitwrk_client_host, port=settings.bitwrk_client_port,
        timeout=1)
    try:
        conn.request('GET', "/id")
        resp = conn.getresponse()
        if resp.status != http.client.OK:
            return False
        data = resp.read(256)
        if data != b"BitWrk Go Client":
            return False
        conn.request('GET', "/version")
        resp = conn.getresponse()
        if resp.status != http.client.OK:
            return False
        data = resp.read(256)
        if not re.match(b"[0-9]+\\.[0-9]+\\.[0-9]+", data):
            return False
        return True
    except:
        try:
            conn.close()
        except:
            pass
        return False

class StartBrowserOperator(bpy.types.Operator):
    """Open BitWrk admin console in web browser"""
    bl_idname = "bitwrk.startbrowser"
    bl_label = "Open BitWrk Client User Interface"

    @classmethod
    def poll(cls, context):
        return probe_bitwrk_client(context.scene.bitwrk_settings)
    
    def execute(self, context):
        settings=context.scene.bitwrk_settings
        webbrowser.open("http://{}:{}/".format(settings.bitwrk_client_host, settings.bitwrk_client_port)) 
        return {'FINISHED'}

    def invoke(self, context, event):
        return self.execute(context)


class RENDER_PT_bitwrk_settings(bpy.types.Panel):
    bl_label = "BitWrk distributed rendering"
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
        if probe_bitwrk_client(settings):
            self.layout.operator("bitwrk.startbrowser", icon='URL')
        else:
            self.layout.label("No BitWrk client at this address", icon='ERROR')
        
        self.layout.prop(settings, "complexity")
        row = self.layout.split(0.333)
        row.label("Article id: ", icon="RNDCURVE")
        row.label(get_article_id(settings.complexity))
        
        resx, resy = render_resolution(context.scene)
        max_pixels = max_tilesize(context.scene)
        u,v = optimal_tiling(resx, resy, max_pixels)
        row = self.layout.split(0.333)
        row.label("Tiles per frame", icon='MESH_GRID')
        row.label("{}   ({}x{}, efficiency: {:.1%})".format(u*v, u, v, resx*resy/u/v/max_pixels))
        
        self.layout.prop(settings, "concurrency")
        self.layout.prop(settings, "boost_factor")
        if settings.boost_factor > 1:
            self.layout.label("A boost factor greater than 1.0 makes rendering more expensive!", icon='ERROR')
