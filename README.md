BitWrk - Bitcoin-fueled Peer-to-Peer Rendering (and more)
=========================================================

[![5 Minute introduction to BitWrk](https://img.youtube.com/vi/KmwcxwhIRr0/0.jpg)](https://www.youtube.com/watch?v=KmwcxwhIRr0)

Artists use [Blender](http://blender.org), a powerful yet free 3D software, to create impressive
pictures and movies. This a requires a time-consuming, and costly, production step called _rendering_.

BitWrk integrates with Blender and makes rendering much quicker by dispatching it to a swarm of
computers. 

By creating a _marketplace for computing power_, BitWrk introduces a new kind of
_cloud computing_, in which resources are shared in a peer-to-peer fashion. It
works like a stock exchange, using crypto currency [Bitcoin](https://bitcoin.org/)
for payment.

This is interesting for two groups of people:
- The **buyers**: Users who require lots of computing power at their finger tips.
- The **sellers**: Hardware owners who have computing power to spare and would like to
  monetize that resource.

BitWrk provides a service to both groups by connecting them in an easy-to-use way.

### On the web
[bitwrk.net](https://bitwrk.net/) | [Download](https://github.com/indyjo/bitwrk/releases) | [Facebook](https://www.facebook.com/bitwrk) | [Twitter](https://twitter.com/BitWrk)

### Documentation
[News](NEWS.md) | [Quickstart instructions](QUICKSTART.md) | [Concepts](CONCEPTS.md) | [Compiling](COMPILING.md) | [License information](COPYING)

Status
------

As of version 0.6.0:
- BitWrk concentrates on the use case of providing peer-to-peer rendering for [Blender](http://blender.org),
  the free rendering software, into which it integrates by use of an add-on. A feature often requested by
  Blender users is local network rendering, even for single frames. By providing local workers with local
  jobs without going through the BitWrk service, this version useful even for non-p2p users.
  Renderings using Cycles (Blender's modern rendering engine) have been successfully accelerated
  at a small scale. While some features may be missing or not work as expected, BitWrk has shown
  to work very well with projects of small to medium size and high rendering complexity. With support
  for linked resources and scripted drivers, bitwrk-blender is approaching a state where it can be
  used for larger projects, too.
- BitWrk is now integrated with a Bitcoin payment processing system, allowing users to pay for
  compute power, in Bitcoin. For this, the user has to request a deposit address, which will
  be provided after a couple of seconds by the payment processor. Bitcoin transactions need at
  least 6 confirmations, i.e. depositing on BitWrk takes one hour on average. Withdrawals aren't
  enabled yet for security reasons. Users are advised to keep the amount of money stored on BitWrk
  as small as possible (deposits can be as small as 0.001 BTC!). Of course, a pay-out can be
  triggered manually by the developer. Ask him!
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

### Build status
master: [![Build Status](https://travis-ci.org/indyjo/bitwrk.svg?branch=master)](https://travis-ci.org/indyjo/bitwrk)
| experimental: [![Build Status](https://travis-ci.org/indyjo/bitwrk.svg?branch=experimental)](https://travis-ci.org/indyjo/bitwrk)
