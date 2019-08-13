Compiling BitWrk yourself
=========================

Prerequisites
--------------
Download and install Google's Go SDK to be able to compile BitWrk:
    http://golang.org/doc/install

From a command prompt, you should be able to run the "go" tool.

Compiling the source package
-----------------------------
- Download and unpack the latest BitWrk client package from
  https://github.com/indyjo/bitwrk/releases
- Compile and start the BitWrk client software:

        # Version number 0.3.0 serves as an example
        cd bitwrk-0.3.0/

        # Now compile the BitWrk client software needed for buying and selling
        go build ./client/cmd/bitwrk-client/

        # If everything went fine, the BitWrk client can be started now.
        ./bitwrk-client

Compiling from GIT
-------------------
Compiling from GIT is the same, just don't forget the "--recursive" option to also clone
the submodules:
        git clone --recursive https://github.com/indyjo/bitwrk.git
