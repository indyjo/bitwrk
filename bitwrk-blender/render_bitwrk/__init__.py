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

bl_info = {
    "name": "BitWrk Distributed Rendering",
    "description": "A peer-to-peer rendering service",
    "author": "Jonas Eschenburg",
    "version": (0, 6, 3),
    "blender": (2, 76, 0),
    "category": "Render",
}

# Minimum Python version: 3.2 (tempfile.TemporaryDirectory)

from render_bitwrk import render, settings, settings_panel, bitwrkclient, worker

if "bpy" in locals():
    import imp
    imp.reload(render)
    imp.reload(settings)
    imp.reload(settings_panel)
    imp.reload(bitwrkclient)
    imp.reload(worker)
import bpy

def register():
    bpy.utils.register_class(render.BitWrkRenderEngine)
    bpy.utils.register_class(settings_panel.RENDER_PT_bitwrk_settings)
    bpy.utils.register_class(settings.BitWrkSettings)
    bpy.utils.register_class(settings_panel.StartBrowserOperator)
    bpy.utils.register_class(settings_panel.StartBitwrkClientOperator)
    bpy.utils.register_class(settings_panel.StopBitwrkClientOperator)
    bpy.utils.register_class(settings_panel.StartWorkerOperator)
    bpy.utils.register_class(settings_panel.StopWorkerOperator)
    for name in dir(bpy.types):
        klass = getattr(bpy.types, name)
        if 'COMPAT_ENGINES' not in dir(klass):
            continue
        if 'CYCLES' not in klass.COMPAT_ENGINES:
            continue
        if 'BITWRK_RENDER' not in klass.COMPAT_ENGINES:
            klass.COMPAT_ENGINES.add('BITWRK_RENDER')
        
    
def unregister():
    try:
        bitwrkclient.stop_bitwrk_client()
    except:
        pass
    bpy.utils.unregister_class(settings_panel.StopWorkerOperator)
    bpy.utils.unregister_class(settings_panel.StartWorkerOperator)
    bpy.utils.unregister_class(settings_panel.StopBitwrkClientOperator)
    bpy.utils.unregister_class(settings_panel.StartBitwrkClientOperator)
    bpy.utils.unregister_class(settings_panel.StartBrowserOperator)
    bpy.utils.unregister_class(settings.BitWrkSettings)
    bpy.utils.unregister_class(settings_panel.RENDER_PT_bitwrk_settings)
    bpy.utils.unregister_class(render.BitWrkRenderEngine)

