# ##### BEGIN GPL LICENSE BLOCK #####
#
#  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
#  Copyright (C) 2013-2020  Jonas Eschenburg <jonas@bitwrk.net>
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

import bpy, webbrowser
from bpy.props import StringProperty, IntProperty, PointerProperty, EnumProperty, FloatProperty
from render_bitwrk.common import get_article_id, max_tilesize, render_resolution
from render_bitwrk.tiling import optimal_tiling
import render_bitwrk.bitwrkclient as bitwrkclient
import render_bitwrk.worker as worker
from render_bitwrk.render import is_render_active

class StartBrowserOperator(bpy.types.Operator):
    """Open BitWrk admin console in web browser"""
    bl_idname = "bitwrk.startbrowser"
    bl_label = "Open BitWrk Client User Interface"

    @classmethod
    def poll(cls, context):
        return bitwrkclient.probe_bitwrk_client(context.scene.bitwrk_settings)
    
    def execute(self, context):
        settings=context.scene.bitwrk_settings
        webbrowser.open("http://{}:{}/".format(settings.bitwrk_client_host, settings.bitwrk_client_port)) 
        return {'FINISHED'}

    def invoke(self, context, event):
        return self.execute(context)
    
class StartBitwrkClientOperator(bpy.types.Operator):
    """Start a private BitWrk client as a sub-process of Blender"""
    bl_idname = "bitwrk.startclient"
    bl_label = "Start BitWrk client"

    @classmethod
    def poll(cls, context):
        return bitwrkclient.can_start_bitwrk_client(context.scene.bitwrk_settings)

    def execute(self, context):
        settings=context.scene.bitwrk_settings
        bitwrkclient.start_bitwrk_client(settings) 
        return {'FINISHED'}


class StopBitwrkClientOperator(bpy.types.Operator):
    """Stops a previously started private BitWrk client"""
    bl_idname = "bitwrk.stopclient"
    bl_label = "Stop BitWrk client"

    @classmethod
    def poll(cls, context):
        return bitwrkclient.can_stop_bitwrk_client()

    def execute(self, context):
        bitwrkclient.stop_bitwrk_client() 
        return {'FINISHED'}

class StartWorkerOperator(bpy.types.Operator):
    """Join the rendering swarm with this computer"""
    bl_idname = "bitwrk.startworker"
    bl_label = "Start worker"

    @classmethod
    def poll(cls, context):
        return worker.can_start_worker(context.scene.bitwrk_settings)

    def execute(self, context):
        settings=context.scene.bitwrk_settings
        worker.start_worker(settings) 
        return {'FINISHED'}

class StopWorkerOperator(bpy.types.Operator):
    """Stop rendering on this computer"""
    bl_idname = "bitwrk.stopworker"
    bl_label = "Stop worker"

    @classmethod
    def poll(cls, context):
        return worker.can_stop_worker()

    def execute(self, context):
        worker.stop_worker() 
        return {'FINISHED'}

class RENDER_PT_bitwrk_settings(bpy.types.Panel):
    bl_label = "BitWrk distributed rendering"
    bl_space_type = "PROPERTIES"
    bl_region_type = "WINDOW"
    bl_context = "render"
    COMPAT_ENGINES = {"BITWRK_RENDER"}
    
    @classmethod
    def poll(cls, context):
        rd = context.scene.render
        return rd.engine == 'BITWRK_RENDER'
    
    def draw(self, context):
        settings=context.scene.bitwrk_settings

        self.layout.prop(settings, "expert_mode")
        self.layout.separator()

        if bitwrkclient.probe_bitwrk_client(settings):
            self.layout.operator("bitwrk.startbrowser", icon='URL')
        else:
            self.layout.label(
                text="No BitWrk client at {}:{}".format(settings.bitwrk_client_host, settings.bitwrk_client_port),
                icon='ERROR')
        
        row = self.layout.row(align=True)
        row.operator("bitwrk.startclient", icon='PLAY')
        row.operator("bitwrk.stopclient", icon='X')
        
        if settings.expert_mode and not bitwrkclient.can_stop_bitwrk_client():
            layout = self.layout.column()
            layout.enabled = not is_render_active() and not worker.worker_alive()
            row = layout.split(factor=0.5)
            row.label(text="BitWrk client executable file:")
            row.prop(settings, "bitwrk_client_executable_path", text="")
            row = layout.split(factor=0.5)
            row.label(text="BitWrk client host:")
            row.prop(settings, "bitwrk_client_host", text="")
            row = layout.split(factor=0.5)
            row.label(text="BitWrk client port:")
            row.prop(settings, "bitwrk_client_port", text="")
        
        if not bitwrkclient.probe_bitwrk_client(settings):
            row = self.layout.split(factor=0.5)
            if settings.expert_mode:
                self.layout.label(text="BitWrk can dispatch jobs to local network computers:")
                row = self.layout.split(factor=0.02)
                row.label(text=" ")
                row.prop(settings, "bitwrk_client_allow_nonlocal_workers")
            
        if bitwrkclient.probe_bitwrk_client(settings):
            self.layout.separator()

            if settings.expert_mode:
                row = self.layout.row()
                row.enabled = not worker.worker_alive()
                row.prop(settings, "complexity")

            if settings.expert_mode:
                row = self.layout.split(factor=0.5)
                row.label(text="Article id: ", icon="RNDCURVE")
                row.label(text=get_article_id(settings.complexity, settings.trusted_render))
            
            resx, resy = render_resolution(context.scene)
            max_pixels = max_tilesize(context.scene)
            u,v = optimal_tiling(resx, resy, max_pixels)
            if settings.expert_mode:
                row = self.layout.split(factor=0.333)
                row.label(text="Tiles per frame", icon='MESH_GRID')
                row.label(text="{}   ({}x{}, efficiency: {:.1%})".format(u*v, u, v, resx*resy/u/v/max_pixels))
            else:
                self.layout.label(text="{} tiles per frame ({}x{})".format(u*v, u, v), icon='MESH_GRID')

            if settings.expert_mode:
                row = self.layout.split(factor=0.333)
                row.label(text="Tiles at once", icon='NLA')
                row.prop(settings, "concurrency")

            if settings.expert_mode:
                row = self.layout.split(factor=0.333)
                row.enabled = not is_render_active()
                row.label(text="Boost factor", icon='NEXT_KEYFRAME')
                row.prop(settings, "boost_factor")
            if settings.boost_factor > 1:
                self.layout.label(text="A boost factor greater than 1.0 makes rendering more expensive!", icon='ERROR')

class RENDER_PT_bitwrk_trusted_settings(bpy.types.Panel):
    bl_label = "Trusted Cloud Rendering"
    bl_parent_id = "RENDER_PT_bitwrk_settings"
    bl_space_type = "PROPERTIES"
    bl_region_type = "WINDOW"
    bl_options = {'DEFAULT_CLOSED'}

    @classmethod
    def poll(cls, context):
        settings=context.scene.bitwrk_settings
        return bitwrkclient.probe_bitwrk_client(settings)

    def draw_header(self, context):
        settings=context.scene.bitwrk_settings
        self.layout.enabled = not is_render_active()
        self.layout.prop(settings, "trusted_render", text="")

    def draw(self, context):
        settings=context.scene.bitwrk_settings
        if settings.trusted_render:
            # FAKE_USER_ON is a shield which can be interpreted as providing protection and security.
            self.layout.label(
                icon='FAKE_USER_ON',
                text="Your scene is rendered on a trusted cloud.")
            self.layout.label(
                icon='LOCKED',
                text="Your assets are secure.")
            self.layout.label(
                icon='BLANK1',
                text="Disable 'Trusted Cloud Rendering' for a more economical alternative.")
        else:
            self.layout.label(
                icon='INFO',
                text="Your scene is rendered on a public swarm.")
            self.layout.label(
                icon='UNLOCKED',
                text="Don't use for privacy-sensitive projects.")
            self.layout.label(
                icon='BLANK1',
                text="Enable 'Trusted Cloud Rendering' for more security and reliability.")

class RENDER_PT_bitwrk_local_worker_settings(bpy.types.Panel):
    bl_label = "Also render on this computer"
    bl_parent_id = "RENDER_PT_bitwrk_settings"
    bl_space_type = "PROPERTIES"
    bl_region_type = "WINDOW"
    bl_options = {'DEFAULT_CLOSED'}

    @classmethod
    def poll(cls, context):
        settings=context.scene.bitwrk_settings
        return bitwrkclient.probe_bitwrk_client(settings)

    def draw(self, context):
        settings=context.scene.bitwrk_settings

        row = self.layout.row(align=True)
        row.operator("bitwrk.startworker", icon='PLAY')
        row.operator("bitwrk.stopworker", icon='X')

        row = self.layout.row()
        row.active = not worker.worker_alive()
        row.prop(settings, "worker_device")

        if settings.expert_mode:
            layout = self.layout.row()
            layout.enabled=not worker.worker_alive()
            layout.prop(settings, "use_custom_python_executable")
            if settings.use_custom_python_executable:
                layout.prop(settings, "custom_python_executable", text="")
