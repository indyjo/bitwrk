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

import io, os, struct
from render_bitwrk.resources import RESOURCE_COLLECTIONS, object_filepath, object_uniqpath, resource_id

class Tagged:
    """Produces an IFF-like stream"""
    def __init__(self, out):
        self.out = out
        self.aliases = {}
        
    def writeResource(self, file, origpath, abspath):
        """Writes a resource linked by the blend file into the stream.
        Chunk format is:
         'rsrc' CHUNKLENGTH
                ALIASLENGTH alias...
                ORIGLENGTH origpath...
                FILELENGTH filedata...
        """
        
        if abspath in self.aliases:
            return self.aliases[abspath]
         
        if type(origpath) != bytes:
            origpath = origpath.encode('utf-8')
         
        if origpath in self.aliases:
            # Only write resources not written yet
            return
        
        alias = resource_id(abspath).encode('utf-8')
        
        file.seek(0, os.SEEK_END)
        filelength = file.tell()
        file.seek(0, os.SEEK_SET)

        # chunk size must not exceed MAX_INT
        chunklength = filelength + len(origpath) + len(alias) + 12
        if chunklength > 0x8fffffff:
            raise RuntimeError('File is too big to be written: %d bytes' % chunklength)
        
        self.aliases[origpath] = alias
        self.out.write(struct.pack('>4sI', b'rsrc', chunklength))
        self.out.write(struct.pack('>I', len(alias)))
        self.out.write(alias)
        self.out.write(struct.pack('>I', len(origpath)))
        self.out.write(origpath)
        self.out.write(struct.pack('>I', filelength))
        while filelength > 0:
            data = file.read(min(filelength, 4096))
            filelength = filelength - len(data)
            self.out.write(data)
        self.aliases[abspath] = alias
        return alias
        
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
        
    def bundleResources(self, engine, data):
        for collection_name in RESOURCE_COLLECTIONS:
            collection = getattr(data, collection_name)
            for obj in collection:
                try:
                    if hasattr(obj, 'packed_file') and obj.packed_file is not None:
                        file = io.BytesIO(obj.packed_file.data)
                    else:
                        path = object_filepath(obj)
                        if path:
                            file = open(path, "rb")
                        else:
                            continue
                    
                    with file:
                        alias = self.writeResource(file, obj.filepath, object_uniqpath(obj))
                        engine.report({'INFO'}, "Successfully bundled {} resource {} = {}".format(collection_name, alias, obj.name))
                except (FileNotFoundError, NotADirectoryError) as e:
                    engine.report({'WARNING'}, "Error bundling {} resource {}: {}".format(collection_name, obj.name, e))
