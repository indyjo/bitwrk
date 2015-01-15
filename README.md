BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
======================================================================

BitWrk is aimed at two groups of people:
- The **buyers**: Users who require lots of computing power at their finger tips.
  For example, artists using rendering software such as [Blender](http://blender.org)
  to create impressive movies.
- The **sellers**: Hardware owners who have computing power to spare and would like to
  monetize that resource in times of low workload.
  
BitWrk provides a service to both groups by connecting them in an easy-to-use way.

Users of BitWrk can even be a buyer and a seller at the same time, enabling them to compensate for
bursts of high need for computing power by continuously providing some computing power to others, at
virtually no cost.

> Keep in touch with this project and be informed about news and updates:
> **Web:** http://bitwrk.net/
> **Facebook:** https://www.facebook.com/bitwrk
> **Twitter:** https://twitter.com/BitWrk

tip4commit: [![tip for next commit](http://tip4commit.com/projects/541.svg)](http://tip4commit.com/projects/541)
master: [![Build Status](https://travis-ci.org/indyjo/bitwrk.svg?branch=master)](https://travis-ci.org/indyjo/bitwrk)
experimental: [![Build Status](https://travis-ci.org/indyjo/bitwrk.svg?branch=experimental)](https://travis-ci.org/indyjo/bitwrk)

What next?
----------
- Visit BitWrk's website at http://bitwrk.net
- For the impatient: Following the [Quick Start Instructions](QUICKSTART.md) gets you
  started in 5 minutes.
- [About BitWrk](ABOUT.md) explains what BitWrk is and what it is meant to become.
- [Compiling BitWrk Yourself](COMPILING.md) caters to developers and Linux users.
- Read more about the [Concepts Behind BitWrk](CONCEPTS.md).
- Consult [this file](COPYING) about the license under which BitWrk is distributed (GPLV3).


News
----
  - **2015-01-15:** News have moved to BitWrk's website: http://bitwrk.net
  - **2014-12-30:** BitWrk was featured in a Lightning Talk by the lead developer on the
  31st Chaos Communication Congress: https://www.youtube.com/watch?v=BHBe5g5KKiw
  - **2014-11-11:** Payment system integration is progressing. Required refactoring BitWrk into
  separate projects:
  - **bitwrk** now contains code specific to client and server and may be refactored further at
  a later time.
  - **bitwrk-common** contains code that is shared amongst client, server, and payment processor.
  - **cafs**, the Content-Addressable File System, has been extracted for use in third-party projects.
- [More news...](NEWS.md)

Status
------

As of version 0.4.1:
- BitWrk includes "bitwrk-blender", an add-on for [Blender](http://blender.org), the free
  rendering software.
  bitwrk-blender consists of *render_bitwrk.py*, a Python addon which registers
  a new rendering engine, and *blender-slave.py*, a script for sellers.
  Renderings using Cycles (Blender's modern rendering engine) have been successfully accelerated
  at a small scale. While some features may be missing or not work as expected, BitWrk has shown
  to work very well with projects of small to medium size and high rendering complexity. With support
  for linked resources and scripted drivers, bitwrk-blender is approaching a state where it can be
  used for larger projects, too. 
- A basic server, written in Go (http://golang.org/), is deployed on Google App Engine.
  It exports an API for entering bids and updating transactions. Every transaction's lifecycle can
  be traced, and all communication is secured with Elliptic-Curve cryptographic
  signatures. These are of the same kind than those that can be generated using
  the original Bitcoin client, so it is very easy to test for correctness.
- A client (also called the "daemon"), written in Go, providing a browser-based user interface to
  everything related to BitWrk. The daemon enables control of ongoing trades, registered workers
  and automatic trading mandates and will take care of deposits onto and withdrawals from the BitWrk
  service.
- The client is meant to act as a proxy, taking tasks from
  local programs and dispatching them to the BitWrk service. For sellers, it
  provides the service to offer local worker programs to the BitWrk
  exchange and to keep them busy.

In the current phase of development, there is no way to transfer money into
or out of the BitWrk service. Thus, **no actual money can be made or lost.**
Every new client account starts with **1 BTC virtual starting capital**.

Have fun!
2015-ß1-15, Jonas Eschenburg
