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

import atexit, bpy.app, os.path, subprocess

WORKER_PROC = None
worker_path = os.path.join(os.path.abspath(os.path.dirname(__file__)), "blender-slave.py")

def _exithandler():
    global WORKER_PROC
    p, WORKER_PROC = WORKER_PROC, None
    if p is not None:
        p.terminate()
        
def worker_alive():
    global WORKER_PROC
    if WORKER_PROC is None:
        return False
    if WORKER_PROC.poll() is not None:
        # process has exited
        WORKER_PROC = None
        return False
    return True

def can_start_worker(settings):
    if worker_alive():
        return False
    return os.path.isfile(worker_path)

def start_worker(settings):
    global WORKER_PROC
    if worker_alive():
        return
    if settings.use_custom_python_executable:
        python_path = settings.custom_python_executable
    else:
        python_path = bpy.app.binary_path_python 
    args = [
        python_path,
        worker_path,
        "--blender", bpy.app.binary_path,
        "--bitwrk-host", str(settings.bitwrk_client_host),
        "--bitwrk-port", str(settings.bitwrk_client_port),
        "--max-cost", str(settings.complexity),
        "--device", str(settings.worker_device),
        ]
    print("Starting worker:", args)
    WORKER_PROC = subprocess.Popen(args)
    atexit.register(_exithandler)
    
def can_stop_worker():
    return worker_alive()

def stop_worker():
    global WORKER_PROC
    if not worker_alive():
        return
    print("Terminating worker")
    p, WORKER_PROC = WORKER_PROC, None
    p.terminate()
    atexit.unregister(_exithandler)
    
