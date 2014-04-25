BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
======================================================================

> In 2011, an open marketplace for computing power was a visionary concept.
> In 2014, it has become reality.

BitWrk is based on the idea that many of today's applications consume enormous
amounts of computing power. Their thirst for CPU and GPU power is never satisfied.
However expensive, your hardware is never enough.

This is what *BitWrk* tries to solve:
- The computing power you don't have can be provided by the internet,
  in a peer-to-peer fashion.
- Small units of computation are dispatched to potentially thousands of participating
  peers, giving a huge performance boost.
- Internet currencies such as [Bitcoin](http://bitcoin.org/) are ideally suited to
  support the kind of micro-transactions necessary to make this work.
  
> **BitWrk = Crowd + Cloud**

BitWrk includes an add-on for [Blender](http://blender.org/), the Free
3D rendering software suite. This enables Blender users to accelerate their renderings using
the power of peer-to-peer computing.

> *"I want BitWrk to become a true community project. You can support the project by
> trying out the software and testing it for bugs! If you prefer to support BitWrk
> financially, consider sending some milli-Bitcoins to* **tip4commit**. *They will be distributed
> to everybody who contributes code to BitWrk. Alternatively, you could donate directly to
> 1BiTWrKBPKT2yKdfEw77EAsCHgpjkqgPkv and help finance some servers providing Blender
> rendering. Last but not least, BitWrk is in need of developers."*
>
>   -- Jonas Eschenburg, developer of Bitwrk

tip4commit: [![tip for next commit](http://tip4commit.com/projects/541.svg)](http://tip4commit.com/projects/541)
master: [![Build Status](https://travis-ci.org/indyjo/bitwrk.svg?branch=master)](https://travis-ci.org/indyjo/bitwrk)
experimental: [![Build Status](https://travis-ci.org/indyjo/bitwrk.svg?branch=experimental)](https://travis-ci.org/indyjo/bitwrk)



Quick Start Instructions
------------------------
For the impatient, this will get you running within 5 minutes.

These steps apply to users of Windows, Mac OS X and Linux, although there
might be shortcuts for some users (like installing Go using the system's
package manager).

For selling to work, you will need to open a TCP port of your choice. This
usually means configuring your local DSL router. If you don't know what this
means, please Google for "open incoming tcp port" :-)

Without an open port, you can't sell, but you can still buy computing power on
the BitWrk network (this is what you will typically do)!

- **Step 1:** Download and install Google's Go SDK to be able to compile BitWrk:
  http://golang.org/doc/install
  
  From a command prompt, you should be able to run the "go" tools.
- **Step 2:** Download and unpack the latest BitWrk client package from
  https://github.com/indyjo/bitwrk/releases 
- **Step 3:** Compile and start the BitWrk client software:
        
        # Version number 0.3.0 serves as an example
        cd bitwrk-0.3.0/
        
        # Now set GOPATH environment variable to directory root
        # Linux/Mac OS X users:
        export GOPATH=$(pwd)
        # Windows users:
        set GOPATH=%cd%
        
        # Now compile the BitWrk client software needed for buying and selling
        cd src/cmd/bitwrk-client
        go install
        
        # If everything went fine, the BitWrk client can be started now.
        ../../../bin/bitwrk-client

  Now you should see the BitWrk client's admin user interface on http://localhost:8081/,
  showing your account number (which has been randomly chosen) and your current (virtual)
  balance of **BTC 1** in the black bar at the top of the page.
  
  Your next step is to try buying and selling on the BitWrk network using Blender,
  BitWrk's first supported application.

Blender Integration
-------------------
Starting with the 20140124 release, BitWrk supports the popular 3D rendering
software, Blender (http://blender.org/), as a proof-of-concept project.


### Accelerating Blender with BitWrk
In order to use BitWrk to accelerate Blender's "Cycles" rendering engine, perform
the following steps: 
- Setup the BitWrk client as described in the previous section.
- Start Blender (at least version 2.69). Select a scene you like. Verify that it looks
  good when rendered with the "Cycles" rendering engine, and that it doesn't import
  any assets from external library files.
- Go to **User Preferences -> Addons -> Install From File**
- Select **render_bitwrk.py**. You find it in BitWrk's **bitwrk-blender** folder.
- An add-on called "Render: BitWrk Distributed Rendering" should show up. If not,
  search for the new plugin by entering "bitwrk" in the add-on search field.
- Activate the add-on by pressing the checkbox next to the running man icon.
- Click "Save User Settings" to have the BitWrk add-on load every time you start Blender. 
- Back in the main window, you can now select "BitWrk distributed rendering" as the
  active rendering engine.
- You should see a new panel, "BitWrk Settings Panel". Everything can be left as is
  for now. There should be a button labeled "Open BitWrk Client User Interface".
- Next time you hit render (F12), the task is dispatched to the BitWrk service as several
  small tiles.
- You now need to browse to the BitWrk client's user interface (on http://localhost:8081/)
  permit the buys you just made. You can choose a price you are willing to pay for each
  tile (this is just proof-of-concept for now, there is no money involved with BitWrk at
  this stage). Best to leave it at the default.
  
### Selling rendering power on BitWrk
If you would like to **sell Blender rendering** on BitWrk, run the provided script
"blender-slave.py" the following way (you need to have Python > 3.2 installed, see
http://www.python.org):

    python3 blender-slave.py --blender /Path/To/Blender/blender


Idea
----
BitWrk aims to be a marketplace for computing power. Rather than providing
computing resources itself (like "cloud" service providers do),
it is a marketplace where buyers and sellers meet.

In its core, BitWrk works like a stock exchange. The difference is that
it's not stocks that are traded, but computing tasks. There are different
kinds of computing tasks on BitWrk, just like there are different stocks
traded on a stock exchange.

Buyers are people who wish to get some computing tasks done (as quickly as
possible, and as cheap as possible). They profit from BitWrk because they
get on-demand access to enormous computing resources, from their desktop
computers.

Sellers are people who provide the service of handling those tasks to the
public. They profit directly from the money earned in exchange.

Prices are determined by the rules of supply and demand. Participants may
range from hobbyists to companies (both buyers, and sellers).

The use of [Bitcoin](http://bitcoin.org) as BitWrk's preferred currency
guarantees a low entry barrier, especially for potential buyers, provided
that the success of Bitcoin continues. It also enables registration-less,
anonymous participation.

If, on the other hand, Bitcoin turns out _not_ to be a good choice, that's not going to be a problem. BitWrk itself does not _depend_ on it. Other currencies and payment methods can be integrated later on.

Status
------
This project has been under heavy development for the last couple of
months and currently consists of:
- A rudimentary server written in Go (http://golang.org/), running on
  Google App Engine. It exports an API for entering bids and updating
  transactions. The lifecycle of every transaction can be traced,
  and all communication is secured with Elliptic-Curve cryptographic
  signatures of the same kind than those that can be generated using
  the original Bitcoin client.
- A client, also written in Go, that contains all necessary logic
  to perform both sides of a transaction. A browser-based user interface
  enables control of ongoing trades, registered workers and automatic
  trading mandates.
  The client is meant to act as a proxy, taking tasks from
  local programs and dispatching them to the BitWrk service. For sellers, it
  provides the service to offer local worker programs to the BitWrk
  exchange and to keep them busy.
- "bitwrk-blender", an add-on to the Blender graphics
  software, consisting of *render_bitwrk.py*, a Python addon which registers
  a new rendering engine, and *blender-slave.py*, a script for sellers.

In the current phase of development, there is no way to transfer money into
or out of the BitWrk service. Thus, **no actual money can be made or lost.**
Every new client account starts with **1 BTC virtual starting capital**.


News
----

- **2014-04-25:** Release of **BitWrk 0.3.0 (Mercury)** featuring support for Blender 2.70a
  and many enhancements and bug fixes.
- **2014-03-26:** BitWrk is making big progress towards a new release. Many user interface
  enhancements, both in BitWrk's browser-based client, as well as in the Blender add-on,
  make working with BitWrk smoother every day. A huge boost in performace comes from a
  unique compression mechanism that reduces network transmissions to a minimum.
- **2014-01-25:** Experimental Blender integration is now available.
- **2013-12-04:** A lot of progress has been made on the client side. Basic
  management functionality is now available for trades, workers and mandates.
  The client identity is no longer randomly generated every time the client
  is started, but saved on disk. This is a necessary precondition for later
  being able to link Bitcoin transactions to the account
- **2013-11-27:** Some progress has been made, mainly on the client side.
  A sample application has been adapted to use BitWrk: See
  https://github.com/indyjo/rays for the Rays raytracer project.
- **2013-11-15:** After a break of two months, development has continued.
  The client now has the ability to not only list activities, but
  also to ask the user for a permission (valid up to a specified number of
  trades or minutes) or to cancel not yet granted activities.
- **2013-09-01:** There is now a simple user interface that shows the account's
  current balance, annd lists currently scheduled activities. It is possible to
  cancel (forbid) activities interactively. There is now a REST API to register
  and unregister workers.
- **2013-08-16:** The client is now able to perform a full transaction. Both
  buyer and seller side are implemented. There is no mechanism to register
  workers yet, so a dummy worker is registered: The work package is sent to
  http://httpbin.org/post and the result is whatever that page returns.
- **2013-08-08:** The client no longer places a random bid on the server.
  Performing a POST to <pre>http://localhost:8081/buy/&lt;articleid&gt;</pre> simulates
  how a buy appears to clients, where the result is just a copy of the
  work data.  A rudimentary in-memory content-addressable file storage
  (CAFS) keeps files and serves them under
  <pre>http://localhost:8081/file/&lt;sha256, hex-encoded&gt;</pre>
  To see what's going on, execute:
<pre>
curl -v -F data=@&lt;some filename&gt; -L http://localhost:8081/buy/foobar
</pre>


Usage
-----

The client software provides a graphical web frontend that presents the
user with a view of everything going on between the local machine and
the BitWrk service.

Running the client is very straightforward: Just run <pre>bitwrk-client</pre>
and navigate your Web browser to http://localhost:8081/.

There are also some command-line options:
<pre>
$ ./bitwrk-client --help
Usage of bitwrk-client:
  -bitwrkurl="http://bitwrk.appspot.com/": URL to contact the bitwrk service at
  -extaddr="auto": IP address or name this host can be reached under from the internet
  -extport=-1: Port that can be reached from the Internet (-1 disables incoming connections)
  -intport=8081: Maintenance port for admin interface
  -log-cafs=false: Enable logging for content-addressable file storage
  -num-unmatched-bids=1: Mamimum number of unmatched bids for an article on server
  -resourcedir="auto": Directory where the bitwrk client loads resources from
</pre>
<dl>
<dt><strong>-bitwrkurl</strong></dt>
<dd>The URL the client used to connect to the server. This is useful for testing
locally or for using alternative BitWrk service providers.</dd>
<dt><strong>-extaddr, -extport</strong></dt>
<dd>If you would like to sell on Bitwrk, the buyers must be able to connect to
your computer. You need to provide them with your host's DNS name or IP address,
and a port number. If you are behind a firewall, you must setup your router
to forward the port given here to your computer. See your router's documentation
on "port forwarding" on how to accomplish this.<br />
If your IP address is dynamic, it is best to leave -extaddr set to "auto". The
client will find out which IP address to use. If -extport is set to "-1", 
selling on BitWrk is disabled.</dd>
<dt><strong>-intport</strong></dt>
<dd>The port number on which the client listens on for local connections. When left
to the default value, the client's user interface will be reachable by opening
<a href="http://localhost:8081/">http://localhost:8081/</a> in your web browser.
Local programs will be able to dispatch work to the BitWrk network by doing a
POST to http://localhost:8081/<em>&lt;article-id&gt;</em>, where <em>&lt;article-id</em>
identifies the article to trade. It must be an article that is traded on the BitWrk
server.
<dt><strong>-num-unmatched-bids</strong></dt>
<dd>Limits the number of not-yet-matched bids that are sent to the BitWrk service.</dd>
</dl>

Identity Management
-------------------

Every participant on the BitWrk service is identified by a unique and seemingly random
combination of numbers and letters, called account id, something like
**1JtLbmh74Tcb5CZk7eZZBH8z4zg4sjey1i**

This ID serves two distinct purposes:
 - It is a unique user ID for participants of the BitWrk service.
 - It is also a valid address for receiving money in the Bitcoin (BTC) currency.

When communicating with the BitWrk service, such as when placing a bid, the client must
authenticate, i.e. prove that it really *is* the the owner address it *claims* to have.

Then, after some time of selling work on BitWrk, the Money earned will be transferred
(via Bitcoin) back to this address.

In order to be able to prove the ownership of its account ID, the BitWrk client must
keep a secret file, called the private key.

On first start, a random private key (and a corresponding BitWrk account ID/Bitcoin address) is
generated and stored on disk permanently (in a file only readable to the user running the
BitWrk client: *~/.bitwrk-client/privatekey.wif*). As the key is stored in a format called
Wallet Interchange Format (WIF), it can be imported into a Bitcoin wallet using the
*importprivkey* command. This gives the Bitcoin client access to the money sent to that
Bitcoin address.

**It is very important to have the private key file backed up in some safe place.**
**It is also very important that neither the private key file, nor the backup be visible
to others.**

Server
------

The server is a web application written for Google Appengine.
Its purpose is to
- accept bids from buyers and sellers
- find matching bids and create transactions
- listen for messages from clients updating the transactions
- enforcing rules by which these transactions must be handled
- do bookkeeping of the participants' accounts

As a user of BitWrk, you shouldn't need to worry about the server. You need
to trust it, though, especially if you decide to send money to it (which as
of now is not possible, but will be). As a trust-building measure, the
server's source code is open-sourced, too.

Have fun!
2014-04-25, Jonas Eschenburg

