# Variables to compile go client
GO_CLIENT=bitwrk-client
GO_RELEASE_DIST=./dist
CLIENT_LINUX=$(GO_RELEASE_DIST)/bitwrk_linux_amd64/$(GO_CLIENT)
CLIENT_DARWIN=$(GO_RELEASE_DIST)/bitwrk_darwin_amd64/$(GO_CLIENT)
CLIENT_WINDOWS=$(GO_RELEASE_DIST)/bitwrk_windows_amd64/$(GO_CLIENT).exe
ADDON_NAME_ROOT=render_bitwrk
VERSION?=snapshot
ADDON_NAME=$(ADDON_NAME_ROOT)-$(VERSION)
ifeq ($(VERSION),snapshot)
GORELEASER_CMD:=--snapshot
else
GORELEASER_CMD:=
endif

# Variables to zip addon and client daemon into one 
BUILD_TMPDIR=tmp
CLIENT_DIR=bitwrk_client
RESOURCE_DIR=resources
RENDER_DIR=bitwrk-blender
ADDON_DIR=render_bitwrk
SUFFIX_DARWIN=-osx.x64
SUFFIX_LINUX=-linux.x64
SUFFIX_WINDOWS=-windows.x64

all: build-go prep-addon package-darwin package-linux package-windows cleanup-addon

build-go:
	goreleaser $(GORELEASER_CMD) --skip-publish --rm-dist

prep-addon:
	echo "CLEAN UP PREVIOUS BUILD"
	rm -rf $(BUILD_TMPDIR)
	echo "CREATE ADDON DIRECTORY STRUCTURE"
	mkdir $(BUILD_TMPDIR) && \
	cp -r $(RENDER_DIR)/$(ADDON_DIR) $(BUILD_TMPDIR)/ && \
	mkdir $(BUILD_TMPDIR)/$(ADDON_DIR)/$(CLIENT_DIR) && \
	cp -r $(RESOURCE_DIR) $(BUILD_TMPDIR)/$(ADDON_DIR)/$(CLIENT_DIR)/

package-darwin:
	echo "DARWIN: COPY CLIENT EXECUTABLE TO ADDON STRUCTURE"
	cp $(CLIENT_DARWIN) $(BUILD_TMPDIR)/$(ADDON_DIR)/$(CLIENT_DIR)/ && \
	echo "DARWIN: ZIP ADDON" && \
	cd $(BUILD_TMPDIR) && \
	zip -r ../$(ADDON_NAME)$(SUFFIX_DARWIN).zip * && \
	echo "DARWIN: REMOVE CLIENT EXECUTABLE" && \
	cd .. && \
	rm $(BUILD_TMPDIR)/$(ADDON_DIR)/$(CLIENT_DIR)/$(GO_CLIENT)

package-linux:
	echo "LINUX: COPY CLIENT EXECUTABLE TO ADDON STRUCTURE"
	cp $(CLIENT_LINUX) $(BUILD_TMPDIR)/$(ADDON_DIR)/$(CLIENT_DIR)/ && \
	echo "LINUX: ZIP ADDON" && \
	cd $(BUILD_TMPDIR) && \
	zip -r ../$(ADDON_NAME)$(SUFFIX_LINUX).zip * && \
	echo "LINUX: REMOVE CLIENT EXECUTABLE" && \
	cd .. && \
	rm $(BUILD_TMPDIR)/$(ADDON_DIR)/$(CLIENT_DIR)/$(GO_CLIENT)

package-windows:
	echo "WINDOWS: COPY CLIENT EXECUTABLE TO ADDON STRUCTURE" && \
	cp $(CLIENT_WINDOWS) $(BUILD_TMPDIR)/$(ADDON_DIR)/$(CLIENT_DIR)/ && \
	echo "WINDOWS: ZIP ADDON" && \
	cd $(BUILD_TMPDIR) && \
	zip -r ../$(ADDON_NAME)$(SUFFIX_WINDOWS).zip * && \
	echo "WINDOWS: REMOVE CLIENT EXECUTABLE" && \
	cd .. && \
	rm $(BUILD_TMPDIR)/$(ADDON_DIR)/$(CLIENT_DIR)/$(GO_CLIENT).exe

cleanup-addon:
	rm -rf $(BUILD_TMPDIR)
