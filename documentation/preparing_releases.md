Howto: Preaparing a new release for BitWrk
==========================================

Server-side code changes
------------------------
- server_motd.go: Change welcome message

Client-side code changes
------------------------
- blender-slave.py: Verify that get_blender_version supports the
  correct set of versions and prints the right message
- render_bitwrk.py: bl_info contains version number
- client_version.py: ClientVersion contains version number
- info.json: Contains version number

Now test it, at least once! Do not forget to test server, too!

Documentation changes
---------------------
- README.md: News section, status, overall a good opportunity to work on it

Updating server
---------------
    appcfg.py update bitwrk-server/app.yaml
    
Git operations
--------------
- Commit everything, verify that no open issues remain
- Create tag:
    git tag -a vx.y.z
- Push tag:
    git push origin vx.y.z
    
Compiling for Mac OS X
----------------------
- Login as root, create directory under /tmp
- Unpack and compile:
    wget --content-disposition https://github.com/indyjo/bitwrk/archive/v0.4.0.tar.gz
    tar xzf bitwrk-0.4.0.tar.gz
    cd bitwrk-0.4.0/
    export GOPATH=$(pwd)
    cd src/cmd/bitwrk-client/
    go install
- Prepare binary tgz
    cd ../../../../
    mv bitwrk-0.4.0 bitwrk-0.4.0-src
    mkdir bitwrk-0.4.0
    cp -a bitwrk-0.4.0-src/share bitwrk-0.4.0-src/bin bitwrk-0.4.0/
    tar czvf bitwrk-0.4.0-osx.x64.tgz bitwrk-0.4.0
- Prepare bitwrk-blender zip
    mkdir bitwrk-blender-0.4.0
    cp bitwrk-0.4.0-src/bitwrk-blender/* bitwrk-blender-0.4.0/
    zip -r bitwrk-blender-0.4.0.zip bitwrk-blender-0.4.0/
    

Compiling for Windows
---------------------
- Similar, but observe different directory layout!

Remaining work
--------------
- Create GitHub release
- Announce on Facebook and Twitter
- Announce on blenderartists, blendpolis and bitcointalk
