BitWrk-Blender brings support for the BitWrk distributed computing service to Blender, the
Open Source 3D renderer. The files in this folder should have come packaged as a .zip file.

You don't need to unpack this file, just install it as a Blender add-on package:

 - Start Blender
 - Select the "File->User Preferences" menu item
 - Switch to tab "Add-ons"
 - Press the "Install from file..." button in the bottom of the preferences dialog
 - Select the .zip file and double-click on it
 - In the search box, enter "bitwrk" to filter the list of add-ons shown
 - Make sure that "Render: BitWrk Distributed Rendering" has been checked
 - If this is an update: restart the add-on by un-checking it first
 - Back in the main window, you should be able to switch the renderer to BitWrk
 
 This package also contains blender-slave.py, the script that provides Blender
 rendering to the BitWrk service.
 
 For more information, see https://bitwrk.net !
 
 Have fun! 