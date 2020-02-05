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

import atexit, bpy.path, time, http, re, os, subprocess, threading, stat

# Functions for probing host:port settings for a running BitWrk client
LAST_PROBE_LOCK = threading.RLock()
LAST_PROBE_TIME = time.time() - 10.0
LAST_PROBE_RESULT = False
LAST_PROBE_SETTINGS = None
LAST_PROBE_THREAD = None
EXECUTABLE_RELATIVE_PATH = 'bitwrk_client/bitwrk-client'

def probe_bitwrk_client(settings):
    global LAST_PROBE_LOCK, LAST_PROBE_TIME, LAST_PROBE_RESULT, LAST_PROBE_SETTINGS, LAST_PROBE_THREAD
    with LAST_PROBE_LOCK:
        if LAST_PROBE_THREAD is None and time.time() - LAST_PROBE_TIME >= 1.0:
            LAST_PROBE_THREAD = threading.Thread(target=do_probe_bitwrk_client, args=(settings,), daemon=True)
            LAST_PROBE_THREAD.start()
        
        return LAST_PROBE_RESULT if settings_string(settings) == LAST_PROBE_SETTINGS else False
    
def do_probe_bitwrk_client(settings):
    global LAST_PROBE_LOCK, LAST_PROBE_TIME, LAST_PROBE_RESULT, LAST_PROBE_SETTINGS, LAST_PROBE_THREAD
    result = do_probe_bitwrk_client_pure(settings)
    with LAST_PROBE_LOCK:
        LAST_PROBE_RESULT = result
        LAST_PROBE_TIME = time.time()
        LAST_PROBE_SETTINGS = settings_string(settings)
        LAST_PROBE_THREAD = None
    
def settings_string(settings):
    return "{}:{}".format(settings.bitwrk_client_host, settings.bitwrk_client_port)
    
def do_probe_bitwrk_client_pure(settings):
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


def client_executable_path(settings):
    exec_dirname = os.path.dirname(os.path.abspath(__file__))
    return os.path.join(exec_dirname, EXECUTABLE_RELATIVE_PATH)

CLIENT_PROC = None

def _exithandler():
    global CLIENT_PROC
    p, CLIENT_PROC = CLIENT_PROC, None
    if p is not None:
        p.terminate()
        
def bitwrk_client_alive():
    global CLIENT_PROC
    if CLIENT_PROC is None:
        return False
    if CLIENT_PROC.poll() is not None:
        # process has exited
        CLIENT_PROC = None
        return False
    return True


def make_bitwrk_client_executable(clientpath):
    """Set the client permissions to executable for both owner and group"""
    statinfo = os.stat(clientpath)
    os.chmod(clientpath, statinfo.st_mode | stat.S_IXUSR | stat.S_IXGRP)


def can_start_bitwrk_client(settings):
    if bitwrk_client_alive():
        return False
    if probe_bitwrk_client(settings):
        return False
    clientpath = client_executable_path(settings)

    make_bitwrk_client_executable(clientpath)

    return os.path.isfile(clientpath)

def start_bitwrk_client(settings):
    global CLIENT_PROC
    if bitwrk_client_alive():
        return
    clientpath = client_executable_path(settings)
    args = [clientpath, "-intport", str(settings.bitwrk_client_port)]
    if settings.bitwrk_client_allow_nonlocal_workers:
        args.extend(["-intiface", ""])
    CLIENT_PROC = subprocess.Popen(args)
    atexit.register(_exithandler)
    
def can_stop_bitwrk_client():
    return bitwrk_client_alive()

def stop_bitwrk_client():
    global CLIENT_PROC
    if not bitwrk_client_alive():
        return
    p, CLIENT_PROC = CLIENT_PROC, None
    p.terminate()
    atexit.unregister(_exithandler)
    
