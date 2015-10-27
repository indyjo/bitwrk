Concepts of BitWrk
==================
Understanding BitWrk can be complex. This document tries to take apart the components
that make BitWrk work.

The Client Software
-------------------
The client software (or _client_, for short), is the user-visible part of BitWrk. Its job
is to negotiate between an application in need of computing power,
the BitWrk service, the other participants on the network and you, the user.

Software that has been optimized to work with BitWrk will only communicate with the
BitWrk client, never with the BitWrk service itself.

You as a user will interact with the client's user interface that is accessible with
a web browser (by default, it will run at http://localhost:8081/). The client will
submit bids to the BitWrk service upon your behalf, and only when explicitly allowed to
do so. Additionally, you will be presented a view on everything going on between the
local machine and the BitWrk service.


### Usage
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
  -num-unmatched-bids=1: Maximum number of unmatched bids for an article on server.
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
<dd>Limits the number of not-yet-matched bids that are sent to the BitWrk service.
It is usually enough to limit this to 1, but sometimes performance can profit from
a higher value.</dd>
</dl>

The Server
----------
The server is a web application written for Google Appengine.
Its purpose is to
- accept bids from buyers and sellers
- find matching bids and create transactions
- listen for messages from clients updating the transactions
- enforcing rules by which these transactions must be handled
- do bookkeeping of the participants' accounts

The server is designed to **not keep any secrets in its database**. So, a
security leak by which an attacker could steal the data stored on the server
would *not* allow them to perform any monetary transactions in the name of users.
 
As a user of BitWrk, you shouldn't need to worry about the server. You need
to trust it, though, especially if you decide to send money to it (which as
of now is not possible, but will be). As a trust-building measure, the
server's source code is open-sourced, too.

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

How Bidding Works
-----------------

In order to take part in a BitWrk transaction, participants may bid to either work
for money (sell), or pay for work (buy).

A buy bid might look like this:
> I will pay *up to* 200uBTC to anyone who does 1 unit of _X_ work for me.
where _X_ would be something like "Blender 2.75 rendering up to 512 million rays".
(In case you're wondering, these units of work are standardized).

In contrast, a sell bid might look like this:
> I will perform 1 unit of _X_ work for anyone who pays *at least* 100uBTC.

Any bid stays valid for a duration of 2 minutes. A _matching_ bid may
 - exist at the time the bid is entered,
 - be created within the 2 minutes of the first bids validity
 - not be created at all.

If a matching bid is found, both bids are simultaneously marked as _matched_
(which removes them from the pool of valid bids)
and a _transaction_ is created as proof of the contract between buyer and seller.

The transaction also defines the actual price the buyer pays and the seller receives
upon completion of the job. It is always the price defined by the earlier of the two bids.

| Earlier bid           | Later bid             | Resulting price |
| --------------------: | --------------------: | --------------: |
| BUY for up to 200     | SELL for at least 100 |             200 |
| SELL for at least 100 | BUY for up to 200     |             100 |
| BUY for up to 100     | SELL for at least 200 | No match!       |
| SELL for at least 200 | BUY for up to 100     | No match!       |

Transaction Fees
----------------

BitWrk imposes a fee on transactions. The fee is currently fixed at 3% of the transaction
price. It is paid by the buyer in addition to the transaction price.

If a buyer's bid can be matched instantly, the transaction price is defined by the minimum
price set by the seller. Therefore, the fee also depends on the seller's price. An amount
equal to the sum of both is blocked on the buyer's account for the duration of the
transaction.

If a buyer's bid can _not_ be matched right away, an amount equal to the bid's price plus
the corresponding fee is blocked on the buyer's account. In case a transaction
arises from the bid at a later time, its price and fee will equal those of the bid.
Otherwise, if the bid expires before a transaction is created, the blocked amount will be
reimbursed, including the fee.


Transaction Outcomes
--------------------

BitWrk ensures that, at the time a transaction results from two matching bids, an
amount equal to the transaction price plus the corresponding fee is blocked on the
buyer's account. There is no way for the buyer to spend the money and be unable to pay.

Transactions have just two states, _active_ and _retired_, but while they're _active_,
they go through different phases. Each phase transition affects the _active_ life of
the transaction. Some phases increase the time left by a fixed number of minutes,
others cause instant retirement. For details see the [documentation of BitWrk's protocol](documentation/protocol.md).

If the time has come to retire a transaction, there are two possibilities. Either the
transaction has ended in a success phase (_unverified_ and _finished_). In that case, 
the transaction price is transferred to the seller and the fee is finally deducted.
Or the transaction ends in one of the other phases and the buyer's blocked amount is
reimbursed.
