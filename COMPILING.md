Compiling BitWrk yourself
=========================

BitWrk releases usually come with compiled binaries for all major platforms,
but if you plan to hack on it or want to be on the cutting edge, you are encouraged to create your own builds.

BitWrk, in its current form, consists of two separate modules:
 - `bitwrk-client`, the background daemon that manages communication with other participants and with the BitWrk service, and provides a web-based user interface. It is written in [Go](http://golang.org) and compiled to native code.
 - `render_bitwrk`, the add-on for Blender which integrates into Blender's user interface and communicates with the local `bitwrk-client`. It is written in [Python](http://python.org) and just needs to be packaged into a zip file.

Compiling `bitwrk-client`
=========================

Prerequisites
-------------

Download and install Google's Go SDK to be able to compile BitWrk:
    http://golang.org/doc/install

From a command prompt, you should be able to run the `go` command.

If you want to compile from [git](https://git-scm.com/) (the recommended way), make sure that you are able to run the `git` command.

Compiling from git
-------------------

First check out the most recent version of BitWrk:

        # You may also clone from your private branch on github.com
        git clone https://github.com/indyjo/bitwrk.git
        
Then compile:

        cd bitwrk
        go build ./client/cmd/bitwrk-client/
        ./bitwrk-client

Packaging `render_bitwrk`
=========================

Due to Blender's naming conventions, the add-on is packaged into a file called `render_bitwrk-x.y.z.zip`, where `x.y.z` is the version number. To aid with packaging, there is a Makefile which can be used with GNU make:

        cd bitwrk/bitwrk-blender
        
        # The version number doesn't really matter, but it helps to keep an overview
        make version=0.7.0-snapshot
        
Alternatively, in case you don't have GNU make installed, just put the contents of `bitwrk/bitwrk-blender` (except `Makefile`) into a .ZIP archive.
