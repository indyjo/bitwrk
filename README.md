BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
======================================================================

<a href="http://www.youtube.com/watch?feature=player_embedded&v=BHBe5g5KKiw"
   target="_blank"><img src="http://img.youtube.com/vi/BHBe5g5KKiw/0.jpg" 
   alt="IMAGE ALT TEXT HERE" width="240" height="180" border="10" /></a>

BitWrk introduces a new kind of _cloud computing_, in which resources are shared in a peer-to-peer
fashion.

This is interesting for two groups of people:
- The **buyers**: Users who require lots of computing power at their finger tips.
  For example, artists using rendering software such as [Blender](http://blender.org)
  to create impressive movies.
- The **sellers**: Hardware owners who have computing power to spare and would like to
  monetize that resource in times of low workload.
  
BitWrk provides a service to both groups by connecting them in an easy-to-use way.

Users of BitWrk can even be a buyer and a seller at the same time, enabling them to compensate for
bursts of high need for computing power by continuously providing some computing power to others, at
virtually no cost.

> Keep in touch with this project and be informed about news and updates:<br>
> **Website:** http://bitwrk.net/
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
  - **2015-11-01:** Release of BitWrk 0.5.1 "Moon": Support for Blender 2.76,
  compressed data transmission, revised transaction logic and lots of bugs fixed
  - **2015-08-10:** Release of BitWrk 0.5.0: Bitcoin integration is finally here, allowing
  users to pay in Bitcoin
  - **2015-01-15:** News have moved to BitWrk's website: http://bitwrk.net
  - **2014-12-30:** BitWrk was featured in a Lightning Talk by the lead developer on the
  31st Chaos Communication Congress: https://www.youtube.com/watch?v=BHBe5g5KKiw
  - **2014-12-05:** New participants now start with a zero balance as preparations for an upcoming
  beta test have started. The test will include real Bitcoin transactions processed by the
  payment system.
  - **2014-11-11:** Payment system integration is progressing. Required refactoring BitWrk into
  separate projects:
  - **bitwrk** now contains code specific to client and server and may be refactored further at
  a later time.
  - **bitwrk-common** contains code that is shared amongst client, server, and payment processor.
  - **cafs**, the Content-Addressable File System, has been extracted for use in third-party projects.
- [More news...](NEWS.md)

Status
------

As of version 0.5.1:
- BitWrk is now integrated with a Bitcoin payment processing system, allowing users to pay for
  compute power, in Bitcoin. For this, the user has to request a deposit address, which will
  be provided after a couple of seconds by the payment processor. Bitcoin transactions need at
  least 6 confirmations, i.e. depositing on BitWrk takes one hour on average. Withdrawals aren't
  enabled yet for security reasons. Users are advised to keep the amount of money stored on BitWrk
  as small as possible (deposits can be as small as 0.001 BTC!). Of course, a pay-out can be
  triggered manually by the developer. Ask him!
- BitWrk includes "bitwrk-blender", an add-on for [Blender](http://blender.org), the free
  rendering software.
  bitwrk-blender consists of *render_bitwrk.py*, a Python addon which registers
  a new rendering engine, and *blender-slave.py*, a script for sellers.
  Renderings using Cycles (Blender's modern rendering engine) have been successfully accelerated
  at a small scale. While some features may be missing or not work as expected, BitWrk has shown
  to work very well with projects of small to medium size and high rendering complexity. With support
  for linked resources and scripted drivers, bitwrk-blender is approaching a state where it can be
  used for larger projects, too. 
- A central server, written in Go (http://golang.org/), is deployed on Google App Engine.
  It exports an API for entering bids and updating transactions. Every transaction's lifecycle can
  be traced, and all communication is secured with Elliptic-Curve cryptographic
  signatures. These are of the same kind than those that can be generated using
  the original Bitcoin client, so it is very easy to test for correctness.
- A client (also called the "daemon"), written in Go, provides a browser-based user interface to
  everything related to BitWrk. The daemon enables control of ongoing trades, registered workers
  and automatic trading mandates. It also provides access to BitWrk's Bitcoin-based payment system.
- The client acts as a proxy, taking tasks from
  local programs and dispatching them to the BitWrk service. For sellers, it
  provides the service to offer local worker programs to the BitWrk
  exchange and to keep them busy.

Have fun!
2015-11-01, Jonas Eschenburg
