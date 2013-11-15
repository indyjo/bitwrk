Bitwrk - A Bitcoin-friendly, anonymous marketplace for computing power
======================================================================

This is going to be a proof-of-concept implementation of a marketplace
for computing services, an idea I had in 2011 when I saw the enormous
amounts of computing capacity dedicated to mining Bitcoin.

Since then, the work needed to mine a single Bitcoin has increased by
at least a factor of 50! (See http://blockchain.info/charts/hash-rate)

If generating this enormous amount of work for nothing more than a
simple cryptographic lottery can be lucrative, there must be demand
for tasks more useful. Let's find out!

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
- A client, also written in Go, that currently contains enough logic
  to perform both sides of a transaction. A browser-based user interface
  is in the works, but still incomplete. There is very rudimentary
  support for registering and unregistering workers and still no UI
  for managing accounts.
  The client is meant to act as a proxy, taking tasks from
  local programs and dispatching them to the internet. For sellers, it
  provides the service to offer local worker programs to the BitWrk
  exchange and to keep them busy.

In the current phase of development, there is no way to transfer money into
or out of the BitWrk service. Thus, no actual money can be made or lost.


News
----

- 2013-11-15: After a break of two months, development has continued.
  The client now has the ability to not only list activities, but
  also to ask the user for a permission (valid up to a specified number of
  trades or minutes) or to cancel not yet granted activities.
- 2013-09-01: There is now a simple user interface that shows the account's
  current balance, annd lists currently scheduled activities. It is possible to
  cancel (forbid) activities interactively. There is now a REST API to register
  and unregister workers.
- 2013-08-16: The client is now able to perform a full transaction. Both
  buyer and seller side are implemented. There is no mechanism to register
  workers yet, so a dummy worker is registered: The work package is sent to
  http://httpbin.org/post and the result is whatever that page returns.
- 2013-08-08: The client no longer places a random bid on the server.
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
server's source code is open-sourced too.

The client is a command-line tool. To find out about its usage, type:
<pre>
$ ./bitwrk-client --help
Usage of bitwrk-client:
  -bitcoinprivkey="random": The private key of the Bitcoin address to use for authentication
  -bitwrkurl="http://bitwrk.appspot.com/": URL to contact the bitwrk service at
  -extaddr="auto": IP address or name this host can be reached under from the internet
  -extport=-1: Port that can be reached from the Internet (-1 disables incoming connections)
  -intport=8081: Maintenance port for admin interface
  -resourcedir="auto": Directory where the bitwrk client loads resources from
</pre>
<dl>
<dt><strong>-bitcoinprivkey</strong></dt>
<dd>When placing a bid (regardless of whether it's a buy or a sell), the client
must authenticate to the server. This is done by using the same kind of private
keys that Bitcoin uses to authenticate a transaction. In fact, every participant
of a Bitwrk transaction uses a Bitcoin address as his participant ID. Money
earned by selling work on BitWrk will be transferred (via Bitcoin) back to this
address.<br />
By default, a random key is generated. In later versions, the key will be stored
on disk permanently. It is also possible to pass a key in Wallet Interchange
Format (WIF), the format used to export and import private keys from and into
a Bitcoin wallet using <em>dumpprivkey</em> and <em>importprivkey</em> commands.</dd>
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

Have fun!
2013-11-15, Jonas Eschenburg

