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

import bpy, os, hashlib

# Collections under bpy.data which contain linkable resources
RESOURCE_COLLECTIONS = ["images", "sounds", "texts", "movieclips"]


def resource_id(path):
    if type(path) != bytes:
        path = path.encode('utf-8')
    return hashlib.md5(path).hexdigest()

def resource_path(path):
    return"//rsrc." + resource_id(path) + ".data"

def object_filepath(obj):
    """Returns a file system path for an object that is suitable for opening the file.
    Takes linked resources and libraries into account.
    
    Returns None if no such path exists.
    """
    if not obj.filepath:
        return
    if hasattr(obj, 'packed_file') and obj.packed_file:
        return
    if not obj.filepath:
        return
    path = obj.filepath
    while hasattr(obj, 'library') and obj.library:
        lib = obj.library
        if not lib.filepath:
            raise RuntimeError("Library without a filepath: " + lib)
        path = bpy.path.abspath(path, os.path.dirname(lib.filepath))
        obj = lib
    return bpy.path.abspath(path)

def object_type(obj):
    if hasattr(obj, "type"):
        return obj.type
    t = type(obj)
    for typename in dir(bpy.types):
        typeclass = getattr(bpy.types, typename)
        if t == typeclass:
            return typename.upper()
    return "__UNKNOWN__"
    
def object_uniqpath(obj):
    """Returns a special path that is suitable to identify an object uniquely
    and to derive a resource id. Takes linked resources and libraries into account
    in the following way:
      - A referenced file (no packed data) is assigned its absolute, normalized path
      - Files packed into the main blend file are assigned a path that looks like this:
        object_uniqpath(library):IMAGE(the_image_name)
      - The main blend file itself has uniqpath "" (empty)
    """
    if obj is None:
        return ""
    if hasattr(obj, 'packed_file') and obj.packed_file:
        return "{}:{}({})".format(object_uniqpath(obj.library), object_type(obj), obj.name)
    else:
        path = object_filepath(obj)
        return os.path.abspath(path) if path else None
