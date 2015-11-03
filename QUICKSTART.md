Quick Start Instructions
========================
For the impatient, this will get you running within 5 minutes.

These steps apply to users of the 64 bit versions of BitWrk on Windows and Mac OS X, the
systems for which binary packages are provided. Linux users can easily compile BitWrk
[themselves](COMPILING.md) and possibly resort to other shortcuts, such as installing
BitWrk or the Go development kit using the system's package manager.

Running the BitWrk client
-------------------------
### Mac OS X
- **Step 1:** Download the binary file from https://github.com/indyjo/bitwrk/releases into your *Downloads* folder
        
        bitwrk-x.y.z-osx-x64.tar.gz
- **Step 2:** Open a Terminal and type:
        
        cd ~/Downloads/
        tar xzf bitwrk.x.y.z-osx-x64.tar.gz
        cd bitwrk-x.y.y/bin
        ./bitwrk-client
- **Step 3:** Open http://localhost:8081/ in your browser.
        
### Windows 7 or 8
- **Step 1:** Download the binary file from https://github.com/indyjo/bitwrk/releases
        
        bitwrk-x.y.z-windows.x64.zip
- **Step 2:** Open the downloaded .zip file and drag the contained folder "bitwrk-x.y.z" on your desktop.
- **Step 3:** In the extracted folder, double-click the file called "bitwrk-client.exe".
  It should open in a command shell window.   
- **Step 4:** Open http://localhost:8081/ in your browser.

### Done!
Now you should see the BitWrk client's admin user interface on http://localhost:8081/,
showing your account number (which has been randomly chosen) and your current
balance of **BTC 0** in the status bar at the top of the page.

Your next step is to try buying and selling on the BitWrk network using Blender,
BitWrk's first supported application.

For selling, you may skip a couple of paragraphs.
For buying, i.e. rendering, you need to first deposit some money on your account.

Depositing money on your account
--------------------------------
Before you can start buying on BitWrk, you need to deposit a small amount of Bitcoin on your
account, which will be used to pay for the computing power you use. Deposits can be as small
as 1 mBTC (BTC 0.001), i.e. you *don't* need to put large amounts of money on BitWrk, and
there is *no* subscription involved.

To deposit money on your account:
- Make sure you have a Bitcoin client installed (either on you PC, or on your cell phone, tablet
  etc.) that has some money on it. Please refer to http://bitcoin.org for more information on that
  topic.
- In the BitWrk client's user interface, go to the "Accounts" tab (http://localhost:8081/ui/account)
- New accounts don't have a deposit address assigned to them. Click on "Generate a new deposit address"
  and wait for a couple of seconds. A QR code should appear which can be scanned with your cell phone.
  If you have a Bitcoin client installed on your computer, you may directly click the address link.
- Using your Bitcoin client, deposit a *small* amount of money to the generated address.
- Because of the way Bitcoin works, your account will be credited with the transferred amount after
  about one hour, which equates to 6 Bitcoin confirmations.


Blender Integration
-------------------
Starting with the 20140124 release, BitWrk supports the popular 3D rendering
software, Blender (http://blender.org/), as a proof-of-concept project.


### Accelerating Blender with BitWrk
In order to use BitWrk to accelerate Blender's "Cycles" rendering engine, perform
the following steps: 
- Setup the BitWrk client as described in the previous section.
- Start Blender (if in doubt, use the version supported by the latest BitWrk release).
- Select a scene you like. Verify that it looks
  good when rendered with the "Cycles" rendering engine.
- Go to **User Preferences -> Addons -> Install From File**
- Select **render_bitwrk.py**. You find it in BitWrk's **bitwrk-blender** folder.
- An add-on called "Render: BitWrk Distributed Rendering" should show up. If not,
  search for the new add-on by entering "bitwrk" in the add-on search field.
- Activate the add-on by pressing the checkbox next to the running man icon.
- Click "Save User Settings" to have the BitWrk add-on load every time you start Blender. 
- Back in the main window, you can now select "BitWrk distributed rendering" as the
  active rendering engine.
- You should see a new panel, "BitWrk distributed rendering". Everything can be left as is
  for now. There should be a button labeled "Open BitWrk Client User Interface".
- Next time you hit render (F12), the task is dispatched to the BitWrk service as several
  small tiles.
- You now need to browse to the BitWrk client's user interface (on http://localhost:8081/)
  permit the buys you just made. You can choose a price you are willing to pay for each
  tile. The minimum price allowed by BitWrk is 10 uBTC (BTC 0.00001).


### Selling rendering power on BitWrk
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
