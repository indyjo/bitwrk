Quick Start Instructions
========================
For the impatient, this will get you running within 5 minutes.

These steps apply to users of the 64 bit versions of BitWrk on Windows and Mac OS X, the
systems for which binary packages are provided. Linux users can easily compile BitWrk
[themselves](COMPILING.md) and possibly resort to other shortcuts, such as installing
BitWrk or the Go development kit using the system's package manager.

Using BitWrk for rendering 
--------------------------
[![5 Minute introduction to BitWrk](https://img.youtube.com/vi/KmwcxwhIRr0/0.jpg)](https://www.youtube.com/watch?v=KmwcxwhIRr0)

You need at least Blender 2.76 (Blender 2.78 is recommended).

Download the BitWrk binary release specific your OS and unpack it somewhere.

Download **render_bitwrk-[version].zip** and install it as a Blender add-on (see [Blender Manual](https://docs.blender.org/manual/en/dev/preferences/addons.html)).

In the settings of the Blender add-on there is an option to select which BitWrk client you would like to start. Select the executable from the archive you downloaded (bitwrk-client.exe or just bitwrk-client). Click on "Start BitWrk client".

### Rendering for free
To start a render, press F12 as usual. You will see that a number of colored tiles is shown in the render area. Before rendering can begin, you need to set the price you're willing to pay for a tile. You can do that in the BitWrk client's user interface. Click on "Open BitWrk Client User Interface". Click on "Publish". Set **"BTC 0"** as price. Set "Valid up to" to 100 trades. Click "Submit" and BitWrk will start dispatching tiles to the network, accelerating your rendering.

You can accelerate it further by starting a worker on your own computer. 

### Depositing money on your account
In the long term, rendering won't always be free. In order to pay for the computing power you
use, you need to deposit a small amount of Bitcoin on your aacount. Deposits can be as small
as 1 mBTC (BTC 0.001), i.e. you *don't* need to put large amounts of money on BitWrk, and
there is *no* subscription involved.

To deposit money on your account:
- Make sure you have a Bitcoin client installed (either on your PC, or on your cell phone, tablet
  etc.) that has some money on it. Please refer to http://bitcoin.org for more information on that
  topic.
- In the BitWrk client's user interface, go to the "Accounts" tab (http://localhost:8081/ui/account)
- New accounts don't have a deposit address assigned to them. Click on "Generate a new deposit address"
  and wait for a couple of seconds. A QR code should appear which can be scanned with your cell phone.
  If you have a Bitcoin client installed on your computer, you may directly click the address link.
- Using your Bitcoin client, deposit a *small* amount of money to the generated address.
- Because of the way Bitcoin works, your account will be credited with the transferred amount after
  about one hour, which equates to 6 Bitcoin confirmations.


Selling rendering power on BitWrk
---------------------------------
This is a little bit more involved and requires some knowledge abort networking and using
the command line.

For selling to work, you will need to open a TCP port of your choice. This
usually means configuring your local DSL router. If you don't know what this
means, please Google for "open incoming tcp port" :-)

Without an open port, you can't sell, but you can still buy computing power on
the BitWrk network (this is what you will typically do)!

Suppose you have port 8082 reachable by the internet. Now stop any running BitWrk
clients by closing the respective command shell window (for Windows users) or by
typing Ctrl-C in the Terminal (for Mac users). Restart the BitWrk client with
selling enabled:
    bitwrk-client -extport 8082

Now run the provided script "blender-slave.py" the following way (you need to have Python > 3.2 installed, see
http://www.python.org):

    python3 blender-slave.py --blender /Path/To/Blender/blender
