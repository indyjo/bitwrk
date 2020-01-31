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
from bpy.props import BoolProperty, StringProperty, IntProperty, PointerProperty, EnumProperty, FloatProperty
import cycles.properties

# used by BitWrkSettings PropertyGroup
def set_complexity(self, value):
    self['complexity'] = value

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
        settings.complexity = EnumProperty(
            name="Complexity",
            description="Defines the maximum allowed computation complexity for each rendered tile",
            items=[
                ('512M', "0.5 Giga-rays", "", 3),
                ('2G',  " 2 Giga-rays", "", 0),
                ('8G',  " 8 Giga-rays", "", 1),
                ('32G', "32 Giga-rays", "", 2)],
            default='512M',
            set=set_complexity,
            get=lambda value: value['complexity'] if 'complexity' in value else 3)
        settings.concurrency = IntProperty(
            name="Concurrent tiles",
            description="Maximum number of BitWrk trades active in parallel",
            default=4,
            min=1,
            max=256)
        settings.boost_factor = FloatProperty(
            name="Boost factor",
            description="Makes rendering faster (and more expensive) by making tiles smaller than they need to be",
            default=1.0,
            min=1.0,
            max=64.0,
            precision=2,
            subtype='FACTOR')
        settings.expert_mode = BoolProperty(
            name="Show advanced options",
            description="Enables display of expert settings",
            default=False)
        # settings.bitwrk_client_executable_path = StringProperty(
        #     name="BitWrk client executable",
        #     description="Path to the BitWrk client's executable file",
        #     subtype='FILE_PATH')
        settings.use_custom_python_executable = BoolProperty(
            name="Use custom Python interpreter",
            description="Use specified Python interpreter instead of the one provided by Blender",
            default=False)
        settings.custom_python_executable = StringProperty(
            name="Custom Python interpreter",
            description="The custom Python interpreter executable to use for Blender worker",
            subtype='FILE_PATH',
            default=bpy.app.binary_path_python)
        settings.bitwrk_client_allow_nonlocal_workers = BoolProperty(
            name="Allow other computers as workers",
            description="Allow other computers on the network to register as workers",
            default=False)
        settings.worker_device = EnumProperty(
            name="Worker Device",
            description="Device that worker uses for rendering",
            items=cycles.properties.enum_devices,
            default='CPU')
        settings.trusted_render = BoolProperty(
            name="Trusted Cloud Rendering",
            description="If enabled, render on servers of BitWrk's trusted cloud, where your assets are secure." +
                        " If disabled, render on the public swarm, for a lower price.",
            default=True)
        
        bpy.types.Scene.bitwrk_settings = PointerProperty(type=BitWrkSettings, name="BitWrk Settings", description="Settings for using the BitWrk service")

    @classmethod
    def unregister(cls):
        del bpy.types.Scene.bitwrk_settings
        
    def max_cost(self):
        if self.complexity == '512M':
            return  512*1024*1024
        elif self.complexity == '2G':
            return  2*1024*1024*1024
        elif self.complexity == '8G':
            return  8*1024*1024*1024
        elif self.complexity == '32G':
            return  32*1024*1024*1024
        else:
            print(dir(self), self)
            print(self.complexity)
            raise RuntimeError()
