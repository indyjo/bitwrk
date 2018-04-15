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
#  along with this program.  If not, see <http://www.gnu.org/licenses/>.
#
# ##### END GPL LICENSE BLOCK #####

import bpy
from cycles import engine as cycles_engine

# Keep this method in sync with draw_device from scripts/addons/cycles/ui.py
def draw_device(self, context):
    '''
    Appended to the main render settings panel, simulates the feature selection
    available when Cycles rendering is selected.
    ''' 
    scene = context.scene
    if scene.render.engine != 'BITWRK_RENDER':
        return
    
    cscene = scene.cycles
    layout = self.layout

    layout.prop(cscene, "feature_set")
    
    # Check if Open Shading Language is active and warn
    layout.prop(cscene, "shading_system")
    if cscene.shading_system:
        layout.label("OSL doesn't work on BitWrk if workers use GPU", icon='ERROR')

def register():
    bpy.types.RENDER_PT_render.append(draw_device)
    
def unregister():
    bpy.types.RENDER_PT_render.remove(draw_device)

