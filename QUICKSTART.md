Quick Start Instructions
========================
For the impatient, this will get you running within 5 minutes.

Using BitWrk for rendering 
--------------------------
[![5 Minute introduction to BitWrk](https://img.youtube.com/vi/KmwcxwhIRr0/0.jpg)](https://www.youtube.com/watch?v=KmwcxwhIRr0)

You need at least Blender 2.78 (Blender 2.79 is recommended).

Users of *Windows, Linux and Mac OS X* are encouraged to download the BitWrk binary release specific your OS and unpack it somewhere on your computer's file system. The exact location doesn't matter.

Users of *other operating systems* can easily compile BitWrk
[themselves](COMPILING.md).

Download **render_bitwrk-[version].zip** and install it as a Blender add-on (see [Blender Manual](https://docs.blender.org/manual/en/dev/preferences/addons.html)).

After installation of the add-on, Blender will provide the option to choose *"BitWrk Render"* as a rendering engine. Select it. A new panel titled *"BitWrk distributed rendering*" appears at the bottom of the *Render* tab in Blender's *Properties* view. There is an option *"BitWrk client executable file"*: Select the executable from the archive you just downloaded (bitwrk-client.exe or just bitwrk-client). Click on *"Start BitWrk client"*. You will notice that a number of options appears in the panel, now that Blender has established a connection to a BitWrk client.

### Swarm rendering for FREE
To start a render, press *F12* as usual. You will see that a number of colored tiles is shown in the render area. Before rendering can begin, you need to set the price you're willing to pay for a tile, starting at 0 BTC. You can do that in the BitWrk client's **browser-based user interface**. Click on *"Open BitWrk Client User Interface"*. Every tile is represented by an entry under "Activities". Click on "Publish" in any of them. In the dialog that pops up, set **"BTC 0"** as price. Enable "Valid for up to" and set it to 1000 trades (or more). Click "Submit" and BitWrk will start dispatching tiles to the swarm.

### Local rendering
Further acceleration can be achieved by using your own computer in addition to the BitWrk swarm.
For this to happen, you need to start a *worker* process on your own computer. The BitWrk settings panel has an option for this. Just click on *"Start Worker"* and your computer will automatically render some of the tiles. If you own a GPU, you may choose *"GPU Compute"* as a worker device before starting the worker.

### Network rendering
If you're lucky enough to have a home network with a number of computers running idle, then *network rendering* is an option for you. You may combine network rendering with local rendering and even swarm rendering for optimal performance. Network rendering is also a good option if your project is privacy-sensitive, because it does not involve the BitWrk service at all and no data is transfered to third parties.

With network rendering, the computers in your network have to cover three roles (the terminology comes from Blender3D's own network rendering add-on):

 - There must be *exactly one* **Master** computer. This is where the BitWrk client will be run on, the one that will perform all communication to the BitWrk service, if any.
 - Each computer in the network may fulfill a **Slave** role. These computers do the actual rendering, which is why they must each run a worker process. The more slaves, the faster the rendering.
 - At least one computer will be the **Client**. Contrary to what the name suggests, this is not where the BitWrk client runs, but where you, the user, will do the actual 3D modeling, and where you trigger the rendering.
 
#### The Master
 - Unpack the BitWrk client somewhere
 - Start Blender
 - Install the BitWrk add-on
 - Select *"Show advanced options"*
 - Select *"Allow other computers as workers"*
 - Start BitWrk client
 - Optionally: Start worker
 
#### The Slaves
 - No need to install the BitWrk client
 - Start Blender
 - Install the BitWrk add-on
 - Select *"Show advanced options"*
 - Under *"BitWrk client host"*, enter the hostname or IP of the **master**
 - Under *"Worker Device"*, select GPU or CPU appropriately
 - Start worker

#### The Client
 - No need to install the BitWrk client
 - Start Blender
 - Install the BitWrk add-on
 - Select *"Show advanced options"*
 - Under *"BitWrk client host"*, enter the hostname or IP of the **master**
 - Hit *F12* to render

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
This is a little bit more involved and requires some knowledge about networking and using
the command line.

For selling to work, you will need to open a TCP port of your choice. This
usually means configuring your local DSL router. If you don't know what this
means, please google for "open incoming tcp port" :-)

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
