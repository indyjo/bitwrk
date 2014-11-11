News
====
- **2014-11-11:** Payment system integration is progressing. Required refactoring BitWrk into
  separate projects:
  - **bitwrk** now contains code specific to client and server and may be refactored further at
  a later time.
  - **bitwrk-common** contains code that is shared amongst client, server, and payment processor.
  - **cafs**, the Content-Addressable File System, has been extracted for use in third-party projects.
- **2014-10-22:** Release of **BitWrk 0.4.1** concentrating on Blender 2.72, featuring an
  optimal tiling algorithm and some usability enhancements.  
- **2014-09-10:** Release of **BitWrk 0.4.0 (Venus)** featuring a nicer user interface and
  several advanced enhancements to the Blender add-on. 
- **2014-08-13:** Making progress towards a 0.4.0 release, with support for Blender 2.71,
  including linked resources, and work data up to 512MB!
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
  current balance, and lists currently scheduled activities. It is possible to
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