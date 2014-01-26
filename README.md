Bitwrk - A Bitcoin-friendly, anonymous marketplace for computing power
======================================================================

This is going to be a proof-of-concept implementation of a marketplace
for computing services, an idea I had in 2011 when I saw the enormous
amounts of computing capacity dedicated to mining Bitcoin.

If generating this enormous amount of work for nothing more than a
simple cryptographic lottery can be lucrative, there must be demand
for tasks more useful. Tasks that people are actually willing to pay for.
Let's find out!

Quick Start Instructions
------------------------
For the impatient, this will get you running within 5 minutes.

These steps apply to users of Windows, Mac OS X and Linux, although there
might be shortcuts for some users (like installing Go using the system's
package manager).
- **Step 1:** Download and install Google's Go SDK to be able to compile BitWrk:
  http://golang.org/doc/install
  
  From a command prompt, you should be able to run the "go" tools.
- **Step 2:** Download the Bitwrk client, compile and start it:
  https://github.com/indyjo/bitwrk/archive/master.zip

        cd bitwrk-master
        # Linux/Mac OS X users:
        . env-vars.sh # To set the GOPATH for compilation
        # Windows users:
        set GOPATH=C:/Path/To/bitwrk-master/gopath
        cd bitwrk-client
        go build
        # Port 8082 needs to be reachable for selling to work
        ./bitwrk-client -extport 8082

  Now navigate your web browser to http://localhost:8081/ and keep it open.
  You should see  your account number (which has been randomly chosen) and your current balance of **BTC 1**.
- **Step 3:** Download the sample application, compile and start it:
  https://github.com/indyjo/rays/archive/master.zip

        cd rays-master
        cd gorays
        go build
        # For buying:
        ./gorays -bitwrk-master -a ../ART
        # For selling:
        ./gorays -bitwrk-slave


Blender Integration
-------------------
Starting with the 20140124 release, BitWrk supports everybody's darling 3D rendering
software, Blender (http://blender.org/), as a proof-of-concept project.

If you are a **Blender artist** and you would like to try out the
new Blender integration:
- Perform steps 1 and 2 from the previous section.
- Start Blender 2.69. Select a scene you like. Verify that it looks good when rendered
  with the "Cycles" rendering engine. Also make sure that "Path Tracing" is selected
  on the "Sampling" rendering properties panel ("Branched Path Tracing" is not supported
  yet).
- Go to **User Preferences -> Addons -> Install From File**
- Select **render_bitwrk.py**. You find it in BitWrk's **bitwrk-blender** folder.
- Now search for the new plugin by entering "bitwrk" in the add-on search field.
  Render: BitWrk Distributed Rendering should be the only selectable add-on now.
- Activate the add-on by pressing the checkbox next to the running man icon.
- Back in the main window, you can now select "BitWrk distributed rendering" as the
  active rendering engine.
- You should see a new panel, "BitWrk Settings Panel". Everything can be left as is.
- Next time you hit render, the task is dispatched to the BitWrk service as several
  small tiles. You should control the BitWrk client's web UI (on http://localhost:8081)
  to see the client's interaction with the BitWrk service.
  
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
- "bitwrk-blender", an experimental integration into the Blender graphics
  software, consisting of *render_bitwrk.py*, a Python addon which registers
  a new rendering engine, and *blender-slave.py*, a script for sellers.
- "gorays", a sample application. It's a simple raytracer demonstrating
  how to *use* BitWrk, and also, for developers,  how to *extend* an
  existing application to leverage the BitWrk service.

In the current phase of development, there is no way to transfer money into
or out of the BitWrk service. Thus, **no actual money can be made or lost.**
Every new client account starts with **1 BTC virtual starting capital**.


News
----

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
2014-01-26, Jonas Eschenburg

