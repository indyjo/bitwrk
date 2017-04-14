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

"""Methods for processing blend files.

When called by Blender as a -P script, processes the .blend file in a way suitable for
transferring it to a BitWrk worker:
 - All referenced files are referenced by an MD5 hash instead of a (potentially unsafe) file name.
 - All scripted drivers are removed, because Python scripts cannot be executed on a worker.
   Blender automatically calculates and persists the selected frame's actual driven values.
""" 

import bpy, subprocess, sys, traceback
from render_bitwrk.resources import RESOURCE_COLLECTIONS, object_filepath, object_uniqpath, resource_path

def save_copy(filepath):
    bpy.ops.wm.save_as_mainfile(filepath=filepath, check_existing=False, copy=True, relative_remap=True, compress=False)

def process_file(filepath):
    """Opens the given blend file in a separate Blender process and substitutes
    file paths to those which will exist on the worker side."""
    ret = subprocess.call([sys.argv[0], "-b", "--enable-autoexec", "-noaudio", filepath, "-P", __file__, "--", "process", filepath])
    if ret != 0:
        raise RuntimeError("Error processing file '{}': Calling blender returned code {}".format(filepath, ret))

def _repath():
    """Modifies all included paths to point to files named by the pattern
    '//rsrc.' + md5(absolute original path) + '.data'
    This method is called in a special blender session.
    """
    
    # Switch to object mode for make_local
    bpy.ops.object.mode_set(mode='OBJECT')
    # Make linked objects local to current blend file.
    bpy.ops.object.make_local(type='ALL')
    
    def repath_obj(obj):
        path = object_uniqpath(obj)
        if path:
            obj.filepath = resource_path(path)
        else:
            print("...skipped")
            
    # Iterate over all resource types (including libraries) and assign paths
    # to them that will correspond to valid files on the remote side.
    for collection_name in RESOURCE_COLLECTIONS:
        collection = getattr(bpy.data, collection_name)
        print("Repathing {}:".format(collection_name))
        for obj in collection:
            print("  {} ({})".format(obj.filepath, object_filepath(obj)))
            repath_obj(obj)
            print("   -> " + obj.filepath)

def _remove_scripted_drivers():
    """Removes Python drivers which will not execute on the seller side.
    Removing them has the benefit of materializing the values they have evaluated to
    in the current context."""

    for collection_name in dir(bpy.data):
        collection = getattr(bpy.data, collection_name)
        if not isinstance(collection, type(bpy.data.objects)):
            continue
        
        # Iterate through ID objects with animation data
        for idobj in collection:
            if not isinstance(idobj, bpy.types.ID) or not hasattr(idobj, "animation_data"):
                break
            anim = idobj.animation_data
            if not anim:
                continue
            for fcurve in anim.drivers:
                driver = fcurve.driver
                if not driver or driver.type != 'SCRIPTED':
                    continue
                print("Removing SCRIPTED driver '{}' for {}['{}'].{}".format(driver.expression, collection_name, idobj.name, fcurve.data_path))
                try:
                    idobj.driver_remove(fcurve.data_path)
                except TypeError as e:
                    print("  -> {}".format(e))
   
if __name__ == "__main__":
    try:
        idx = sys.argv.index("--")
    except:
        raise Exception("This script must be given a special parameter: -- process")
    
    try:
        args = sys.argv[idx + 1:]
        if len(args) > 1 and args[0] == 'process':
            _repath()
            _remove_scripted_drivers()
            bpy.ops.wm.save_as_mainfile(filepath=args[1], check_existing=False, compress=False)
    except:
        traceback.print_exc()
        sys.exit(-1)

