# ##### BEGIN GPL LICENSE BLOCK #####
#
#  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
#  Copyright (C) 2013-2017  Jonas Eschenburg <jonas@bitwrk.net>
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

import bpy, colorsys, http.client, os, select, sys, tempfile, traceback, urllib.request
from render_bitwrk.chunked import Chunked
from render_bitwrk.tagged import Tagged
from render_bitwrk.common import get_article_id, max_tilesize, render_resolution
from render_bitwrk.tiling import optimal_tiling
from render_bitwrk.blendfile import save_copy, process_file
from render_bitwrk.bitwrkclient import probe_bitwrk_client

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
        
    def dispatch(self, settings, data, filepath, engine):
        """Dispatches one tile to the bitwrk client.
        The complete blender data is packed into the transmission.
        """ 
        # draw rect in preview color
        tile = engine.begin_result(self.minx, self.miny, self.resx, self.resy)
        tile.layers[0].passes[0].rect = [self.color] * (self.resx*self.resy)
        engine.end_result(tile)
        
        self.result = engine.begin_result(self.minx, self.miny, self.resx, self.resy)
        self.conn = http.client.HTTPConnection(
            settings.bitwrk_client_host, settings.bitwrk_client_port,
            timeout=600)
        try:
            self.conn.putrequest("POST", "/buy/" + get_article_id(settings.complexity, settings.trusted_render))
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
                tagged.bundleResources(engine, bpy.data)
                with open(filepath, "rb") as file:
                    tagged.writeFile('blen', file)
            finally:
                chunked.close()
        except:
            print("Exception in dispatch:", sys.exc_info())
            engine.report({'ERROR'}, "Exception in dispatch: {}".format(traceback.format_exc()))
            self.conn.close()
            self.conn = None
            self.result.layers[0].passes[0].rect = [[1,0,0,1]] * (self.resx*self.resy)
            engine.end_result(self.result)
            self.result = None
            return False
        else:
            return True
        
    def collect(self, settings, engine, is_multilayer):
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
                        if is_multilayer:
                            self.result.load_from_file(filename)
                        else:
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
            engine.report({'WARNING'}, "Exception in collect: {}".format(traceback.format_exc()))
            self.result.layers[0].passes[0].rect = [[1,0,0,1]] * (self.resx*self.resy)
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

_render_count = 0
def is_render_active():
    global _render_count
    return _render_count > 0

class BitWrkRenderEngine(bpy.types.RenderEngine):
    """BitWrk Rendering Engine"""
    bl_idname = "BITWRK_RENDER"
    bl_label = "BitWrk Render"
    bl_description = "Performs distributed rendering using the BitWrk marketplace for compute power"
    
    def render(self, scene):
        global _render_count
        _render_count += 1
        try:
            if not hasattr(scene, 'bitwrk_settings'):
                self.report({'ERROR'}, "Must first setup BitWrk")
                return
            if not probe_bitwrk_client(scene.bitwrk_settings):
                self.report({'ERROR'}, "Must first connect to BitWrk client")
                return
            with tempfile.TemporaryDirectory() as tmpdir:
                self._doRender(scene, tmpdir)
        except:
            self.report({'ERROR'}, "Exception while rendering: {}".format(traceback.format_exc()))
        finally:
            _render_count -= 1
        
    def _doRender(self, scene, tmpdir):
        # Export the file to a temporary directory and call this script
        # in a separate blender session for remapping all paths. This
        # seems to be the only way to change paths on the temp file only,
        # without affecting the original file.
        filename = os.path.join(tmpdir, "mainfile.blend")
        save_copy(filename)
        process_file(filename)
        self.report({'INFO'}, "mainfile.blend successfully exported: {}".format(filename))
        
        max_pixels_per_tile = max_tilesize(scene)
        is_multilayer = len(scene.render.layers) > 1 and not scene.render.use_single_layer
        resx, resy = render_resolution(scene)
        
        tiles = self._makeTiles(scene.frame_current, resx, resy, max_pixels_per_tile)
        # Sort by distance to center
        tiles.sort(key=lambda t: abs(t.minx + t.resx/2 - resx/2) + abs(t.miny + t.resy/2 - resy/2))
        
        settings = scene.bitwrk_settings
        num_active = 0
        while not self.test_break():        
        
            remaining = [t for t in tiles if not t.success]
            if not remaining:
                self.report({'INFO'}, "Successfully rendered {} tiles on BitWrk.".format(len(tiles)))
                break
                
            # Dispatch some unfinished tiles
            for tile in remaining:
                if tile.conn is None and num_active < settings.concurrency:
                    if tile.dispatch(settings, bpy.data, filename, self):
                        num_active += 1
            
            # Poll from all tiles currently active
            active = filter(lambda tile: tile.conn is not None, tiles)
            rlist, wlist, xlist = select.select(active, [], active, 2.0)
            
            # Collect from all tiles where data has arrived
            for list in rlist, xlist:
                for tile in list:
                    if tile.conn is not None:
                        tile.collect(settings, self, is_multilayer)
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
        
        
    
    def _makeTiles(self, frame, resx, resy, max_pixels):
        #print("make tiles:", minx, miny, resx, resy, max_pixels)
        U, V = optimal_tiling(resx, resy, max_pixels)
        
        result = []
        for v in range(V):
            ymin = resy * v // V
            ymax = resy * (v+1) // V 
            for u in range(U):
                xmin = resx * u // U
                xmax = resx * (u+1) // U
                c = BitWrkRenderEngine._getcolor()
                result.append(Tile(frame, xmin, ymin, xmax-xmin, ymax-ymin, [c[0], c[1], c[2], 1]))
        
        return result
