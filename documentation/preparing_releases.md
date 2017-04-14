Howto: Preparing a new release for BitWrk
=========================================

Server-side code changes
------------------------
- server_motd.go: Change welcome message

Client-side code changes
------------------------
- blender-slave.py: Verify that get\_blender\_version supports the
  correct set of versions and prints the right message
- render\_bitwrk.py: bl_info contains version number
- client_version.go: ClientVersion contains version number
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

Creating a source tarball
-------------------------
Use "git-archive-all" to prepare the tarballs, because github's tarballs don't include
the submodules (cafs, bitwrk-common)

    # git-archive-all can be cloned from github like this:
    # git clone https://github.com/Kentzo/git-archive-all.git
    git-archive-all --prefix bitwrk-x.y.z/ ../bitwrk-x.y.z.tar.gz
    # repeat for .zip

Now upload both .tar.gz and .zip to github, associate with release.

Compiling for Mac OS X
----------------------
- Login as root, create directory under /tmp
- Unpack and compile:  
    wget https://github.com/indyjo/bitwrk/releases/download/v0.5.0/bitwrk-0.5.0.tar.gz  
    tar xzf bitwrk-0.5.0.tar.gz  
    cd bitwrk-0.5.0/  
    export GOPATH=$(pwd)  
    go install ./src/github.com/indyjo/bitwrk-client/cmd/bitwrk-client/...  
- Prepare binary tgz  
    cd ../../../../  
    mv bitwrk-0.5.0 bitwrk-0.5.0-src  
    mkdir bitwrk-0.5.0  
    cp -a bitwrk-0.5.0-src/share bitwrk-0.5.0-src/bin bitwrk-0.5.0/  
    tar czvf bitwrk-0.5.0-osx.x64.tgz bitwrk-0.5.0  
- Prepare bitwrk-blender zip  
    mkdir bitwrk-blender-0.5.0  
    cp bitwrk-0.5.0-src/bitwrk-blender/* bitwrk-blender-0.5.0/  
    zip -r bitwrk-blender-0.5.0.zip bitwrk-blender-0.5.0/  

Compiling for Windows
---------------------
- Similar, but observe different directory layout!

Remaining work
--------------
- Create GitHub release
- Announce on Facebook and Twitter
- Announce on blenderartists, blendpolis and bitcointalk
